package handler

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/middleware"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/quota"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/whatsapp"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ── Kompatibilitas WAMP (ppob.clipku.com) + Slackersender ────────────────────
// PPOB (WampService) POST ke {wamp_url}/send-message dan /send-media dengan
// field: api_key, sender, number, message (+ url, media_type, caption untuk
// media). Endpoint ini menerjemahkannya ke pengiriman via device milik
// pemilik API key. Tidak perlu ubah kode PPOB — cukup isi konfigurasi:
//   wamp_url        = <APP_URL domain ini, mis. https://wasender.slackercode.us>
//   wamp_api_key    = API key dari Setting > API Key
//   wamp_sender     = ID / nomor / nama device pengirim
//   wamp_status_api = 1
//
// Kontrak endpoint juga disamakan dengan API Slackersender (dokumen
// api-slackersender-global.md): /send-message, /send-media, /send-location,
// /info-device, /check-number — masing-masing POST (JSON/form) atau GET
// (query string). Auth via field api_key, respons sukses = HTTP 200 +
// "status": true. Hanya kontrak pengiriman yang disamakan; fitur lain
// Slackersender tidak diadopsi.

func registerWampCompatRoutes(r *gin.Engine, db *gorm.DB, wm *whatsapp.Manager) {
	// POST direkomendasikan (api_key tidak nongol di access log);
	// GET didaftarkan demi kesamaan kontrak Slackersender.
	// Alias "/api/..." menampung klien yang base URL-nya terlanjur diisi
	// "<domain>/api" (kesalahan umum — terlihat di log produksi sebagai 404).
	// Handler sama; auth tetap lewat api_key.
	for _, prefix := range []string{"", "/api"} {
		r.POST(prefix+"/send-message", wampSend(db, wm, false))
		r.GET(prefix+"/send-message", wampSend(db, wm, false))
		r.POST(prefix+"/send-media", wampSend(db, wm, true))
		r.GET(prefix+"/send-media", wampSend(db, wm, true))
		r.POST(prefix+"/send-location", wampSendLocation(db, wm))
		r.GET(prefix+"/send-location", wampSendLocation(db, wm))
		r.POST(prefix+"/info-device", wampInfoDevice(db))
		r.GET(prefix+"/info-device", wampInfoDevice(db))
		r.POST(prefix+"/check-number", wampCheckNumber(db, wm))
		r.GET(prefix+"/check-number", wampCheckNumber(db, wm))
	}
}

func wampSend(db *gorm.DB, wm *whatsapp.Manager, isMedia bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			APIKey    string `form:"api_key" json:"api_key"`
			Sender    string `form:"sender" json:"sender"`
			Number    string `form:"number" json:"number"`
			Message   string `form:"message" json:"message"`
			URL       string `form:"url" json:"url"`
			MediaType string `form:"media_type" json:"media_type"`
			Caption   string `form:"caption" json:"caption"`
			ReplyTo   string `form:"reply_to" json:"reply_to"`
		}
		// Dukung form-encoded (default WampService) maupun JSON.
		_ = c.ShouldBind(&req)
		if req.APIKey == "" {
			req.APIKey = c.GetHeader("X-API-Key")
		}

		numbers := splitWampNumbers(req.Number)
		if req.APIKey == "" || len(numbers) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": "api_key dan number wajib diisi"})
			return
		}

		content := req.Message
		mediaURL := ""
		msgType := "text"
		if isMedia {
			if req.URL == "" {
				c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": "url media wajib diisi"})
				return
			}
			mediaURL = req.URL
			// media_type Slackersender: image/video/audio/document.
			// Kosong / tak dikenal → image (perilaku lama; PPOB yang tidak
			// mengirim media_type tetap terkirim sebagai gambar).
			msgType = mapWampMediaType(req.MediaType)
			content = req.Caption
			if content == "" {
				content = req.Message
			}
		}
		if content == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": "message/caption wajib diisi"})
			return
		}
		if utf8.RuneCountInString(content) > 10000 {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": "Pesan maksimal 10000 karakter"})
			return
		}

		// Validasi API key -> user (helper bersama semua endpoint compat).
		ak, ok := authenticateAPIKey(c, db, req.APIKey, "messages:send")
		if !ok {
			return // respons error sudah dikirim helper
		}

		// Resolve sender -> device milik user.
		device, err := resolveWampSender(db, ak.UserID, req.Sender)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"status": false, "message": err.Error()})
			return
		}

		wampDispatch(c, db, wm, ak.UserID, device, numbers, wampPayload{
			Type:     msgType,
			Content:  content,
			MediaURL: mediaURL,
			Caption:  req.Caption,
			ReplyTo:  req.ReplyTo,
		})
	}
}

