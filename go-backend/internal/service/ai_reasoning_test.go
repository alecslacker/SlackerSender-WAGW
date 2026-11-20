package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Bentuk respons model reasoning (GLM-5.x) yang kehabisan token: content kosong,
// reasoning_content terisi, finish_reason "length". Inilah yang membuat AI Reply
// "tidak menjawab" tanpa error apa pun.
const reasoningHabisJSON = `{"id":"x","choices":[{"index":0,"message":{"role":"assistant","content":"","reasoning_content":"Pengguna bertanya singkat, saya perlu memikirkan..."},"finish_reason":"length"}],"usage":{"total_tokens":512}}`

func TestJawabanKosongModelReasoningTerjelaskan(t *testing.T) {
	srv, _ := tangkapBody(t, reasoningHabisJSON)
	s := &AIService{HTTPClient: srv.Client()}
	resp, err := s.postChatCompletions(srv.URL, "k", "glm-5.3", []ChatMessage{{Role: "user", Content: "halo"}}, 500, 0.7)
	if err != nil {
		t.Fatalf("content kosong bukan error transport: %v", err)
	}
	if resp.Content != "" || !resp.Reasoning || resp.FinishReason != "length" {
		t.Fatalf("respons salah dibaca: %+v", resp)
	}
	if h := resp.EmptyHint(); !strings.Contains(h, "berpikir") {
		t.Errorf("petunjuk harus menyebut berpikir, dapat %q", h)
	}
}

func TestEmptyHintCabang(t *testing.T) {
	cases := []struct {
		r    ChatResponse
		want string
	}{
		{ChatResponse{FinishReason: "length", Reasoning: true}, "berpikir"},
		{ChatResponse{FinishReason: "length"}, "max_tokens"},
		{ChatResponse{FinishReason: "stop"}, "finish_reason=stop"},
		{ChatResponse{}, "tidak mengirim finish_reason"},
	}
	for _, c := range cases {
		if got := c.r.EmptyHint(); !strings.Contains(got, c.want) {
			t.Errorf("%+v -> %q, mau memuat %q", c.r, got, c.want)
		}
	}
}

func TestSSEKosongTapiAdaReasoningErrorDenganSebab(t *testing.T) {
	body := []byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"mikir\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n")
	_, err := parseSSEChatResponse(body)
	if err == nil {
		t.Fatal("perilaku lama dipertahankan: stream tanpa content tetap error")
	}
	if !strings.Contains(err.Error(), "berpikir") {
		t.Errorf("error harus menjelaskan sebab, dapat: %v", err)
	}
}

func TestSSEBenarBenarKosongTetapError(t *testing.T) {
	if _, err := parseSSEChatResponse([]byte("data: [DONE]\n")); err == nil {
		t.Error("stream tanpa apa pun harus tetap error (perilaku lama)")
	}
}

func TestSSEBerisiJawabanMembawaFinishReason(t *testing.T) {
	body := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"Halo\"}}]}\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\" kak\"},\"finish_reason\":\"stop\"}]}\n")
	resp, err := parseSSEChatResponse(body)
	if err != nil || resp.Content != "Halo kak" || resp.FinishReason != "stop" || resp.Reasoning {
		t.Errorf("hasil salah: %+v err=%v", resp, err)
	}
}

func TestReasoningEffortFor(t *testing.T) {
	cases := []struct {
		p     AIProvider
		model string
		want  string
	}{
		{ProviderZAI, "glm-5.3", "low"},
		{ProviderZAI, "GLM-5.3-FLASH", "low"},
		{ProviderZAI, " glm-5.3-flash ", "low"},
		{ProviderZAI, "glm-5.1", ""},        // belum terdokumentasi menerima reasoning_effort
		{ProviderZAI, "glm-4.5", ""},        // bukan GLM-5.x: jangan kirim field baru
		{ProviderNineRouter, "glm-5.3", ""}, // 9router: belum tentu meneruskan
		{ProviderOpenAI, "gpt-4o", ""},      // provider lain tak dikirimi
		{ProviderCustom, "glm-5.3", ""},     // endpoint bebas: tak dikirimi
		{ProviderZAI, "", ""},
	}
	for _, c := range cases {
		if got := ReasoningEffortFor(c.p, c.model); got != c.want {
			t.Errorf("ReasoningEffortFor(%s,%q)=%q, mau %q", c.p, c.model, got, c.want)
		}
	}
}

