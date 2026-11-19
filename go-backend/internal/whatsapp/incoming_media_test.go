package whatsapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// Header minimal file asli untuk sniffing mime.
var (
	pngHead = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00")
	jpgHead = []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00")
	pdfHead = []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n1 0 obj\n")
	zipHead = []byte("PK\x03\x04\x14\x00\x06\x00\x08\x00\x00\x00!\x00")
	htmlDoc = []byte("<html><script>alert(1)</script></html>")
	exeHead = []byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff")
)

func TestExtensionForMedia(t *testing.T) {
	cases := []struct {
		nama   string
		data   []byte
		file   string
		want   string
		wantOK bool
	}{
		{"png", pngHead, "", ".png", true},
		{"jpeg", jpgHead, "", ".jpg", true},
		{"pdf", pdfHead, "laporan.pdf", ".pdf", true},
		// Ekstensi dari ISI, bukan nama file pengirim.
		{"png berkedok .html", pngHead, "x.html", ".png", true},
		{"docx (zip) nama aman", zipHead, "surat.DOCX", ".docx", true},
		{"zip nama .html ditolak", zipHead, "x.html", "", false},
		{"html ditolak", htmlDoc, "index.html", "", false},
		{"html berkedok png ditolak", htmlDoc, "foto.png", "", false},
		{"exe ditolak", exeHead, "setup.exe", "", false},
		{"teks .txt boleh", []byte("catatan biasa\n"), "catatan.txt", ".txt", true},
		{"teks tanpa ekstensi aman ditolak", []byte("catatan biasa\n"), "catatan", "", false},
		{"kosong ditolak", nil, "a.png", "", false},
	}
	for _, tc := range cases {
		got, ok := extensionForMedia(tc.data, tc.file)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("%s: dapat (%q,%v), mau (%q,%v)", tc.nama, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestSaveIncomingMedia(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "chat")
	url1, err := saveIncomingMedia(dir, pngHead, ".png")
	if err != nil {
		t.Fatalf("simpan: %v", err)
	}
	url2, _ := saveIncomingMedia(dir, pngHead, ".png")
	if url1 == url2 {
		t.Fatal("nama file harus acak/unik")
	}
	if !strings.HasPrefix(url1, "/uploads/chat/") || !strings.HasSuffix(url1, ".png") {
		t.Fatalf("URL publik tak sesuai: %s", url1)
	}
	name := strings.TrimPrefix(url1, "/uploads/chat/")
	if len(name) != 32+len(".png") {
		t.Errorf("nama harus 128-bit hex + ekstensi, dapat %q", name)
	}
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil || string(b) != string(pngHead) {
		t.Fatalf("isi file tidak sama (err=%v)", err)
	}
}

func TestIsTechnicalMessage(t *testing.T) {
	teknis := []*waE2E.Message{
		{},
		{SenderKeyDistributionMessage: &waE2E.SenderKeyDistributionMessage{}},
		{ProtocolMessage: &waE2E.ProtocolMessage{}, MessageContextInfo: &waE2E.MessageContextInfo{}},
	}
	for i, m := range teknis {
		if !isTechnicalMessage(m) {
			t.Errorf("kasus teknis #%d harus dianggap teknis", i)
		}
	}
	bukan := []*waE2E.Message{
		{Conversation: proto.String("halo")},
		{LocationMessage: &waE2E.LocationMessage{}},
		{ImageMessage: &waE2E.ImageMessage{}},
		{ContactMessage: &waE2E.ContactMessage{}, MessageContextInfo: &waE2E.MessageContextInfo{}},
	}
	for i, m := range bukan {
		if isTechnicalMessage(m) {
			t.Errorf("kasus non-teknis #%d tidak boleh diabaikan", i)
		}
	}
}

func TestUnsupportedMessageLabel(t *testing.T) {
	cases := []struct {
		nama string
		m    *waE2E.Message
		want string
	}{
		{"lokasi", &waE2E.Message{LocationMessage: &waE2E.LocationMessage{Name: proto.String("Monas")}}, "📍 Lokasi: Monas"},
		{"lokasi tanpa nama", &waE2E.Message{LocationMessage: &waE2E.LocationMessage{}}, "📍 Lokasi"},
		{"kontak", &waE2E.Message{ContactMessage: &waE2E.ContactMessage{DisplayName: proto.String("Budi")}}, "👤 Kontak: Budi"},
		{"polling", &waE2E.Message{PollCreationMessage: &waE2E.PollCreationMessage{Name: proto.String("Libur?")}}, "📊 Polling: Libur?"},
		{"tombol dengan teks", &waE2E.Message{ButtonsMessage: &waE2E.ButtonsMessage{ContentText: proto.String("Pilih menu")}}, "Pilih menu"},
		{"tak dikenal", &waE2E.Message{}, "Pesan tidak didukung"},
		{"nil", nil, "Pesan tidak didukung"},
	}
	for _, tc := range cases {
		if got := unsupportedMessageLabel(tc.m); got != tc.want {
			t.Errorf("%s: dapat %q, mau %q", tc.nama, got, tc.want)
		}
	}
}

func TestIncomingMediaOf(t *testing.T) {
	if _, ok := incomingMediaOf(&waE2E.Message{Conversation: proto.String("x")}); ok {
		t.Error("teks biasa bukan media")
	}
	if _, ok := incomingMediaOf(nil); ok {
		t.Error("nil bukan media")
	}
	for nama, m := range map[string]*waE2E.Message{
		"image":    {ImageMessage: &waE2E.ImageMessage{}},
		"video":    {VideoMessage: &waE2E.VideoMessage{}},
		"audio":    {AudioMessage: &waE2E.AudioMessage{}},
		"sticker":  {StickerMessage: &waE2E.StickerMessage{}},
		"document": {DocumentMessage: &waE2E.DocumentMessage{FileName: proto.String("a.pdf")}},
	} {
		if _, ok := incomingMediaOf(m); !ok {
			t.Errorf("%s harus dikenali sebagai media", nama)
		}
	}
	if info, _ := incomingMediaOf(&waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{FileName: proto.String("a.pdf")}}); info.fileName != "a.pdf" {
		t.Errorf("nama file dokumen tak terbawa: %q", info.fileName)
	}
}
