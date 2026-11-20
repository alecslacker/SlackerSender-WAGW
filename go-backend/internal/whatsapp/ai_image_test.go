package whatsapp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/service"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

// Header minimal tiap format (cukup untuk sniffing http.DetectContentType).
var (
	hdrJPEG = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	hdrPNG  = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	hdrWEBP = append(append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), make([]byte, 8)...), 0x00)
	hdrGIF  = []byte("GIF89a\x01\x00\x01\x00")
	hdrHTML = []byte("<!DOCTYPE html><html><body>x</body></html>")
	hdrPDF  = []byte("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")
)

func TestShouldHandleForAI(t *testing.T) {
	cases := []struct {
		nama                         string
		text                         string
		hasImage, readImages, expect bool
	}{
		{"teks biasa", "halo", false, false, true},
		{"teks biasa, ReadImages aktif", "halo", false, true, true},
		{"caption + ReadImages mati (perilaku lama)", "harga?", true, false, true},
		{"caption + ReadImages aktif", "harga?", true, true, true},
		{"gambar tanpa caption + ReadImages aktif", "", true, true, true},
		{"gambar tanpa caption + ReadImages mati (perilaku lama: diabaikan)", "", true, false, false},
		{"teks spasi saja + gambar + aktif", "   \n", true, true, true},
		{"teks spasi saja + gambar + mati", "   ", true, false, false},
		{"tanpa teks, bukan gambar, aktif", "", false, true, false},
		{"tanpa teks, bukan gambar, mati", "", false, false, false},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			if got := shouldHandleForAI(c.text, c.hasImage, c.readImages); got != c.expect {
				t.Errorf("shouldHandleForAI(%q,%v,%v)=%v, mau %v", c.text, c.hasImage, c.readImages, got, c.expect)
			}
		})
	}
}

func TestValidateAIImage(t *testing.T) {
	ok := map[string]struct {
		data []byte
		mime string
	}{
		"jpeg": {hdrJPEG, "image/jpeg"},
		"png":  {hdrPNG, "image/png"},
		"webp": {hdrWEBP, "image/webp"},
	}
	for nama, c := range ok {
		t.Run("lolos "+nama, func(t *testing.T) {
			img, err := validateAIImage(c.data)
			if err != nil {
				t.Fatalf("seharusnya lolos: %v", err)
			}
			if img.MIME != c.mime || !bytes.Equal(img.Data, c.data) {
				t.Errorf("hasil salah: mime=%q", img.MIME)
			}
		})
	}

	tolak := map[string][]byte{
		"gif (Gemini tak menerima)": hdrGIF,
		"html (file aktif)":         hdrHTML,
		"pdf":                       hdrPDF,
		"kosong":                    {},
		"nil":                       nil,
		"biner acak":                bytes.Repeat([]byte{0x01, 0x02, 0x03}, 50),
		// Klaim ekstensi/mime pengirim tak dipakai: isi HTML tetap ditolak walau
		// diawali spasi.
		"html bersembunyi": append([]byte("   \n"), hdrHTML...),
	}
	for nama, data := range tolak {
		t.Run("tolak "+nama, func(t *testing.T) {
			if _, err := validateAIImage(data); err == nil {
				t.Errorf("seharusnya ditolak")
			}
		})
	}
}

func TestValidateAIImageBatasUkuran(t *testing.T) {
	pas := append(append([]byte{}, hdrJPEG...), make([]byte, aiImageMaxBytes-len(hdrJPEG))...)
	if len(pas) != aiImageMaxBytes {
		t.Fatalf("fixture salah: %d", len(pas))
	}
	if _, err := validateAIImage(pas); err != nil {
		t.Errorf("tepat di batas harus lolos: %v", err)
	}
	lebih := append(pas, 0x00)
	if _, err := validateAIImage(lebih); err == nil {
		t.Errorf("1 byte di atas batas harus ditolak")
	}
}

