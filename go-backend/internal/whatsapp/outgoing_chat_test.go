package whatsapp

import (
	"testing"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"go.mau.fi/whatsmeow/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func outgoingTestManager(t *testing.T) (*Manager, *SessionState) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ChatInbox{}, &models.ChatConversation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	m := &Manager{db: db, sessions: map[uint]*SessionState{}}
	return m, &SessionState{DeviceID: 1, UserID: 5}
}

func countOut(m *Manager) int64 {
	var n int64
	m.db.Model(&models.ChatInbox{}).Where("direction = ?", "out").Count(&n)
	return n
}

func TestRecordOutgoingChatMencatatBalasan(t *testing.T) {
	m, sess := outgoingTestManager(t)
	jid := types.NewJID("6285777778781", types.DefaultUserServer)

	m.recordOutgoingChat(sess, jid, "WAID1", SendOptions{Type: "text", Content: "Halo juga.. ada yang bisa dibantu?"})

	var in models.ChatInbox
	if err := m.db.Where("direction = ?", "out").First(&in).Error; err != nil {
		t.Fatalf("pesan keluar tidak tercatat: %v", err)
	}
	if in.Phone != "6285777778781" || in.Content != "Halo juga.. ada yang bisa dibantu?" || in.Type != "text" || !in.IsRead || in.WaMessageID != "WAID1" {
		t.Errorf("isi baris salah: %+v", in)
	}
	var conv models.ChatConversation
	if err := m.db.Where("phone = ?", "6285777778781").First(&conv).Error; err != nil {
		t.Fatalf("percakapan tidak dibuat: %v", err)
	}
	if conv.LastMessage != "Halo juga.. ada yang bisa dibantu?" || conv.UnreadCount != 0 {
		t.Errorf("ringkasan percakapan salah: %+v", conv)
	}
}

func TestRecordOutgoingChatTidakDobel(t *testing.T) {
	m, sess := outgoingTestManager(t)
	jid := types.NewJID("6285777778781", types.DefaultUserServer)
	opts := SendOptions{Type: "text", Content: "sekali"}

	m.recordOutgoingChat(sess, jid, "WAID-SAMA", opts)
	m.recordOutgoingChat(sess, jid, "WAID-SAMA", opts)
	if n := countOut(m); n != 1 {
		t.Errorf("WA ID sama dicatat %d kali, mau 1", n)
	}
	m.recordOutgoingChat(sess, jid, "WAID-LAIN", opts)
	if n := countOut(m); n != 2 {
		t.Errorf("WA ID berbeda harus dicatat terpisah, dapat %d", n)
	}
}

func TestRecordOutgoingChatAbaikanGrupDanBroadcast(t *testing.T) {
	m, sess := outgoingTestManager(t)
	m.recordOutgoingChat(sess, types.NewJID("120363000000000000", types.GroupServer), "G1", SendOptions{Type: "text", Content: "x"})
	m.recordOutgoingChat(sess, types.StatusBroadcastJID, "B1", SendOptions{Type: "text", Content: "x"})
	if n := countOut(m); n != 0 {
		t.Errorf("grup/broadcast tidak boleh dicatat, tercatat %d", n)
	}
}

func TestRecordOutgoingChatLIDMasukKePercakapanNomorAsli(t *testing.T) {
	m, sess := outgoingTestManager(t)
	// Percakapan sudah berkunci nomor asli, SenderJID menyimpan LID-nya.
	m.db.Create(&models.ChatConversation{
		UserID: 5, DeviceID: 1, Phone: "628888001337", SenderJID: "200506739830854@lid",
	})
	// Suffix perangkat pada JID tujuan tidak boleh mengacaukan pemetaan.
	jid := types.JID{User: "200506739830854", Server: types.HiddenUserServer, Device: 81}

	m.recordOutgoingChat(sess, jid, "WAID-LID", SendOptions{Type: "text", Content: "balasan"})

	var in models.ChatInbox
	if err := m.db.Where("direction = ?", "out").First(&in).Error; err != nil {
		t.Fatalf("tidak tercatat: %v", err)
	}
	if in.Phone != "628888001337" {
		t.Errorf("Phone = %q, mau 628888001337 (percakapan nomor asli)", in.Phone)
	}
	var n int64
	m.db.Model(&models.ChatConversation{}).Count(&n)
	if n != 1 {
		t.Errorf("harus tetap 1 percakapan, dapat %d", n)
	}
}

func TestRecordOutgoingChatMedia(t *testing.T) {
	m, sess := outgoingTestManager(t)
	jid := types.NewJID("6285777778781", types.DefaultUserServer)

	// URL publik tersimpan; path file:// (milik server) tidak boleh bocor ke klien.
	m.recordOutgoingChat(sess, jid, "M1", SendOptions{Type: "image", Content: "keterangan", MediaURL: "https://contoh.com/a.jpg"})
	m.recordOutgoingChat(sess, jid, "M2", SendOptions{Type: "image", MediaURL: "file://./public/uploads/chat/x.jpg"})

	var a, b models.ChatInbox
	m.db.Where("wa_message_id = ?", "M1").First(&a)
	m.db.Where("wa_message_id = ?", "M2").First(&b)
	if a.MediaURL != "https://contoh.com/a.jpg" || a.Type != "image" {
		t.Errorf("media publik salah: %+v", a)
	}
	if b.MediaURL != "" {
		t.Errorf("path file:// bocor ke chat: %q", b.MediaURL)
	}
	var conv models.ChatConversation
	m.db.Where("phone = ?", "6285777778781").First(&conv)
	if conv.LastMessageType != "image" || conv.LastMessage == "" {
		t.Errorf("ringkasan media salah: %+v", conv)
	}
}

func TestOutgoingChatTextDanTipe(t *testing.T) {
	cases := []struct {
		opts     SendOptions
		wantText string
		wantType string
	}{
		{SendOptions{Type: "text", Content: "halo"}, "halo", "text"},
		{SendOptions{Type: "", Content: "halo"}, "halo", "text"},
		{SendOptions{Type: "interactive", Content: "pilih menu"}, "pilih menu", "text"},
		{SendOptions{Type: "poll", Content: "Libur?"}, "📊 Libur?", "text"},
		{SendOptions{Type: "location", LocName: "Monas"}, "📍 Monas", "text"},
		{SendOptions{Type: "location"}, "📍 Lokasi", "text"},
		{SendOptions{Type: "document", Content: "laporan.pdf"}, "", "document"},
		{SendOptions{Type: "voice"}, "", "voicenote"},
		{SendOptions{Type: "image", Content: "cap"}, "cap", "image"},
	}
	for _, tc := range cases {
		if got := outgoingChatText(tc.opts); got != tc.wantText {
			t.Errorf("outgoingChatText(%+v) = %q, mau %q", tc.opts, got, tc.wantText)
		}
		if got := outgoingChatType(tc.opts.Type); got != tc.wantType {
			t.Errorf("outgoingChatType(%q) = %q, mau %q", tc.opts.Type, got, tc.wantType)
		}
	}
}
