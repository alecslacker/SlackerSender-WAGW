package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

var fotoUji = ChatImage{MIME: "image/jpeg", Data: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x01, 0x02, 0x03}}

func b64Uji() string { return base64.StdEncoding.EncodeToString(fotoUji.Data) }

// tangkapBody menjalankan server uji yang merekam body request dan membalas respons.
func tangkapBody(t *testing.T, balasan string) (*httptest.Server, *[]byte) {
	t.Helper()
	var got []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(balasan))
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func TestOpenAICompatibleMengirimGambarSebagaiImageURL(t *testing.T) {
	srv, got := tangkapBody(t, `{"choices":[{"message":{"content":"ok"}}]}`)
	s := &AIService{HTTPClient: srv.Client()}
	_, err := s.postChatCompletions(srv.URL, "k", "m", []ChatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "ini apa?", Images: []ChatImage{fotoUji}},
	}, 100, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(*got, &body); err != nil {
		t.Fatal(err)
	}
	// system tetap string
	var sys string
	if err := json.Unmarshal(body.Messages[0].Content, &sys); err != nil || sys != "sys" {
		t.Fatalf("system harus tetap string, dapat %s", body.Messages[0].Content)
	}
	// user = array [text, image_url]
	var parts []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ImageURL struct {
			URL string `json:"url"`
		} `json:"image_url"`
	}
	if err := json.Unmarshal(body.Messages[1].Content, &parts); err != nil {
		t.Fatalf("user harus array: %v (%s)", err, body.Messages[1].Content)
	}
	if len(parts) != 2 || parts[0].Type != "text" || parts[0].Text != "ini apa?" {
		t.Fatalf("bagian teks salah: %+v", parts)
	}
	want := "data:image/jpeg;base64," + b64Uji()
	if parts[1].Type != "image_url" || parts[1].ImageURL.URL != want {
		t.Errorf("image_url salah: %+v", parts[1])
	}
}

func TestOpenAICompatibleGambarTanpaTeksTidakKirimBlokTeksKosong(t *testing.T) {
	out := OpenAIMessages([]ChatMessage{{Role: "user", Images: []ChatImage{fotoUji}}})
	raw, _ := json.Marshal(out)
	if bytes.Contains(raw, []byte(`"type":"text"`)) {
		t.Errorf("blok teks kosong ikut terkirim: %s", raw)
	}
}

// Regresi: tanpa gambar, JSON "messages" HARUS identik dengan marshal langsung
// (perilaku sebelum fitur vision).
func TestTanpaGambarBodyOpenAITidakBerubah(t *testing.T) {
	msgs := []ChatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "halo"},
		{Role: "assistant", Content: "hai"},
		{Role: "user", Content: "harga?"},
	}
	lama, _ := json.Marshal(msgs)
	baru, _ := json.Marshal(OpenAIMessages(msgs))
	if !bytes.Equal(lama, baru) {
		t.Errorf("body berubah:\nlama=%s\nbaru=%s", lama, baru)
	}
}

func TestGambarHanyaMenempelDiPesanTerakhir(t *testing.T) {
	out := OpenAIMessages([]ChatMessage{
		{Role: "user", Content: "lama"},
		{Role: "assistant", Content: "jawab"},
		{Role: "user", Content: "baru", Images: []ChatImage{fotoUji}},
	}).([]map[string]interface{})
	if _, ok := out[0]["content"].(string); !ok {
		t.Errorf("riwayat harus tetap string: %#v", out[0]["content"])
	}
	if _, ok := out[2]["content"].([]interface{}); !ok {
		t.Errorf("pesan terakhir harus array: %#v", out[2]["content"])
	}
}

func TestAnthropicMessagesGambarBase64(t *testing.T) {
	sys, msgs := anthropicMessages([]ChatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "tolong lihat", Images: []ChatImage{fotoUji}},
	})
	if sys != "sys" || len(msgs) != 1 {
		t.Fatalf("system/pesan salah: %q %d", sys, len(msgs))
	}
	raw, _ := json.Marshal(msgs[0])
	var m struct {
		Role    string `json:"role"`
		Content []struct {
			Type   string `json:"type"`
			Text   string `json:"text"`
			Source struct {
				Type      string `json:"type"`
				MediaType string `json:"media_type"`
				Data      string `json:"data"`
			} `json:"source"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("%v (%s)", err, raw)
	}
	if len(m.Content) != 2 || m.Content[0].Type != "image" ||
		m.Content[0].Source.Type != "base64" ||
		m.Content[0].Source.MediaType != "image/jpeg" ||
		m.Content[0].Source.Data != b64Uji() {
		t.Errorf("blok gambar salah: %s", raw)
	}
	if m.Content[1].Type != "text" || m.Content[1].Text != "tolong lihat" {
		t.Errorf("blok teks salah: %s", raw)
	}
}

func TestAnthropicTanpaGambarContentTetapString(t *testing.T) {
	_, msgs := anthropicMessages([]ChatMessage{{Role: "user", Content: "halo"}})
	if s, ok := msgs[0]["content"].(string); !ok || s != "halo" {
		t.Errorf("content harus string: %#v", msgs[0]["content"])
	}
}

func TestGeminiMengirimInlineData(t *testing.T) {
	srv, got := tangkapBody(t, `{"candidates":[{"content":{"parts":[{"text":"foto kucing"}]}}]}`)
	s := &AIService{Provider: ProviderGemini, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	resp, err := s.Complete(ChatRequest{
		Provider: ProviderGemini,
		Model:    "gemini-x",
		Messages: []ChatMessage{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "apa ini?", Images: []ChatImage{fotoUji}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "foto kucing" {
		t.Errorf("balasan salah: %q", resp.Content)
	}
	var body struct {
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text       string `json:"text"`
				InlineData struct {
					MIME string `json:"mime_type"`
					Data string `json:"data"`
				} `json:"inline_data"`
			} `json:"parts"`
		} `json:"contents"`
		SystemInstruction struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"system_instruction"`
	}
	if err := json.Unmarshal(*got, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Contents) != 1 || len(body.Contents[0].Parts) != 2 {
		t.Fatalf("struktur salah: %s", *got)
	}
	p := body.Contents[0].Parts
	if p[0].InlineData.MIME != "image/jpeg" || p[0].InlineData.Data != b64Uji() {
		t.Errorf("inline_data salah: %+v", p[0])
	}
	if p[1].Text != "apa ini?" {
		t.Errorf("teks salah: %+v", p[1])
	}
	if len(body.SystemInstruction.Parts) != 1 || body.SystemInstruction.Parts[0].Text != "sys" {
		t.Errorf("system_instruction salah: %s", *got)
	}
}

func TestGeminiTanpaGambarBodyTidakBerubah(t *testing.T) {
	srv, got := tangkapBody(t, `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`)
	s := &AIService{Provider: ProviderGemini, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	if _, err := s.Complete(ChatRequest{
		Provider: ProviderGemini,
		Messages: []ChatMessage{{Role: "user", Content: "halo"}, {Role: "assistant", Content: "hai"}, {Role: "user", Content: "lagi"}},
	}); err != nil {
		t.Fatal(err)
	}
	want := `"contents":[{"role":"user","parts":[{"text":"halo"}]},{"role":"model","parts":[{"text":"hai"}]},{"role":"user","parts":[{"text":"lagi"}]}]`
	if !bytes.Contains(*got, []byte(want)) {
		t.Errorf("bentuk contents berubah:\n%s", *got)
	}
}