// ── Banyak nomor dalam satu request ──────────────────────────────────────────
// Kontrak Slackersender: field "number" boleh berisi beberapa nomor yang
// dipisah "|" (mis. "6281111|6282222"); pesan yang sama dikirim ke semuanya.
// Pemisah lain (",", ";", baris baru) ikut ditoleransi.

// wampMultiSendGap = jeda acak antar nomor pada request multi-nomor agar pola
// kirim tidak seperti spam (sama semangatnya dengan jeda blast bulk).
const (
	wampMultiMinGapSec = 2
	wampMultiMaxGapSec = 5
)

// splitWampNumbers memecah field number menjadi daftar nomor unik (urutan
// asli dipertahankan); spasi dan entri kosong dibuang.
func splitWampNumbers(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return strings.ContainsRune("|,;\n\r", r)
	})
	seen := make(map[string]bool, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// wampPayload = isi pesan yang sama untuk semua nomor tujuan.
type wampPayload struct {
	Type      string
	Content   string
	MediaURL  string
	Caption   string
	ReplyTo   string
	Latitude  float64
	Longitude float64
}

// wampDispatch menyimpan satu catatan pesan per nomor, mengecek kuota untuk
// TOTAL nomor di muka, lalu mengirim berurutan di background (jeda acak antar
// nomor bila lebih dari satu). Menulis respons ke c.
func wampDispatch(c *gin.Context, db *gorm.DB, wm *whatsapp.Manager, userID uint, device *models.Device, numbers []string, p wampPayload) {
	// Batas penerima per request mengikuti batas bulk paket aktif.
	limit := 1000
	if plan, _, err := quota.EffectivePlan(db, userID); err == nil && plan != nil && plan.MaxBulk > 0 {
		limit = plan.MaxBulk
	}
	if len(numbers) > limit {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  false,
			"message": fmt.Sprintf("Maksimal %d nomor per request (paket aktif)", limit),
		})
		return
	}

	// Kuota pesan bulanan (Fitur 3): tolak 429 bila habis.
	if qr, qerr := quota.CheckN(db, userID, int64(len(numbers))); qerr == nil && !qr.Allowed {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"status":  false,
			"message": quota.ExceededMessage(qr),
			"code":    "QUOTA_EXCEEDED",
		})
		return
	}

	msgs := make([]models.Message, 0, len(numbers))
	for _, num := range numbers {
		msg := models.Message{
			UserID:   userID,
			DeviceID: device.ID,
			To:       num,
			Type:     p.Type,
			Content:  p.Content,
			MediaURL: p.MediaURL,
			Caption:  p.Caption,
			Status:   "pending",
			Via:      "api",
		}
		if err := db.Create(&msg).Error; err != nil {
			// Batalkan yang sudah terlanjur dibuat agar tidak menggantung "pending".
			for i := range msgs {
				db.Model(&msgs[i]).Updates(map[string]interface{}{"status": "failed", "error_msg": "dibatalkan: gagal menyimpan pesan lain dalam request"})
			}
			c.JSON(http.StatusInternalServerError, gin.H{"status": false, "message": "Gagal menyimpan pesan"})
			return
		}
		msgs = append(msgs, msg)
	}

	campaignID := "wamp-" + time.Now().Format("20060102150405")
	go func() {
		for i := range msgs {
			if i > 0 {
				time.Sleep(time.Duration(wampMultiMinGapSec+rand.IntN(wampMultiMaxGapSec-wampMultiMinGapSec+1)) * time.Second)
			}
			msg := &msgs[i]
			waID, err := wm.SendMessageWithOptions(device.ID, msg.To, whatsapp.SendOptions{
				Type:      p.Type,
				Content:   p.Content,
				MediaURL:  p.MediaURL,
				ReplyTo:   p.ReplyTo,
				Latitude:  p.Latitude,
				Longitude: p.Longitude,
			})
			if err != nil {
				db.Model(msg).Updates(map[string]interface{}{"status": "failed", "error_msg": err.Error()})
				recordReport(db, userID, device.ID, campaignID, msg.To, "", "failed", err.Error())
			} else {
				now := time.Now()
				db.Model(msg).Updates(map[string]interface{}{"status": "sent", "message_id": waID, "sent_at": &now})
				recordReport(db, userID, device.ID, campaignID, msg.To, waID, "sent", "")
			}
		}
	}()

	resp := gin.H{"status": true, "message": "Pesan sedang dikirim", "id": msgs[0].ID}
	if len(msgs) > 1 {
		ids := make([]uint, len(msgs))
		for i := range msgs {
			ids[i] = msgs[i].ID
		}
		resp["ids"] = ids
		resp["total"] = len(msgs)
	}
	c.JSON(http.StatusOK, resp)
}

