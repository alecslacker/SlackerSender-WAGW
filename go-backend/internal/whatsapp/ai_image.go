// Gambar masuk untuk AI Reply (vision): keputusan apakah pesan ditangani,
// validasi isi gambar, dan unduhan in-memory untuk dikirim ke LLM.
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
package whatsapp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/service"
	"go.mau.fi/whatsmeow/types/events"
)

const (
	// aiImageMaxBytes: batas ukuran gambar yang dikirim ke LLM. 4 MB — di bawah
	// batas gambar Anthropic dan jauh di bawah batas inline Gemini (20 MB/request).
	aiImageMaxBytes = 4 << 20

	// imageOnlyPlaceholder menggantikan teks kosong (gambar tanpa caption) agar
	// pesan user tidak kosong setelah dibungkus pelindung injection.
	imageOnlyPlaceholder = "(Pengirim mengirim sebuah gambar tanpa keterangan. Jelaskan atau tanggapi gambar tersebut sesuai peranmu.)"

	// ImageUnreadableMessage: balasan sopan saat gambar tak bisa dibaca dan
	// tidak ada caption sebagai pegangan (unduh gagal / model tanpa vision).
	ImageUnreadableMessage = "Maaf, saya belum bisa membaca gambar tersebut. Boleh dijelaskan lewat teks?"
)

// isImageMessage melaporkan apakah pesan membawa gambar (bukan stiker/video).
func isImageMessage(msg *events.Message) bool {
	return msg != nil && msg.Message != nil && msg.Message.GetImageMessage() != nil
}

// shouldHandleForAI: apakah satu config boleh memproses pesan ini.
// Pesan bercaption/teks selalu lolos; gambar TANPA teks hanya lolos bila
// config membaca gambar (perilaku lama: teks kosong = diabaikan).
func shouldHandleForAI(text string, hasImage, readImages bool) bool {
	return strings.TrimSpace(text) != "" || (hasImage && readImages)
}

// aiAuditQuestion: teks pertanyaan untuk log audit chatbot. Log membuang
// pertanyaan kosong, jadi gambar tanpa caption dicatat sebagai "[gambar]".
func aiAuditQuestion(text string, withImage bool) string {
	text = strings.TrimSpace(text)
	if !withImage {
		return text
	}
	if text == "" {
		return "[gambar]"
	}
	return "[gambar] " + text
}

// validateAIImage memeriksa gambar dari ISI file (bukan klaim pengirim):
// hanya jpeg/png/webp (Gemini tak menerima gif) dan maksimal aiImageMaxBytes.
func validateAIImage(data []byte) (service.ChatImage, error) {
	if len(data) == 0 {
		return service.ChatImage{}, fmt.Errorf("gambar kosong")
	}
	if len(data) > aiImageMaxBytes {
		return service.ChatImage{}, fmt.Errorf("gambar %d byte melebihi batas %d", len(data), aiImageMaxBytes)
	}
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	mime := http.DetectContentType(head)
	if i := strings.Index(mime, ";"); i >= 0 {
		mime = strings.TrimSpace(mime[:i])
	}
	switch mime {
	case "image/jpeg", "image/png", "image/webp":
		return service.ChatImage{MIME: mime, Data: data}, nil
	}
	return service.ChatImage{}, fmt.Errorf("tipe gambar %q tidak didukung", mime)
}

// loadAIImage mengunduh gambar pesan masuk ke memori (tidak disimpan ke disk)
// lalu memvalidasinya. Memakai slot unduhan yang sama dengan media Live Chat
// agar banjir gambar tak menghabiskan memori/koneksi.
func (m *Manager) loadAIImage(sess *SessionState, msg *events.Message) (service.ChatImage, error) {
	if !isImageMessage(msg) {
		return service.ChatImage{}, fmt.Errorf("pesan bukan gambar")
	}
	im := msg.Message.GetImageMessage()
	if im.GetFileLength() > aiImageMaxBytes {
		return service.ChatImage{}, fmt.Errorf("gambar %d byte melebihi batas %d", im.GetFileLength(), aiImageMaxBytes)
	}

	ctx, cancel := context.WithTimeout(context.Background(), incomingMediaTimeout)
	defer cancel()
	select {
	case incomingMediaSlots <- struct{}{}:
		defer func() { <-incomingMediaSlots }()
	case <-ctx.Done():
		return service.ChatImage{}, fmt.Errorf("antrean unduhan penuh: %w", ctx.Err())
	}

	cli, err := m.connectedClient(sess.DeviceID)
	if err != nil {
		return service.ChatImage{}, fmt.Errorf("device tidak terhubung: %w", err)
	}
	data, err := cli.Download(ctx, im)
	if err != nil {
		return service.ChatImage{}, fmt.Errorf("unduh gambar gagal: %w", err)
	}
	return validateAIImage(data)
}
