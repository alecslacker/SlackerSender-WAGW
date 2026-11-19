// Penanganan isi pesan masuk untuk Live Chat: unduh media, label pesan yang
// belum didukung, dan penyaringan pesan teknis.
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
package whatsapp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/realtime"
	"github.com/rs/zerolog/log"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

const (
	// incomingMediaMaxBytes = batas ukuran media masuk yang disimpan
	// (sama dengan batas upload Live Chat: 16 MB).
	incomingMediaMaxBytes = 16 << 20
	incomingMediaTimeout  = 60 * time.Second
	// incomingMediaPublicPrefix = URL publik file di bawah ChatUploadsDir
	// (disajikan r.Static("/uploads", "./public/uploads")).
	incomingMediaPublicPrefix = "/uploads/chat/"
)

// incomingMediaSlots membatasi unduhan media masuk yang berjalan bersamaan
// agar banjir media tidak menghabiskan memori/koneksi server.
var incomingMediaSlots = make(chan struct{}, 4)

// incomingMimeExt: mime hasil SNIFFING isi file → ekstensi aman. Ekstensi
// tidak pernah diambil dari nama file kiriman pengirim (cegah file aktif
// seperti .html disajikan dari domain kita).
var incomingMimeExt = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"video/mp4":       ".mp4",
	"video/webm":      ".webm",
	"audio/mpeg":      ".mp3",
	"audio/ogg":       ".ogg",
	"application/ogg": ".ogg",
	"audio/webm":      ".webm",
	"application/pdf": ".pdf",
}

// Dokumen yang isinya terdeteksi generik (zip/biner/teks): ekstensi nama file
// pengirim hanya dipakai bila termasuk daftar aman di bawah.
var (
	incomingBinaryDocExt = map[string]bool{".docx": true, ".xlsx": true, ".pptx": true, ".zip": true, ".doc": true, ".xls": true, ".ppt": true}
	incomingTextDocExt   = map[string]bool{".txt": true, ".csv": true}
)

// technicalMessageFields: field protobuf yang bukan isi percakapan (kunci
// enkripsi grup, protokol internal WA, konteks).
var technicalMessageFields = map[string]bool{
	"protocolMessage":              true,
	"senderKeyDistributionMessage": true,
	"messageContextInfo":           true,
}

// isTechnicalMessage true bila pesan HANYA berisi field teknis (tanpa teks,
// media, lokasi, dll) — tidak layak masuk Live Chat.
func isTechnicalMessage(m *waE2E.Message) bool {
	for _, n := range messageFieldNames(m) {
		if !technicalMessageFields[n] {
			return false
		}
	}
	return true
}