func TestReasoningEffortHanyaTerkirimBilaDiminta(t *testing.T) {
	var got map[string]interface{}
	for _, c := range []struct {
		nama     string
		effort   string
		adaField bool
	}{{"zai low", "low", true}, {"tanpa effort", "", false}} {
		t.Run(c.nama, func(t *testing.T) {
			srv, raw := tangkapBody(t, `{"choices":[{"message":{"content":"ok"}}]}`)
			s := &AIService{Provider: ProviderZAI, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
			if _, err := s.Complete(ChatRequest{
				Provider: ProviderZAI, Model: "glm-5.3", ReasoningEffort: c.effort,
				Messages: []ChatMessage{{Role: "user", Content: "halo"}}, MaxTokens: 2048,
			}); err != nil {
				t.Fatal(err)
			}
			got = nil
			if err := json.Unmarshal(*raw, &got); err != nil {
				t.Fatal(err)
			}
			v, ada := got["reasoning_effort"]
			if ada != c.adaField || (ada && v != c.effort) {
				t.Errorf("reasoning_effort=%v ada=%v, mau ada=%v (%s)", v, ada, c.adaField, *raw)
			}
			if got["max_tokens"] != float64(2048) {
				t.Errorf("max_tokens=%v, mau 2048", got["max_tokens"])
			}
		})
	}
}

// Jaring pengaman: provider menolak reasoning_effort (HTTP 400) -> ulangi sekali
// tanpa field itu, balasan tetap sampai.
func TestCompleteTolerantRetryTanpaReasoningEffortBila400(t *testing.T) {
	var bodies []string
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if strings.Contains(string(b), "reasoning_effort") {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":"unknown field reasoning_effort"}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"halo kak"}}]}`))
	}))
	defer srv.Close()
	s := &AIService{Provider: ProviderZAI, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	resp, err := s.CompleteTolerant(ChatRequest{
		Provider: ProviderZAI, Model: "glm-5.3", ReasoningEffort: "low",
		Messages: []ChatMessage{{Role: "user", Content: "halo"}}, MaxTokens: 2048,
	})
	if err != nil || resp.Content != "halo kak" {
		t.Fatalf("seharusnya pulih: resp=%+v err=%v", resp, err)
	}
	if calls != 2 || strings.Contains(bodies[1], "reasoning_effort") {
		t.Errorf("mau 2 panggilan, yang kedua tanpa field; calls=%d", calls)
	}
}

func TestCompleteTolerantTidakRetryUntukErrorLain(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer srv.Close()
	s := &AIService{Provider: ProviderZAI, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	if _, err := s.CompleteTolerant(ChatRequest{
		Provider: ProviderZAI, Model: "glm-5.3", ReasoningEffort: "low",
		Messages: []ChatMessage{{Role: "user", Content: "halo"}},
	}); err == nil {
		t.Fatal("401 harus tetap error")
	}
	if calls != 1 {
		t.Errorf("401 tak boleh di-retry, calls=%d", calls)
	}
}

func TestCompleteTolerantTanpaReasoningEffortTidakRetry(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	defer srv.Close()
	s := &AIService{Provider: ProviderNineRouter, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, _ = s.CompleteTolerant(ChatRequest{
		Provider: ProviderNineRouter, Model: "m",
		Messages: []ChatMessage{{Role: "user", Content: "halo"}},
	})
	if calls != 1 {
		t.Errorf("tanpa reasoning_effort tak ada alasan retry, calls=%d", calls)
	}
}