// resolveWampSender mencari device milik user berdasarkan sender yang dikirim
// PPOB: bisa berupa ID device (angka), nomor HP, atau nama device. Bila kosong,
// pakai device default / yang pertama.
func resolveWampSender(db *gorm.DB, userID uint, sender string) (*models.Device, error) {
	var device models.Device
	s := strings.TrimSpace(sender)

	if s == "" {
		if err := db.Where("user_id = ? AND is_default = ?", userID, true).First(&device).Error; err != nil {
			if err := db.Where("user_id = ?", userID).Order("id").First(&device).Error; err != nil {
				return nil, fmt.Errorf("perangkat pengirim tidak ditemukan")
			}
		}
		return &device, nil
	}

	if id, err := strconv.ParseUint(s, 10, 64); err == nil {
		if err := db.Where("id = ? AND user_id = ?", uint(id), userID).First(&device).Error; err == nil {
			return &device, nil
		}
	}

	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	if digits != "" {
		digits = normalizeWampPhone(digits)
		q := db.Where("user_id = ?", userID).Where("phone = ? OR phone = ?", digits, strings.TrimPrefix(digits, "62")).First(&device)
		if q.Error == nil {
			return &device, nil
		}
		// Coba juga dengan prefix 62 bila yang dikirim tanpa kode negara.
		if !strings.HasPrefix(digits, "62") {
			if err := db.Where("user_id = ? AND phone = ?", userID, "62"+digits).First(&device).Error; err == nil {
				return &device, nil
			}
		}
	}

	if err := db.Where("user_id = ? AND name = ?", userID, s).First(&device).Error; err == nil {
		return &device, nil
	}

	return nil, fmt.Errorf("perangkat pengirim tidak ditemukan")
}

// normalizeWampPhone mengubah format lokal "08xx" menjadi "628xx" (hanya
// digit masuk). Tanpa ini sender/number berawalan 0 tidak cocok dengan
// devices.phone yang tersimpan berformat 62xx.
func normalizeWampPhone(digits string) string {
	if strings.HasPrefix(digits, "0") && len(digits) > 1 {
		return "62" + digits[1:]
	}
	return digits
}

// authenticateAPIKey memvalidasi api_key (hash) milik endpoint compat,
// menolak key nonaktif/kedaluwarsa, MENEGAKKAN scope (sama seperti
// middleware.RequireScope di endpoint native /api/*), lalu memperbarui
// last_used. Tanpa cek scope, key terbatas (mis. hanya "messages:read")
// bisa mengirim pesan lewat endpoint compat.
// Respons error sudah dikirim ke c; ok=false berarti request ditolak.
func authenticateAPIKey(c *gin.Context, db *gorm.DB, apiKey, scope string) (*models.ApiKey, bool) {
	var ak models.ApiKey
	if err := db.Where("key_hash = ? AND is_active = ?", models.HashAPIKey(apiKey), true).First(&ak).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "message": "API key tidak valid"})
		return nil, false
	}
	if ak.ExpiresAt != nil && time.Now().After(*ak.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "message": "API key kedaluwarsa"})
		return nil, false
	}
	if !apiKeyHasScope(ak.Scopes, scope) {
		c.JSON(http.StatusForbidden, gin.H{
			"status":  false,
			"message": "API key tidak memiliki akses: " + scope,
			"code":    "INSUFFICIENT_SCOPE",
		})
		return nil, false
	}
	// Akun pemilik diblokir/ditangguhkan → API key-nya ikut mati
	// (sama seperti jalur X-API-Key di middleware.AuthRequired).
	var owner models.User
	if err := db.Select("id", "status").Where("id = ?", ak.UserID).First(&owner).Error; err != nil || middleware.IsBlockedStatus(owner.Status) {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "message": "Akun dinonaktifkan", "code": "ACCOUNT_DISABLED"})
		return nil, false
	}
	db.Model(&ak).Update("last_used", time.Now())
	return &ak, true
}

// apiKeyHasScope: "full" (atau scopes kosong = key lama) membuka semua akses.
func apiKeyHasScope(rawScopes, scope string) bool {
	for _, s := range middleware.ParseAPIKeyScopes(rawScopes) {
		if s == "full" || s == scope {
			return true
		}
	}
	return false
}

// mapWampMediaType menerjemahkan media_type Slackersender ke tipe kirim
// internal. Kosong / tak dikenal → "image" (perilaku lama PPOB WAMP).
func mapWampMediaType(mediaType string) string {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "video":
		return "video"
	case "audio":
		return "audio"
	case "document":
		return "document"
	default:
		return "image"
	}
}

