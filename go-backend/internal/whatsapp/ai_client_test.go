package whatsapp

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/service"
)

// jsonSetara membandingkan dua nilai secara struktur (urutan kunci JSON tidak
// berarti — Go mengurutkan kunci map secara alfabet).
func jsonSetara(t *testing.T, a, b interface{}) bool {
	t.Helper()
	var x, y interface{}
	ra, _ := json.Marshal(a)
	rb, _ := json.Marshal(b)
	if err := json.Unmarshal(ra, &x); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rb, &y); err != nil {
		t.Fatal(err)
	}
	return reflect.DeepEqual(x, y)
}

// Regresi: tanpa gambar, body backend AI global setara dengan bentuk lama
// (messages = [{role,content string}], stream=false).
func TestBuildAIPayloadTanpaGambarSamaSepertiLama(t *testing.T) {
	got := buildAIPayload("m", "sys", "halo", nil)
	want := map[string]interface{}{
		"model": "m",
		"messages": []map[string]string{
			{"role": "system", "content": "sys"},
			{"role": "user", "content": "halo"},
		},
		"max_tokens":  2048, // guard: batas token AI Reply (reasoning butuh ruang)
		"temperature": 0.7,
		"stream":      false,
	}
	if !jsonSetara(t, got, want) {
		ra, _ := json.Marshal(got)
		rb, _ := json.Marshal(want)
		t.Errorf("body berubah:\nlama=%s\nbaru=%s", rb, ra)
	}
}

func TestBuildAIPayloadDenganGambarMenjadiImageURL(t *testing.T) {
	img := service.ChatImage{MIME: "image/png", Data: []byte{0x89, 'P', 'N', 'G'}}
	raw, _ := json.Marshal(buildAIPayload("m", "sys", "lihat", []service.ChatImage{img}))

	var body struct {
		Stream   bool `json:"stream"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body.Stream || len(body.Messages) != 2 {
		t.Fatalf("stream/jumlah pesan salah: %s", raw)
	}
	var parts []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ImageURL struct {
			URL string `json:"url"`
		} `json:"image_url"`
	}
	if err := json.Unmarshal(body.Messages[1].Content, &parts); err != nil {
		t.Fatalf("content user harus array: %v (%s)", err, body.Messages[1].Content)
	}
	if len(parts) != 2 || parts[0].Type != "text" || parts[0].Text != "lihat" {
		t.Fatalf("bagian teks salah: %+v", parts)
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL.URL != "data:image/png;base64,iVBORw==" {
		t.Errorf("bagian gambar salah: %+v", parts[1])
	}
}
