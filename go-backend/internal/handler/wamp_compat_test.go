// Test endpoint kompatibilitas WAMP + Slackersender (wamp_compat.go).
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/gin-gonic/gin"
)

// ── Pure function: mapWampMediaType ──────────────────────────────────────────

func TestMapWampMediaType(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"image", "image", "image"},
		{"video", "video", "video"},
		{"audio", "audio", "audio"},
		{"document", "document", "document"},
		{"huruf besar ditoleransi", "VIDEO", "video"},
		{"spasi di-trim", " document ", "document"},
		{"kosong → image (perilaku lama PPOB)", "", "image"},
		{"tak dikenal → image", "sticker", "image"},
	}
	for _, tc := range cases {
		if got := mapWampMediaType(tc.in); got != tc.want {
			t.Errorf("%s: mapWampMediaType(%q) = %q, mau %q", tc.name, tc.in, got, tc.want)
		}
	}
}

// ── Regression: scope API key WAJIB ditegakkan di endpoint compat ───────────
// Bug produksi-kritis: key ber-scope "messages:read" sempat bisa mengirim pesan
// lewat /send-message, padahal endpoint native /api/messages/send menolaknya.

func TestAPIKeyHasScope(t *testing.T) {
	cases := []struct {
		name   string
		scopes string
		scope  string
		want   bool
	}{
		{"key lama (scopes kosong) = full", "", "messages:send", true},
		{"full membuka semua", `["full"]`, "contacts", true},
		{"scope cocok", `["messages:send"]`, "messages:send", true},
		{"read-only DITOLAK kirim", `["messages:read"]`, "messages:send", false},
		{"devices:read tidak boleh cek nomor", `["devices:read"]`, "contacts", false},
		{"multi scope", `["messages:read","devices:read"]`, "devices:read", true},
		{"JSON rusak = full (perilaku ParseAPIKeyScopes)", `bukan-json`, "messages:send", true},
	}
	for _, tc := range cases {
		if got := apiKeyHasScope(tc.scopes, tc.scope); got != tc.want {
			t.Errorf("%s: apiKeyHasScope(%q, %q) = %v, mau %v", tc.name, tc.scopes, tc.scope, got, tc.want)
		}
	}
}

// ── Handler tanpa DB: validasi input ditolak sebelum auth ───────────────────

func newWampTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// db nil aman: semua kasus di bawah ditolak sebelum menyentuh DB.
	r.POST("/send-location", wampSendLocation(nil, nil))
	r.POST("/check-number", wampCheckNumber(nil, nil))
	return r
}

