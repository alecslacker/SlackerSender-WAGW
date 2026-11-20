package service

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Respons Anthropic dengan pencarian web: blok pertama server_tool_use (tanpa
// teks), lalu hasil pencarian, lalu jawaban terpecah ke beberapa blok teks.
const anthropicSearchJSON = `{"content":[
 {"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{"query":"berita"}},
 {"type":"web_search_tool_result","tool_use_id":"srvtoolu_1","content":[{"type":"web_search_result","title":"A","url":"https://www.kompas.com/x"}]},
 {"type":"text","text":"Menurut info terbaru, "},
 {"type":"text","text":"harga naik 5%.","citations":[{"type":"web_search_result_location","url":"https://www.kompas.com/x","title":"Kompas"}]}
],"stop_reason":"end_turn","usage":{"input_tokens":100,"output_tokens":20}}`

func TestParseAnthropicGabungSemuaBlokTeks(t *testing.T) {
	resp, err := parseAnthropicResponse([]byte(anthropicSearchJSON))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Menurut info terbaru, harga naik 5%." {
		t.Errorf("teks salah digabung: %q", resp.Content)
	}
	if !resp.Searched || resp.Tokens != 120 || resp.FinishReason != "end_turn" {
		t.Errorf("metadata salah: %+v", resp)
	}
	// media kosong -> domain (tanpa "www.") dipakai, bukan judul.
	if len(resp.Sources) != 1 || resp.Sources[0] != "kompas.com" {
		t.Errorf("sumber salah: %v", resp.Sources)
	}
}

// Regresi: tanpa pencarian, satu blok teks -> hasil identik dengan content[0].
func TestParseAnthropicTanpaPencarianTidakBerubah(t *testing.T) {
	body := `{"content":[{"type":"text","text":"Halo kak"}],"usage":{"input_tokens":3,"output_tokens":2}}`
	resp, err := parseAnthropicResponse([]byte(body))
	if err != nil || resp.Content != "Halo kak" || resp.Searched || resp.Tokens != 5 {
		t.Errorf("berubah dari perilaku lama: %+v err=%v", resp, err)
	}
}

func TestParseAnthropicKosongDanRusak(t *testing.T) {
	if _, err := parseAnthropicResponse([]byte(`{"content":[]}`)); err == nil {
		t.Error("content kosong harus error (perilaku lama)")
	}
	if _, err := parseAnthropicResponse([]byte(`bukan json`)); err == nil {
		t.Error("JSON rusak harus error")
	}
}

func TestWebSearchSupported(t *testing.T) {
	ya := []AIProvider{ProviderZAI, ProviderAnthropic}
	tidak := []AIProvider{ProviderGemini, ProviderNineRouter, ProviderCustom, ProviderOpenAI, ProviderDeepSeek}
	for _, p := range ya {
		if !WebSearchSupported(p) {
			t.Errorf("%s harus didukung", p)
		}
	}
	for _, p := range tidak {
		if WebSearchSupported(p) {
			t.Errorf("%s TIDAK boleh didukung (Gemini: syarat Search Suggestions; proxy: belum tentu meneruskan)", p)
		}
	}
}