// unsupportedMessageLabel memberi teks tampilan untuk pesan tanpa teks
// biasa yang belum didukung penuh (lokasi, kontak, polling, tombol, dst).
// Hanya untuk tampilan Live Chat — alur bot/AI tetap memakai teks asli.
func unsupportedMessageLabel(m *waE2E.Message) string {
	if m == nil {
		return "Pesan tidak didukung"
	}
	withName := func(prefix, name string) string {
		name = strings.TrimSpace(name)
		if name == "" {
			return prefix
		}
		return prefix + ": " + truncate(name, 120)
	}
	switch {
	case m.GetLocationMessage() != nil:
		return withName("📍 Lokasi", m.GetLocationMessage().GetName())
	case m.GetLiveLocationMessage() != nil:
		return withName("📍 Lokasi langsung", m.GetLiveLocationMessage().GetCaption())
	case m.GetContactMessage() != nil:
		return withName("👤 Kontak", m.GetContactMessage().GetDisplayName())
	case m.GetContactsArrayMessage() != nil:
		a := m.GetContactsArrayMessage()
		if n := strings.TrimSpace(a.GetDisplayName()); n != "" {
			return withName("👤 Kontak", n)
		}
		return fmt.Sprintf("👤 %d kontak", len(a.GetContacts()))
	case m.GetPollCreationMessage() != nil:
		return withName("📊 Polling", m.GetPollCreationMessage().GetName())
	case m.GetPollCreationMessageV2() != nil:
		return withName("📊 Polling", m.GetPollCreationMessageV2().GetName())
	case m.GetPollCreationMessageV3() != nil:
		return withName("📊 Polling", m.GetPollCreationMessageV3().GetName())
	case m.GetButtonsMessage() != nil:
		if t := strings.TrimSpace(m.GetButtonsMessage().GetContentText()); t != "" {
			return t
		}
	case m.GetListMessage() != nil:
		l := m.GetListMessage()
		t := strings.TrimSpace(l.GetTitle())
		if d := strings.TrimSpace(l.GetDescription()); d != "" {
			if t != "" {
				t += "\n"
			}
			t += d
		}
		if t != "" {
			return t
		}
	case m.GetTemplateMessage() != nil:
		if t := strings.TrimSpace(m.GetTemplateMessage().GetHydratedTemplate().GetHydratedContentText()); t != "" {
			return t
		}
	case m.GetInteractiveMessage() != nil:
		if t := strings.TrimSpace(m.GetInteractiveMessage().GetBody().GetText()); t != "" {
			return t
		}
	case m.GetGroupInviteMessage() != nil:
		return withName("👥 Undangan grup", m.GetGroupInviteMessage().GetGroupName())
	case m.GetProductMessage() != nil:
		return "🛍️ Produk"
	case m.GetOrderMessage() != nil:
		return "🛒 Pesanan"
	case m.GetEventMessage() != nil:
		return "📅 Acara"
	}
	return "Pesan tidak didukung"
}

// incomingMediaInfo menggambarkan media yang bisa diunduh dari sebuah pesan.
type incomingMediaInfo struct {
	dl       whatsmeow.DownloadableMessage
	size     uint64
	fileName string
}

// incomingMediaOf mengembalikan media yang dibawa pesan (gambar, video,
// audio, dokumen, stiker); ok=false bila pesan tidak membawa media.
func incomingMediaOf(m *waE2E.Message) (incomingMediaInfo, bool) {
	if m == nil {
		return incomingMediaInfo{}, false
	}
	switch {
	case m.GetImageMessage() != nil:
		i := m.GetImageMessage()
		return incomingMediaInfo{dl: i, size: i.GetFileLength()}, true
	case m.GetVideoMessage() != nil:
		v := m.GetVideoMessage()
		return incomingMediaInfo{dl: v, size: v.GetFileLength()}, true
	case m.GetAudioMessage() != nil:
		a := m.GetAudioMessage()
		return incomingMediaInfo{dl: a, size: a.GetFileLength()}, true
	case m.GetStickerMessage() != nil:
		s := m.GetStickerMessage()
		return incomingMediaInfo{dl: s, size: s.GetFileLength()}, true
	case m.GetDocumentMessage() != nil:
		d := m.GetDocumentMessage()
		return incomingMediaInfo{dl: d, size: d.GetFileLength(), fileName: d.GetFileName()}, true
	}
	return incomingMediaInfo{}, false
}