func TestIsImageMessage(t *testing.T) {
	img := &events.Message{Message: &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}}
	vid := &events.Message{Message: &waE2E.Message{VideoMessage: &waE2E.VideoMessage{}}}
	stk := &events.Message{Message: &waE2E.Message{StickerMessage: &waE2E.StickerMessage{}}}
	txt := &events.Message{Message: &waE2E.Message{Conversation: strPtr("halo")}}
	for nama, c := range map[string]struct {
		msg    *events.Message
		expect bool
	}{
		"gambar": {img, true}, "video": {vid, false}, "stiker": {stk, false},
		"teks": {txt, false}, "msg nil": {nil, false}, "Message nil": {&events.Message{}, false},
	} {
		if got := isImageMessage(c.msg); got != c.expect {
			t.Errorf("%s: dapat %v, mau %v", nama, got, c.expect)
		}
	}
}

func strPtr(s string) *string { return &s }

func TestAIAuditQuestion(t *testing.T) {
	cases := []struct {
		text      string
		withImage bool
		expect    string
	}{
		{"halo", false, "halo"},
		{"  halo  ", false, "halo"},
		{"", false, ""}, // log membuang pertanyaan kosong (perilaku lama)
		{"harga ini?", true, "[gambar] harga ini?"},
		{"", true, "[gambar]"},
		{"   ", true, "[gambar]"},
	}
	for _, c := range cases {
		if got := aiAuditQuestion(c.text, c.withImage); got != c.expect {
			t.Errorf("aiAuditQuestion(%q,%v)=%q, mau %q", c.text, c.withImage, got, c.expect)
		}
	}
}

func TestComposeAIPrompt(t *testing.T) {
	t.Run("tanpa gambar, guard aktif = persis perilaku lama", func(t *testing.T) {
		sys, usr := composeAIPrompt("peran", "halo", true, false)
		if sys != service.GuardedSystemPrompt("peran") || usr != service.GuardedUserMessage("halo") {
			t.Errorf("berubah dari perilaku lama:\nsys=%q\nusr=%q", sys, usr)
		}
		if strings.Contains(sys, service.ImageGuardSuffix) {
			t.Error("ImageGuardSuffix tak boleh muncul tanpa gambar")
		}
	})
	t.Run("tanpa gambar, guard mati = teks apa adanya", func(t *testing.T) {
		sys, usr := composeAIPrompt("peran", "halo", false, false)
		if sys != "peran" || usr != "halo" {
			t.Errorf("sys=%q usr=%q", sys, usr)
		}
	})
	t.Run("gambar + caption, guard aktif", func(t *testing.T) {
		sys, usr := composeAIPrompt("peran", "ini apa?", true, true)
		if !strings.Contains(sys, service.InjectionGuardSuffix) || !strings.Contains(sys, service.ImageGuardSuffix) {
			t.Errorf("kedua guard harus ada: %q", sys)
		}
		if usr != service.GuardedUserMessage("ini apa?") {
			t.Errorf("user salah: %q", usr)
		}
	})
	t.Run("gambar tanpa caption diberi pengantar (tidak kosong)", func(t *testing.T) {
		_, usr := composeAIPrompt("peran", "  ", true, true)
		if !strings.Contains(usr, imageOnlyPlaceholder) {
			t.Errorf("pengantar hilang: %q", usr)
		}
		_, usr2 := composeAIPrompt("peran", "", false, true)
		if usr2 != imageOnlyPlaceholder {
			t.Errorf("guard mati: %q", usr2)
		}
	})
	t.Run("gambar, guard mati: tanpa ImageGuardSuffix", func(t *testing.T) {
		// Pemilik config mematikan proteksi injection → hormati pilihannya.
		sys, _ := composeAIPrompt("peran", "ini apa?", false, true)
		if sys != "peran" {
			t.Errorf("sys=%q", sys)
		}
	})
}