// wampSendLocation — kontrak Slackersender /send-location.
// latitude/longitude dikirim sebagai STRING oleh Slackersender (jaga
// presisi desimal); di-parse ke float64 sebelum masuk ke SendOptions.
func wampSendLocation(db *gorm.DB, wm *whatsapp.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			APIKey    string `form:"api_key" json:"api_key"`
			Sender    string `form:"sender" json:"sender"`
			Number    string `form:"number" json:"number"`
			Latitude  string `form:"latitude" json:"latitude"`
			Longitude string `form:"longitude" json:"longitude"`
		}
		_ = c.ShouldBind(&req)
		if req.APIKey == "" {
			req.APIKey = c.GetHeader("X-API-Key")
		}

		numbers := splitWampNumbers(req.Number)
		if req.APIKey == "" || len(numbers) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": "api_key dan number wajib diisi"})
			return
		}
		if req.Latitude == "" || req.Longitude == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": "latitude dan longitude wajib diisi"})
			return
		}
		lat, err := strconv.ParseFloat(strings.TrimSpace(req.Latitude), 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": "latitude tidak valid"})
			return
		}
		lon, err := strconv.ParseFloat(strings.TrimSpace(req.Longitude), 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": "longitude tidak valid"})
			return
		}

		ak, ok := authenticateAPIKey(c, db, req.APIKey, "messages:send")
		if !ok {
			return
		}

		device, err := resolveWampSender(db, ak.UserID, req.Sender)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"status": false, "message": err.Error()})
			return
		}

		// Lokasi juga pesan keluar → ikut aturan kuota (dicek di wampDispatch).
		wampDispatch(c, db, wm, ak.UserID, device, numbers, wampPayload{
			Type:      "location",
			Content:   fmt.Sprintf("%f,%f", lat, lon),
			Latitude:  lat,
			Longitude: lon,
		})
	}
}

// wampInfoDevice — kontrak Slackersender /info-device: status kesehatan
// perangkat gateway. Field dipetakan dari Device; nilai Slackersender yang
// tidak punya padanan langsung dikirim konstanta (reply_when="Personal").
// Status koneksi dibaca dari kolom devices.status yang disinkronkan
// Manager saat connect/disconnect.
func wampInfoDevice(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			APIKey string `form:"api_key" json:"api_key"`
			Number string `form:"number" json:"number"`
		}
		_ = c.ShouldBind(&req)
		if req.APIKey == "" {
			req.APIKey = c.GetHeader("X-API-Key")
		}

		if req.APIKey == "" || req.Number == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": "api_key dan number wajib diisi"})
			return
		}

		ak, ok := authenticateAPIKey(c, db, req.APIKey, "devices:read")
		if !ok {
			return
		}

		device, err := resolveWampSender(db, ak.UserID, req.Number)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"status": false, "message": err.Error()})
			return
		}

		// message_sent: pesan keluar yang benar-benar terkirim (definisi
		// sama dengan kuota — failed/pending tidak dihitung).
		var messageSent int64
		db.Model(&models.Message{}).
			Where("device_id = ? AND direction = ? AND status IN ?", device.ID, "outgoing", []string{"sent", "delivered", "read"}).
			Count(&messageSent)

		connStatus := "Disconnect"
		if device.Status == "connected" {
			connStatus = "Connect"
		}

		var webhook interface{}
		if device.WebhookURL != "" {
			webhook = device.WebhookURL
		}

		c.JSON(http.StatusOK, gin.H{
			"status": true,
			"info": []gin.H{{
				"id":           device.ID,
				"user_id":      device.UserID,
				"body":         device.Phone,
				"webhook":      webhook,
				"status":       connStatus,
				"created_at":   device.CreatedAt,
				"updated_at":   device.UpdatedAt,
				"message_sent": messageSent,
				"reply_when":   "Personal",
				"wh_read":      boolToInt(device.ReadReceipts),
				"reject_call":  boolToInt(device.RejectCall),
				"wh_typing":    boolToInt(device.TypingIndicator),
			}},
		})
	}
}

// boolToInt meniru representasi flag 0/1 ala Slackersender.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// wampCheckNumber — kontrak Slackersender /check-number: cek apakah
// nomor terdaftar di WhatsApp, kembalikan exists + JID.
func wampCheckNumber(db *gorm.DB, wm *whatsapp.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			APIKey string `form:"api_key" json:"api_key"`
			Sender string `form:"sender" json:"sender"`
			Number string `form:"number" json:"number"`
		}
		_ = c.ShouldBind(&req)
		if req.APIKey == "" {
			req.APIKey = c.GetHeader("X-API-Key")
		}

		if req.APIKey == "" || req.Number == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": "api_key dan number wajib diisi"})
			return
		}

		ak, ok := authenticateAPIKey(c, db, req.APIKey, "contacts")
		if !ok {
			return
		}

		device, err := resolveWampSender(db, ak.UserID, req.Sender)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"status": false, "message": err.Error()})
			return
		}

		exists, jid, err := wm.CheckNumberJID(device.ID, req.Number)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": false, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": true,
			"msg": gin.H{
				"exists": exists,
				"jid":    jid,
			},
		})
	}
}
