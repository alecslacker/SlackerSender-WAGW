package handler

// CRUD "Koneksi Pencarian" milik user + uji sebelum simpan. Pencarian dijalankan
// SERVER kita lewat penyedia pencarian terpisah (tidak terikat model/provider AI).
// Kunci terenkripsi (AES-GCM) dan tak pernah dikirim balik ke frontend.
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/middleware"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/rls"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/security"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerAISearchRoutes(rg *gin.RouterGroup, db *gorm.DB) {
	// Menyimpan kunci penyedia pencarian milik user — hanya key "full".
	ar := rg.Group("/ai-search", middleware.RequireScope("full"))
	{
		ar.GET("", getAISearchConnection(db))
		ar.PUT("", upsertAISearchConnection(db))
		ar.DELETE("", deleteAISearchConnection(db))
		ar.POST("/test", testAISearchConnection(db))
	}
}

// newSearcher: titik suntik KHUSUS TES (server tiruan di loopback ditolak klien
// anti-SSRF bawaan — itu perilaku produksi yang benar dan tak boleh dilonggarkan).
var newSearcher = service.NewSearcher

// Pertanyaan uji: jawaban faktual yang stabil, ada di semua mesin pencari.
const searchProbeQuery = "Presiden pertama Republik Indonesia"

type aiSearchReq struct {
	Provider string `json:"provider"`
	APIKey   string `json:"apiKey"`
	BaseURL  string `json:"baseUrl"`
	IsActive *bool  `json:"isActive"`
}

// searchConnectionDTO: bentuk aman untuk frontend (tanpa kunci).
func searchConnectionDTO(c *models.AISearchConnection) gin.H {
	if c == nil {
		return gin.H{"configured": false}
	}
	status := ""
	if c.SearchChecked() {
		status = c.SearchStatus
	}
	return gin.H{
		"configured":   true,
		"provider":     c.Provider,
		"baseUrl":      c.BaseURL,
		"keyHint":      keyHintView(c.KeyHint),
		"isActive":     c.IsActive,
		"searchStatus": status,
		"checkedAt":    c.CheckedAt,
	}
}

func keyHintView(h string) string {
	if h == "" {
		return ""
	}
	return "••••" + h
}

func getAISearchConnection(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		var conn models.AISearchConnection
		if err := udb.Where("user_id = ?", userID).First(&conn).Error; err != nil {
			c.JSON(http.StatusOK, searchConnectionDTO(nil))
			return
		}
		c.JSON(http.StatusOK, searchConnectionDTO(&conn))
	}
}

// normalizeSearchBase: hanya SearXNG memakai URL; wajib http(s) dan lolos SSRF.
func normalizeSearchBase(provider, raw string) (string, string) {
	if provider != service.SearchSearXNG {
		return "", "" // Z.AI/Brave: URL tetap, abaikan kiriman
	}
	base := strings.TrimSuffix(strings.TrimSpace(raw), "/")
	if base == "" {
		return "", "URL instance SearXNG wajib diisi"
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		return "", "URL harus diawali http:// atau https://"
	}
	if err := security.ValidateOutboundURL(base); err != nil {
		return "", "URL ditolak: " + err.Error()
	}
	return base, ""
}

func upsertAISearchConnection(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		var req aiSearchReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Data tidak valid"})
			return
		}
		provider := strings.ToLower(strings.TrimSpace(req.Provider))
		if !service.IsValidSearchProvider(provider) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Penyedia pencarian tidak dikenal. Pilih: zai, brave, searxng."})
			return
		}
		base, msg := normalizeSearchBase(provider, req.BaseURL)
		if msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": msg})
			return
		}

		var conn models.AISearchConnection
		// Unscoped: hidupkan baris soft-deleted (bukan INSERT baru yang menabrak unique user_id).
		res := udb.Unscoped().Where("user_id = ?", userID).First(&conn)
		isNew := res.Error != nil
		if !isNew && conn.DeletedAt.Valid {
			conn.DeletedAt = gorm.DeletedAt{}
		}

		key := strings.TrimSpace(req.APIKey)
		switch {
		case key != "":
			enc, err := security.Encrypt(key)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengenkripsi API key"})
				return
			}
			conn.APIKeyEnc = enc
			if len(key) > 4 {
				conn.KeyHint = key[len(key)-4:]
			} else {
				conn.KeyHint = "••••"
			}
		case provider != service.SearchSearXNG && (isNew || conn.APIKeyEnc == "" || conn.Provider != provider):
			// Z.AI/Brave butuh kunci; ganti penyedia tanpa kunci baru tak boleh
			// memakai kunci penyedia lain.
			c.JSON(http.StatusBadRequest, gin.H{"message": "API key wajib diisi untuk penyedia ini"})
			return
		}
		if provider == service.SearchSearXNG && key == "" && conn.Provider != provider {
			conn.APIKeyEnc, conn.KeyHint = "", "" // kunci penyedia lain tak ikut terbawa
		}

		// Yang diuji berubah (penyedia/URL/kunci) -> status "lulus" lama tak berlaku lagi;
		// toggle Cari di Web terkunci sampai diuji ulang.
		conn.ResetSearchIfChanged(provider, base, key != "")
		conn.UserID = userID
		conn.Provider = provider
		conn.BaseURL = base
		conn.IsActive = true
		if req.IsActive != nil {
			conn.IsActive = *req.IsActive
		}
		if err := udb.Save(&conn).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan koneksi pencarian"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"connection": searchConnectionDTO(&conn), "message": "Koneksi pencarian disimpan"})
	}
}

