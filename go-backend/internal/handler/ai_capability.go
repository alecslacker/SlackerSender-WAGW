package handler

// Uji kemampuan SEBELUM simpan: "Cek Gambar" (model bisa melihat gambar?) dan
// "Cek Pencarian" (penyedia pencarian web bekerja?), plus CRUD koneksi pencarian.
// Pola sama dengan POST /ai-connection/test: body diisi = uji nilai form yang
// BELUM tersimpan; body kosong = pakai yang tersimpan. Hasil uji hanya
// disimpan bila uji memakai koneksi yang sudah tersimpan DAN sama persis dengan
// yang diuji (model/penyedia sama), supaya status tak menempel pada form lain.
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia

import (
	"crypto/rand"
	"math/big"
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

// newVisionService: titik suntik KHUSUS TES (alasan sama dengan newSearcher).
var newVisionService = service.NewAIServiceForConnection

// pickIndex: titik suntik KHUSUS TES (warna gambar uji dibuat deterministik agar
// jawaban model tiruan bisa tepat). Produksi = randIndex (crypto/rand).
var pickIndex = randIndex

// randIndex: indeks acak [0,n) dari crypto/rand — gambar uji tak boleh bisa
// ditebak pihak luar (warna dipilih per cek).
func randIndex(n int) int {
	if n <= 1 {
		return 0
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return int(time.Now().UnixNano() % int64(n)) // cadangan; tak kritis untuk keamanan
	}
	return int(v.Int64())
}

// resolvedAI: kredensial koneksi AI yang siap dipakai untuk uji.
type resolvedAI struct {
	Provider service.AIProvider
	Model    string
	BaseURL  string
	APIKey   string
	// FromStored: kredensial berasal dari koneksi tersimpan (bukan nilai form).
	FromStored bool
	Conn       *models.AIConnection
}

// resolveAIForTest menyelesaikan kredensial: nilai form bila provider+key diisi,
// selain itu koneksi tersimpan. Mengembalikan (hasil, kode HTTP galat, pesan galat).
func resolveAIForTest(udb *gorm.DB, userID uint, req aiConnectionReq) (*resolvedAI, int, string) {
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	model := strings.TrimSpace(req.Model)
	baseURL := strings.TrimSuffix(strings.TrimSpace(req.BaseURL), "/")
	apiKey := strings.TrimSpace(req.APIKey)
	out := &resolvedAI{}

	if provider == "" || apiKey == "" {
		conn := service.ConnectionForUser(udb, userID)
		if conn == nil {
			return nil, http.StatusBadRequest, "Belum ada koneksi AI yang dikonfigurasi"
		}
		key, err := security.Decrypt(conn.APIKeyEnc)
		if err != nil || key == "" {
			return nil, http.StatusInternalServerError, "API key tersimpan tidak bisa dibaca"
		}
		provider, apiKey, baseURL = conn.Provider, key, conn.BaseURL
		if model == "" {
			model = conn.Model
		}
		out.FromStored, out.Conn = true, conn
	}
	if !service.IsValidProvider(provider) {
		return nil, http.StatusBadRequest, "Provider tidak dikenal"
	}
	if model == "" {
		model = service.DefaultModelFor(service.AIProvider(provider))
	}
	if baseURL != "" {
		if err := security.ValidateOutboundURL(baseURL); err != nil {
			return nil, http.StatusBadRequest, "Base URL ditolak: " + err.Error()
		}
	}
	out.Provider, out.Model, out.BaseURL, out.APIKey = service.AIProvider(provider), model, baseURL, apiKey
	return out, 0, ""
}

// visionCheck menguji apakah koneksi bisa membaca gambar.
func visionCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		var req aiConnectionReq
		_ = c.ShouldBindJSON(&req)

		r, code, msg := resolveAIForTest(udb, userID, req)
		if r == nil {
			c.JSON(code, gin.H{"ok": false, "message": msg})
			return
		}
		svc := newVisionService(r.Provider, r.APIKey, r.BaseURL)
		start := time.Now()
		res := svc.ProbeVision(r.Provider, r.Model, pickIndex)
		latency := time.Since(start).Milliseconds()

		saved := false
		// Simpan hanya bila yang diuji = koneksi tersimpan DAN model-nya masih sama
		// dengan yang tersimpan (uji nilai form tak boleh menempel ke koneksi lain),
		// dan hasilnya pasti (unknown tidak disimpan).
		if r.FromStored && res.Status != service.ProbeUnknown && r.Conn != nil && r.Model == r.Conn.Model {
			now := time.Now()
			if err := udb.Model(&models.AIConnection{}).Where("id = ?", r.Conn.ID).Updates(map[string]interface{}{
				"vision_status": res.Status, "vision_model": r.Model, "vision_checked_at": now,
			}).Error; err == nil {
				saved = true
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"ok": res.Status == "yes", "status": res.Status, "message": res.Detail,
			"model": r.Model, "latencyMs": latency, "saved": saved,
		})
	}
}