// Z.AI: tools web_search terkirim hanya bila diminta, dan TANPA search_engine.
func TestZAIWebSearchBodyDanRespons(t *testing.T) {
	resp := `{"choices":[{"message":{"content":"Jawaban web"},"finish_reason":"stop"}],
	 "web_search":[{"title":"T","link":"https://www.detik.com/a","media":"detikcom"},
	               {"title":"U","link":"https://www.detik.com/b","media":"detikcom"}]}`
	srv, raw := tangkapBody(t, resp)
	s := &AIService{Provider: ProviderZAI, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	out, err := s.Complete(ChatRequest{
		Provider: ProviderZAI, Model: "glm-4.7", WebSearch: true,
		Messages: []ChatMessage{{Role: "user", Content: "berita?"}},
	})
	if err != nil || out.Content != "Jawaban web" || !out.Searched {
		t.Fatalf("hasil salah: %+v err=%v", out, err)
	}
	if len(out.Sources) != 1 || out.Sources[0] != "detikcom" {
		t.Errorf("sumber harus unik: %v", out.Sources)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(*raw, &body); err != nil {
		t.Fatal(err)
	}
	tools, _ := body["tools"].([]interface{})
	if len(tools) != 1 {
		t.Fatalf("tools salah: %s", *raw)
	}
	tool := tools[0].(map[string]interface{})
	ws := tool["web_search"].(map[string]interface{})
	if tool["type"] != "web_search" || ws["enable"] != true || ws["search_result"] != true {
		t.Errorf("bentuk tool salah: %v", tool)
	}
	if _, ada := ws["search_engine"]; ada {
		t.Error("search_engine TIDAK boleh dikirim (docs Z.AI tak konsisten)")
	}
}

// Regresi: WebSearch=false -> body tak membawa "tools" sama sekali.
func TestTanpaWebSearchBodyTidakBerisiTools(t *testing.T) {
	srv, raw := tangkapBody(t, `{"choices":[{"message":{"content":"ok"}}]}`)
	s := &AIService{Provider: ProviderZAI, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	if _, err := s.Complete(ChatRequest{
		Provider: ProviderZAI, Model: "glm-4.7",
		Messages: []ChatMessage{{Role: "user", Content: "halo"}},
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(*raw), "tools") {
		t.Errorf("body bocor field tools: %s", *raw)
	}
}

// rtFunc: RoundTripper tiruan, karena klien Anthropic memakai URL tetap.
type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func balasHTTP(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func TestAnthropicWebSearchEndToEnd(t *testing.T) {
	var sent map[string]interface{}
	cli := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &sent)
		return balasHTTP(200, anthropicSearchJSON), nil
	})}
	s := &AIService{Provider: ProviderAnthropic, APIKey: "k", HTTPClient: cli}
	out, err := s.Complete(ChatRequest{
		Provider: ProviderAnthropic, Model: "claude-x", WebSearch: true,
		Messages: []ChatMessage{{Role: "system", Content: "s"}, {Role: "user", Content: "berita?"}},
	})
	if err != nil || out.Content != "Menurut info terbaru, harga naik 5%." {
		t.Fatalf("jawaban salah: %+v err=%v", out, err)
	}
	tools, _ := sent["tools"].([]interface{})
	if len(tools) != 1 {
		t.Fatalf("tools tak terkirim: %v", sent)
	}
	tool := tools[0].(map[string]interface{})
	if tool["type"] != "web_search_20250305" || tool["name"] != "web_search" ||
		tool["max_uses"] != float64(AnthropicWebSearchMaxUses) {
		t.Errorf("tool salah: %v", tool)
	}
}

func TestAnthropicTanpaWebSearchTidakKirimTools(t *testing.T) {
	var sent map[string]interface{}
	cli := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &sent)
		return balasHTTP(200, `{"content":[{"type":"text","text":"ok"}],"usage":{}}`), nil
	})}
	s := &AIService{Provider: ProviderAnthropic, APIKey: "k", HTTPClient: cli}
	if _, err := s.Complete(ChatRequest{Provider: ProviderAnthropic, Model: "m",
		Messages: []ChatMessage{{Role: "user", Content: "halo"}}}); err != nil {
		t.Fatal(err)
	}
	if _, ada := sent["tools"]; ada {
		t.Errorf("tools bocor tanpa diminta: %v", sent)
	}
}

// Jaring pengaman: provider menolak request bersearch (400) -> ulangi SEKALI
// tanpa pencarian & reasoning_effort; hasil ditandai Downgraded.
func TestCompleteTolerantTurunkanFiturBila400(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if strings.Contains(string(b), "tools") {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":"model tidak mendukung tools"}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"tanpa web"}}]}`))
	}))
	defer srv.Close()
	s := &AIService{Provider: ProviderZAI, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	out, err := s.CompleteTolerant(ChatRequest{
		Provider: ProviderZAI, Model: "glm-4.5", WebSearch: true, ReasoningEffort: "low",
		Messages: []ChatMessage{{Role: "user", Content: "halo"}},
	})
	if err != nil || out.Content != "tanpa web" || !out.Downgraded {
		t.Fatalf("seharusnya pulih & Downgraded: %+v err=%v", out, err)
	}
	if len(bodies) != 2 || strings.Contains(bodies[1], "tools") || strings.Contains(bodies[1], "reasoning_effort") {
		t.Errorf("ulangan harus tanpa tools & reasoning_effort: %v", bodies)
	}
}