// extensionForMedia menentukan ekstensi file dari ISI media (sniffing).
// ok=false bila tipe tidak diizinkan — file tidak disimpan.
func extensionForMedia(data []byte, fileName string) (string, bool) {
	n := len(data)
	if n > 512 {
		n = 512
	}
	sniffed := http.DetectContentType(data[:n])
	if i := strings.Index(sniffed, ";"); i >= 0 {
		sniffed = strings.TrimSpace(sniffed[:i])
	}
	if ext, ok := incomingMimeExt[sniffed]; ok {
		return ext, true
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	switch sniffed {
	case "application/zip", "application/octet-stream":
		if incomingBinaryDocExt[ext] {
			return ext, true
		}
	case "text/plain":
		if incomingTextDocExt[ext] {
			return ext, true
		}
	}
	return "", false
}

// saveIncomingMedia menyimpan data ke dir dengan nama acak 128-bit (tidak
// bisa ditebak) dan mengembalikan URL publiknya.
func saveIncomingMedia(dir string, data []byte, ext string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	var rb [16]byte
	if _, err := rand.Read(rb[:]); err != nil {
		return "", err
	}
	name := hex.EncodeToString(rb[:]) + ext
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return incomingMediaPublicPrefix + name, nil
}

// broadcastIncomingChat menyiarkan pesan masuk ke Live Chat lewat SSE
// ("chat:message") — tanpa ini pesan baru baru tampil setelah halaman dimuat ulang.
func (m *Manager) broadcastIncomingChat(sess *SessionState, inbox *models.ChatInbox) {
	realtime.DefaultHub.SendToUser(sess.UserID, realtime.Event{
		Type: "chat:message",
		Payload: map[string]interface{}{
			"id":           inbox.ID,
			"deviceId":     inbox.DeviceID,
			"waMessageId":  inbox.WaMessageID,
			"phone":        inbox.Phone,
			"name":         inbox.Name,
			"content":      inbox.Content,
			"direction":    "in",
			"type":         inbox.Type,
			"mediaUrl":     inbox.MediaURL,
			"replyTo":      inbox.ReplyTo,
			"replyContent": inbox.ReplyContent,
			"createdAt":    inbox.CreatedAt,
			"isRead":       false,
		},
	})
}

// undecryptableText = placeholder untuk pesan yang gagal didekripsi.
const undecryptableText = "⚠️ Pesan tidak dapat dibaca — minta pengirim mengirim ulang"

// handleUndecryptable mencatat pesan yang gagal didekripsi sebagai placeholder
// di Live Chat. Bila WhatsApp kemudian mengirim ulang pesan yang sama
// (retry), handleIncomingMessage menggantikan placeholder ini (dedupe
// berdasarkan wa_message_id).
func (m *Manager) handleUndecryptable(sess *SessionState, v *events.UndecryptableMessage) {
	if v.Info.IsFromMe || v.Info.IsGroup || v.DecryptFailMode == events.DecryptFailHide {
		return
	}
	sender := v.Info.Sender.User
	senderJID := v.Info.Sender.ToNonAD().String()
	phone := sender
	if v.Info.Sender.Server == types.HiddenUserServer && v.Info.SenderAlt.User != "" &&
		v.Info.SenderAlt.Server == types.DefaultUserServer {
		phone = v.Info.SenderAlt.User
	}
	if phone == "" {
		return
	}

	waID := string(v.Info.ID)
	log.Warn().Uint("deviceID", sess.DeviceID).Str("from", sender).Str("waMsgID", waID).
		Bool("unavailable", v.IsUnavailable).Str("tipeTakTersedia", string(v.UnavailableType)).
		Msg("Pesan masuk gagal didekripsi")

	if waID != "" {
		var n int64
		m.db.Model(&models.ChatInbox{}).
			Where("user_id = ? AND device_id = ? AND wa_message_id = ? AND direction = ?",
				sess.UserID, sess.DeviceID, waID, "in").Count(&n)
		if n > 0 {
			return // sudah tercatat (pesan asli atau placeholder sebelumnya)
		}
	}

	inbox := models.ChatInbox{
		UserID:      sess.UserID,
		DeviceID:    sess.DeviceID,
		Phone:       phone,
		SenderJID:   senderJID,
		Name:        v.Info.PushName,
		Content:     undecryptableText,
		Type:        "text",
		Direction:   "in",
		WaMessageID: waID,
	}
	m.db.Create(&inbox)
	m.updateConversation(sess, phone, senderJID, v.Info.PushName, undecryptableText, "text")
	m.broadcastIncomingChat(sess, &inbox)
}

// fetchIncomingMedia mengunduh media pesan masuk di latar belakang (tidak
// menahan penerimaan pesan), menyimpannya, mengisi media_url, lalu memberi
// tahu Live Chat lewat SSE "chat:media". Gagal → hanya log; bubble tetap
// menampilkan label tipe media.
func (m *Manager) fetchIncomingMedia(sess *SessionState, msg *events.Message, inboxID uint, phone string) {
	info, ok := incomingMediaOf(msg.Message)
	if !ok {
		return
	}
	waID := string(msg.Info.ID)
	if info.size > incomingMediaMaxBytes {
		log.Warn().Uint("deviceID", sess.DeviceID).Uint64("bytes", info.size).Msg("Media masuk melebihi batas, tidak diunduh")
		return
	}

	incomingMediaSlots <- struct{}{}
	defer func() { <-incomingMediaSlots }()

	cli, err := m.connectedClient(sess.DeviceID)
	if err != nil {
		log.Warn().Err(err).Uint("deviceID", sess.DeviceID).Msg("Unduh media masuk dibatalkan: device tidak terhubung")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), incomingMediaTimeout)
	defer cancel()
	data, err := cli.Download(ctx, info.dl)
	if err != nil {
		log.Warn().Err(err).Uint("deviceID", sess.DeviceID).Str("waMsgID", waID).Msg("Gagal mengunduh media masuk")
		return
	}
	if len(data) == 0 || len(data) > incomingMediaMaxBytes {
		log.Warn().Uint("deviceID", sess.DeviceID).Int("bytes", len(data)).Msg("Ukuran media masuk tidak valid, tidak disimpan")
		return
	}
	ext, ok := extensionForMedia(data, info.fileName)
	if !ok {
		log.Warn().Uint("deviceID", sess.DeviceID).Str("waMsgID", waID).Msg("Tipe media masuk tidak diizinkan, tidak disimpan")
		return
	}
	url, err := saveIncomingMedia(ChatUploadsDir, data, ext)
	if err != nil {
		log.Error().Err(err).Uint("deviceID", sess.DeviceID).Msg("Gagal menyimpan media masuk")
		return
	}
	if err := m.db.Model(&models.ChatInbox{}).Where("id = ?", inboxID).Update("media_url", url).Error; err != nil {
		log.Error().Err(err).Uint("deviceID", sess.DeviceID).Msg("Gagal menyimpan media_url pesan masuk")
		return
	}
	realtime.DefaultHub.SendToUser(sess.UserID, realtime.Event{
		Type: "chat:media",
		Payload: map[string]interface{}{
			"id":          inboxID,
			"deviceId":    sess.DeviceID,
			"phone":       phone,
			"waMessageId": waID,
			"mediaUrl":    url,
		},
	})
}