func postJSON(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestWampSendLocationValidation(t *testing.T) {
	r := newWampTestRouter()

	cases := []struct {
		name string
		body string
		want int
	}{
		{"api_key kosong", `{"number":"62888","latitude":"1","longitude":"1"}`, http.StatusBadRequest},
		{"number kosong", `{"api_key":"kunci","latitude":"1","longitude":"1"}`, http.StatusBadRequest},
		{"koordinat kosong", `{"api_key":"kunci","number":"62888"}`, http.StatusBadRequest},
		{"latitude bukan angka", `{"api_key":"kunci","number":"62888","latitude":"bujur","longitude":"106.8"}`, http.StatusBadRequest},
		{"longitude bukan angka", `{"api_key":"kunci","number":"62888","latitude":"-6.2","longitude":"timur"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		w := postJSON(r, "/send-location", tc.body)
		if w.Code != tc.want {
			t.Errorf("%s: status %d (%s), mau %d", tc.name, w.Code, w.Body.String(), tc.want)
		}
		if !strings.Contains(w.Body.String(), `"status":false`) {
			t.Errorf("%s: respons harus mengandung status:false, dapat %s", tc.name, w.Body.String())
		}
	}
}

func TestWampCheckNumberValidation(t *testing.T) {
	r := newWampTestRouter()

	w := postJSON(r, "/check-number", `{"sender":"62811"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("number kosong: status %d (%s), mau 400", w.Code, w.Body.String())
	}
}

// ── Handler dengan DB (skip bila DATABASE_URL tidak tersedia) ────────────────

func TestWampAuthRejectsBadKey(t *testing.T) {
	db := auditTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.ApiKey{}, &models.Device{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/send-message", wampSend(db, nil, false))

	w := postJSON(r, "/send-message", `{"api_key":"kunci-ngawur","number":"62888","message":"hai"}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("api_key salah: status %d (%s), mau 401", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"status":false`) {
		t.Errorf("respons harus status:false, dapat %s", w.Body.String())
	}
}

func TestWampInfoDeviceShape(t *testing.T) {
	db := auditTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.ApiKey{}, &models.Device{}, &models.Message{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	// Bersihkan sisa run sebelumnya secara PERMANEN (Unscoped): email user
	// dan key_hash ber-uniqueIndex, soft delete akan menahan index-nya dan
	// membuat run berikutnya gagal seed duplicate.
	cleanupWampTestData := func() {
		db.Unscoped().Where("name LIKE ?", "wamp-test-%").Delete(&models.ApiKey{})
		db.Unscoped().Where("name LIKE ?", "wamp-test-%").Delete(&models.Device{})
		db.Unscoped().Where("email = ?", "wamp-test-user@example.test").Delete(&models.User{})
	}
	cleanupWampTestData()
	defer cleanupWampTestData()

	// Seed: user + key + device terhubung.
	// UUID dibiarkan kosong: hook BeforeCreate model mengisi UUID valid
	// (kolom bertipe uuid di Postgres menolak string sembarang).
	u := models.User{Name: "wamp-test-user", Email: "wamp-test-user@example.test"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rawKey := "wamp-test-key-1"
	ak := models.ApiKey{UserID: u.ID, Name: "wamp-test-key", KeyHash: models.HashAPIKey(rawKey), IsActive: true}
	if err := db.Create(&ak).Error; err != nil {
		t.Fatalf("seed api key: %v", err)
	}

	dev := models.Device{UserID: u.ID, Name: "wamp-test-dev", Phone: "628111111111", Status: "connected", ReadReceipts: true}
	if err := db.Create(&dev).Error; err != nil {
		t.Fatalf("seed device: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/info-device", wampInfoDevice(db))

	w := postJSON(r, "/info-device", `{"api_key":"`+rawKey+`","number":"628111111111"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("info-device: status %d (%s), mau 200", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		`"status":true`,
		`"body":"628111111111"`,
		`"status":"Connect"`,
		`"reply_when":"Personal"`,
		`"wh_read":1`,
		`"reject_call":0`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("info-device: respons tidak mengandung %s — dapat %s", want, body)
		}
	}

	// Device tidak dikenal → 404.
	w = postJSON(r, "/info-device", `{"api_key":"`+rawKey+`","number":"629999999999"}`)
	if w.Code != http.StatusNotFound {
		t.Errorf("device tak dikenal: status %d (%s), mau 404", w.Code, w.Body.String())
	}
}

// Regression end-to-end: key read-only ditolak 403 di endpoint compat.
func TestWampScopeEnforcedDB(t *testing.T) {
	db := auditTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.ApiKey{}, &models.Device{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	cleanup := func() {
		db.Unscoped().Where("name LIKE ?", "wamp-scope-%").Delete(&models.ApiKey{})
		db.Unscoped().Where("email = ?", "wamp-scope@example.test").Delete(&models.User{})
	}
	cleanup()
	defer cleanup()

	u := models.User{Name: "wamp-scope-user", Email: "wamp-scope@example.test"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	rawKey := "wamp-scope-readonly-key"
	ak := models.ApiKey{UserID: u.ID, Name: "wamp-scope-ro", KeyHash: models.HashAPIKey(rawKey), IsActive: true, Scopes: `["messages:read"]`}
	if err := db.Create(&ak).Error; err != nil {
		t.Fatalf("seed api key: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/send-message", wampSend(db, nil, false))
	r.POST("/info-device", wampInfoDevice(db))

	for _, tc := range []struct{ path, body string }{
		{"/send-message", `{"api_key":"` + rawKey + `","number":"62812","message":"bocor"}`},
		{"/info-device", `{"api_key":"` + rawKey + `","number":"62812"}`},
	} {
		w := postJSON(r, tc.path, tc.body)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "INSUFFICIENT_SCOPE") {
			t.Errorf("%s dengan key read-only: status %d (%s), mau 403 INSUFFICIENT_SCOPE", tc.path, w.Code, w.Body.String())
		}
	}
}