func deleteAISearchConnection(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		if err := udb.Where("user_id = ?", userID).Delete(&models.AISearchConnection{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menghapus"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Koneksi pencarian dihapus."})
	}
}

// testAISearchConnection mencari satu kueri tetap. Body diisi = uji nilai form
// (belum tersimpan); kosong = koneksi tersimpan. Hasil disimpan hanya bila uji
// memakai koneksi tersimpan dan pasti (bukan galat sementara).
func testAISearchConnection(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		var req aiSearchReq
		_ = c.ShouldBindJSON(&req)

		provider := strings.ToLower(strings.TrimSpace(req.Provider))
		apiKey := strings.TrimSpace(req.APIKey)
		baseURL := req.BaseURL
		var stored *models.AISearchConnection
		if provider == "" || (apiKey == "" && provider != service.SearchSearXNG) {
			var conn models.AISearchConnection
			if err := udb.Where("user_id = ?", userID).First(&conn).Error; err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "Belum ada koneksi pencarian yang dikonfigurasi"})
				return
			}
			key := ""
			if conn.APIKeyEnc != "" {
				k, err := security.Decrypt(conn.APIKeyEnc)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "API key tersimpan tidak bisa dibaca"})
					return
				}
				key = k
			}
			stored, provider, apiKey, baseURL = &conn, conn.Provider, key, conn.BaseURL
		}
		if !service.IsValidSearchProvider(provider) {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "Penyedia pencarian tidak dikenal"})
			return
		}
		base, msg := normalizeSearchBase(provider, baseURL)
		if msg != "" {
			c.JSON(http.StatusOK, gin.H{"ok": false, "status": service.ProbeUnknown, "message": msg})
			return
		}
		s, err := newSearcher(service.SearcherConfig{Provider: provider, APIKey: apiKey, BaseURL: base})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
		defer cancel()
		start := time.Now()
		results, serr := s.Search(ctx, searchProbeQuery)
		latency := time.Since(start).Milliseconds()

		status, detail := classifySearchProbe(results, serr)
		saved := false
		if stored != nil && status != service.ProbeUnknown && stored.Provider == provider {
			now := time.Now()
			if err := udb.Model(&models.AISearchConnection{}).Where("id = ?", stored.ID).Updates(map[string]interface{}{
				"search_status": status, "checked_provider": provider, "checked_at": now,
			}).Error; err == nil {
				saved = true
			}
		}
		titles := make([]string, 0, 3)
		for i, r := range results {
			if i == 3 {
				break
			}
			titles = append(titles, r.Domain+": "+r.Title)
		}
		c.JSON(http.StatusOK, gin.H{
			"ok": status == models.CapabilityYes, "status": status, "message": detail,
			"count": len(results), "samples": titles, "latencyMs": latency, "saved": saved,
		})
	}
}

// classifySearchProbe: lulus bila >=1 hasil bermutu. Kosong dari penyedia sehat
// berarti "tidak bekerja"; galat sementara/jaringan/kunci ditolak = belum bisa
// disimpulkan (jangan menandai gagal permanen).
func classifySearchProbe(results []service.SearchResult, err error) (string, string) {
	if err == nil && len(results) > 0 {
		return models.CapabilityYes, "Pencarian bekerja"
	}
	if err != nil && strings.Contains(err.Error(), service.ErrSearchEmpty.Error()) {
		return models.CapabilityNo, "Penyedia menjawab tetapi tanpa hasil — periksa kunci/kuota/penyedia"
	}
	if err != nil {
		return service.ProbeUnknown, friendlySearchError(err)
	}
	return service.ProbeUnknown, "Tidak ada hasil"
}

// friendlySearchError menerjemahkan galat teknis jadi pesan ramah (tanpa isi respons mentah).
func friendlySearchError(err error) string {
	l := strings.ToLower(err.Error())
	switch {
	case strings.Contains(l, "1113") || strings.Contains(l, "insufficient balance"):
		// Z.AI membalas 429 + kode 1113 bila kunci tak punya saldo/paket aktif — bukan rate limit.
		return "Kunci tidak punya paket Coding Plan aktif atau saldo (Z.AI 1113). Periksa paket di panel Z.AI."
	case strings.Contains(l, "search error 401") || strings.Contains(l, "search error 403"):
		return "Kunci/akses ditolak penyedia pencarian. Periksa API key (SearXNG: format JSON harus dinyalakan di instance)."
	case strings.Contains(l, "search error 429"):
		return "Kena batas penggunaan penyedia pencarian (429). Coba lagi nanti atau cek kuota."
	case strings.Contains(l, "search error 404"):
		return "Alamat/jalur penyedia tidak ditemukan (404)."
	case strings.Contains(l, "search error 400") || strings.Contains(l, "search error 422"):
		return "Permintaan ditolak penyedia (" + shortSearchErr(l) + ")."
	case strings.Contains(l, "context deadline") || strings.Contains(l, "timeout") || strings.Contains(l, "pencarian gagal dihubungi"):
		return "Tidak bisa menghubungi penyedia pencarian. Periksa koneksi/alamat."
	case strings.Contains(l, "tidak dapat dibaca"):
		return err.Error()
	}
	return shortSearchErr(err.Error())
}

func shortSearchErr(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 140 {
		return string(r[:140]) + "…"
	}
	return s
}