// ── Pencatatan pesan KELUAR ke Live Chat ─────────────────────────────────────

// outgoingChatText memilih teks tampilan Live Chat untuk pesan keluar.
// Media tanpa keterangan dibiarkan kosong (frontend menampilkan label tipenya).
func outgoingChatText(opts SendOptions) string {
	switch opts.Type {
	case "document":
		// Content = nama file bila FileName kosong; keterangan tidak dipakai.
		return ""
	case "poll":
		return "📊 " + opts.Content
	case "location":
		if opts.LocName != "" {
			return "📍 " + opts.LocName
		}
		return "📍 Lokasi"
	case "sticker", "audio", "voicenote", "voice":
		return ""
	}
	return opts.Content
}

// outgoingChatType memetakan tipe kirim ke tipe tampilan Live Chat.
func outgoingChatType(t string) string {
	switch t {
	case "", "text", "interactive", "buttons":
		return "text"
	case "voice":
		return "voicenote"
	case "poll", "location":
		return "text"
	}
	return t
}

// chatPhoneForJID menentukan kunci percakapan (chat_conversations.phone) untuk
// JID tujuan. JID @lid dipetakan ke nomor asli bila percakapannya sudah
// berkunci nomor asli; kalau tidak, memakai bagian user JID apa adanya.
func (m *Manager) chatPhoneForJID(sess *SessionState, jid types.JID) string {
	user := jid.User
	if jid.Server == types.HiddenUserServer {
		var conv models.ChatConversation
		if err := m.db.Where("user_id = ? AND device_id = ? AND sender_j_id = ?",
			sess.UserID, sess.DeviceID, jid.ToNonAD().String()).First(&conv).Error; err == nil && conv.Phone != "" {
			return conv.Phone
		}
		if pn := m.ResolvePNForLID(sess.DeviceID, user); pn != "" {
			return pn
		}
	}
	return user
}