func TestCompleteTolerantTanpaFiturTambahanTidakRetry(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	defer srv.Close()
	s := &AIService{Provider: ProviderZAI, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	if _, err := s.CompleteTolerant(ChatRequest{Provider: ProviderZAI, Model: "m",
		Messages: []ChatMessage{{Role: "user", Content: "halo"}}}); err == nil {
		t.Fatal("400 tanpa fitur tambahan harus tetap error")
	}
	if calls != 1 {
		t.Errorf("tak ada alasan retry, calls=%d", calls)
	}
}

// Temuan reviewer #1: web_search bukan array TIDAK boleh memecahkan jawaban.
func TestWebSearchBentukAneh(t *testing.T) {
	for nama, ws := range map[string]string{
		"objek":  `{"a":1}`,
		"string": `"x"`,
		"null":   `null`,
		"angka":  `5`,
	} {
		t.Run(nama, func(t *testing.T) {
			srv, _ := tangkapBody(t, `{"choices":[{"message":{"content":"Jawaban utuh"}}],"web_search":`+ws+`}`)
			s := &AIService{Provider: ProviderNineRouter, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
			out, err := s.Complete(ChatRequest{Provider: ProviderNineRouter, Model: "m",
				Messages: []ChatMessage{{Role: "user", Content: "halo"}}})
			if err != nil || out.Content != "Jawaban utuh" || out.Searched {
				t.Fatalf("jawaban normal rusak oleh web_search=%s: %+v err=%v", ws, out, err)
			}
		})
	}
}

// Temuan reviewer #2: narasi sebelum pencarian ("Saya cari dulu") tidak ikut terkirim.
func TestParseAnthropicBuangNarasiSebelumPencarian(t *testing.T) {
	body := `{"content":[
	 {"type":"text","text":"Saya cari dulu ya. "},
	 {"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{}},
	 {"type":"web_search_tool_result","tool_use_id":"srvtoolu_1","content":[]},
	 {"type":"text","text":"Hasilnya: 5%."}
	],"stop_reason":"end_turn","usage":{}}`
	resp, err := parseAnthropicResponse([]byte(body))
	if err != nil || resp.Content != "Hasilnya: 5%." {
		t.Errorf("narasi ikut terkirim: %q err=%v", resp.Content, err)
	}
}

// Tanpa pencarian, teks dari blok mana pun tetap dibaca (tak ada yang dibuang).
func TestParseAnthropicTanpaPencarianTeksTetapUtuh(t *testing.T) {
	body := `{"content":[{"type":"text","text":"A"},{"type":"text","text":"B"}],"usage":{}}`
	resp, _ := parseAnthropicResponse([]byte(body))
	if resp.Content != "AB" || resp.Searched {
		t.Errorf("salah: %+v", resp)
	}
}

// Temuan reviewer #3: pause_turn tanpa jawaban -> error (bisa diturunkan), bukan balasan kosong.
func TestParseAnthropicPauseTurnTanpaJawabanJadiError(t *testing.T) {
	body := `{"content":[{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{}}],"stop_reason":"pause_turn","usage":{}}`
	_, err := parseAnthropicResponse([]byte(body))
	if err == nil || !isBadRequest(err) {
		t.Fatalf("harus error bertipe 400 agar CompleteTolerant menurunkan fitur: %v", err)
	}
}

// Temuan reviewer #4: isBadRequest tak boleh salah-cocok dari isi body error lain.
func TestIsBadRequestTidakSalahCocok(t *testing.T) {
	ya := []string{"AI error 400: bad", "Anthropic error 400: {}", "Gemini error 400: x"}
	tidak := []string{
		`AI error 429: {"detail":"upstream error 400: x"}`,
		"AI error 401: error 400:",
		"AI request failed: dial tcp",
		"",
	}
	for _, s := range ya {
		if !isBadRequest(errors.New(s)) {
			t.Errorf("harus cocok: %q", s)
		}
	}
	for _, s := range tidak {
		if s != "" && isBadRequest(errors.New(s)) {
			t.Errorf("salah cocok: %q", s)
		}
	}
	if isBadRequest(nil) {
		t.Error("nil bukan 400")
	}
}

// Temuan reviewer #5: ulangan menurunkan juga system prompt (tanpa kalimat pencarian).
func TestCompleteTolerantPakaiDowngradeMessages(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if strings.Contains(string(b), `"tools"`) {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":"x"}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()
	s := &AIService{Provider: ProviderZAI, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	out, err := s.CompleteTolerant(ChatRequest{
		Provider: ProviderZAI, Model: "m", WebSearch: true,
		Messages:          []ChatMessage{{Role: "system", Content: "S+ALAT_PENCARIAN"}, {Role: "user", Content: "halo"}},
		DowngradeMessages: []ChatMessage{{Role: "system", Content: "S"}, {Role: "user", Content: "halo"}},
	})
	if err != nil || !out.Downgraded || len(bodies) != 2 {
		t.Fatalf("hasil salah: %+v err=%v n=%d", out, err, len(bodies))
	}
	if !strings.Contains(bodies[0], "ALAT_PENCARIAN") || strings.Contains(bodies[1], "ALAT_PENCARIAN") {
		t.Errorf("prompt ulangan harus tanpa kalimat pencarian:\n1=%s\n2=%s", bodies[0], bodies[1])
	}
}

// Temuan reviewer #7: label sumber dibersihkan & dipotong.
func TestAddSourceBersihkanLabel(t *testing.T) {
	got := addSource(nil, "\x1b[31mmerah\x1b[0m.com", "", "")
	if len(got) != 1 || strings.ContainsRune(got[0], 0x1b) || got[0] != "[31mmerah[0m.com" {
		t.Errorf("karakter kontrol tak dibuang: %q", got)
	}
	panjang := strings.Repeat("a", 200)
	got = addSource(nil, panjang, "", "")
	if len([]rune(got[0])) != maxSourceLabel {
		t.Errorf("label tak dipotong: %d", len([]rune(got[0])))
	}
	if got = addSource(nil, "", "https://www.detik.com/x", ""); got[0] != "detik.com" {
		t.Errorf("domain tanpa www: %v", got)
	}
}

// Provider yang tak didukung (9router) mengabaikan WebSearch=true diam-diam.
func TestNineRouterMengabaikanWebSearch(t *testing.T) {
	srv, raw := tangkapBody(t, `{"choices":[{"message":{"content":"ok"}}]}`)
	s := &AIService{Provider: ProviderNineRouter, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	if _, err := s.Complete(ChatRequest{
		Provider: ProviderNineRouter, Model: "m", WebSearch: true,
		Messages: []ChatMessage{{Role: "user", Content: "halo"}},
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(*raw), "tools") {
		t.Errorf("9router tak boleh dikirimi tools: %s", *raw)
	}
}