// recordOutgoingChat mencatat pesan keluar ke chat_inboxes (direction "out"),
// memperbarui ringkasan percakapan, dan menyiarkan SSE "chat:message" agar
// Live Chat menampilkan balasan otomatis / Menu Bot / AI / API / bulk — bukan
// hanya pesan yang diketik di layar Live Chat. Best-effort: gagal hanya di-log.
func (m *Manager) recordOutgoingChat(sess *SessionState, jid types.JID, waMsgID string, opts SendOptions) {
	// Grup & broadcast tidak punya kunci percakapan yang sama dengan DM.
	if jid.Server != types.DefaultUserServer && jid.Server != types.HiddenUserServer {
		return
	}
	if m.db == nil || sess == nil {
		return
	}
	phone := m.chatPhoneForJID(sess, jid)
	if phone == "" {
		return
	}
	// Jangan membuat baris ganda bila WA ID yang sama sudah tercatat.
	if waMsgID != "" {
		var n int64
		m.db.Model(&models.ChatInbox{}).
			Where("user_id = ? AND device_id = ? AND wa_message_id = ? AND direction = ?",
				sess.UserID, sess.DeviceID, waMsgID, "out").Count(&n)
		if n > 0 {
			return
		}
	}

	display := outgoingChatText(opts)
	msgType := outgoingChatType(opts.Type)
	mediaURL := opts.MediaURL
	// file:// = path server (mis. media upload Live Chat) — bukan URL publik.
	if strings.HasPrefix(mediaURL, "file://") {
		mediaURL = ""
	}

	inbox := models.ChatInbox{
		UserID:      sess.UserID,
		DeviceID:    sess.DeviceID,
		Phone:       phone,
		SenderJID:   jid.ToNonAD().String(),
		Content:     display,
		Type:        msgType,
		Direction:   "out",
		MediaURL:    mediaURL,
		IsRead:      true,
		WaMessageID: waMsgID,
	}
	if opts.ReplyTo != "" {
		inbox.ReplyTo = opts.ReplyTo
	}
	if err := m.db.Create(&inbox).Error; err != nil {
		log.Warn().Err(err).Uint("deviceID", sess.DeviceID).Msg("Gagal mencatat pesan keluar ke Live Chat")
		return
	}

	summary := display
	if summary == "" {
		summary = "[" + msgType + "]"
	}
	var conv models.ChatConversation
	now := time.Now()
	if err := m.db.Where("user_id = ? AND device_id = ? AND phone = ?",
		sess.UserID, sess.DeviceID, phone).First(&conv).Error; err != nil {
		m.db.Create(&models.ChatConversation{
			UserID: sess.UserID, DeviceID: sess.DeviceID, Phone: phone,
			SenderJID:   jid.ToNonAD().String(),
			LastMessage: truncate(summary, 200), LastMessageType: msgType, LastActivity: now,
		})
	} else {
		m.db.Model(&conv).Updates(map[string]interface{}{
			"last_message": truncate(summary, 200), "last_message_type": msgType, "last_activity": now,
		})
	}
	m.RecordFirstResponse(sess.UserID, sess.DeviceID, phone)

	realtime.DefaultHub.SendToUser(sess.UserID, realtime.Event{
		Type: "chat:message",
		Payload: map[string]interface{}{
			"id":          inbox.ID,
			"deviceId":    sess.DeviceID,
			"waMessageId": waMsgID,
			"phone":       phone,
			"content":     display,
			"direction":   "out",
			"type":        msgType,
			"mediaUrl":    mediaURL,
			"createdAt":   inbox.CreatedAt,
			"isRead":      true,
		},
	})
}
