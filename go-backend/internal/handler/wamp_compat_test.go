// Test endpoint kompatibilitas WAMP + Slackersender (wamp_compat.go).
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/whatsapp"
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

// ── Pure function: splitWampNumbers (banyak nomor dipisah "|") ───────────────

func TestSplitWampNumbers(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"satu nomor", "628111", []string{"628111"}},
		{"pemisah pipe", "628111|628222|628333", []string{"628111", "628222", "628333"}},
		{"spasi di-trim", " 628111 | 628222 ", []string{"628111", "628222"}},
		{"entri kosong dibuang", "628111||628222|", []string{"628111", "628222"}},
		{"duplikat dibuang, urutan asli", "628222|628111|628222", []string{"628222", "628111"}},
		{"pemisah lain ditoleransi", "628111,628222;628333\n628444", []string{"628111", "628222", "628333", "628444"}},
		{"hanya pemisah", "| |", []string{}},
		{"kosong", "", []string{}},
		{"format 08xx dipertahankan apa adanya", "0811|62822", []string{"0811", "62822"}},
	}
	for _, tc := range cases {
		got := splitWampNumbers(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("%s: splitWampNumbers(%q) = %v, mau %v", tc.name, tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: splitWampNumbers(%q) = %v, mau %v", tc.name, tc.in, got, tc.want)
				break
			}
		}
	}
}

// ── Handler tanpa DB: validasi input ditolak sebelum auth ───────────────────

func newWampTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// db nil aman: semua kasus di bawah ditolak sebelum menyentuh DB.
	r.POST("/send-message", wampSend(nil, nil, false))
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

// number yang hanya berisi pemisah dianggap kosong → 400 sebelum menyentuh DB.
func TestWampSendMessageEmptyNumbers(t *testing.T) {
	r := newWampTestRouter()
	for _, number := range []string{"", "|", " | | "} {
		w := postJSON(r, "/send-message", `{"api_key":"kunci","number":"`+number+`","message":"hai"}`)
		if w.Code != http.StatusBadRequest {
			t.Errorf("number %q: status %d (%s), mau 400", number, w.Code, w.Body.String())
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

// End-to-end banyak nomor: device sengaja TIDAK terhubung sehingga setiap
// pengiriman gagal terkontrol (tanpa menyentuh WhatsApp sungguhan). Yang
// diverifikasi: satu catatan pesan per nomor unik, ids/total di respons, dan
// respons satu nomor tetap tanpa ids.
func TestWampSendMultiNumbersDB(t *testing.T) {
	db := auditTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.ApiKey{}, &models.Device{}, &models.Message{}, &models.MessageReport{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	cleanup := func() {
		var u models.User
		if err := db.Unscoped().Where("email = ?", "wamp-multi@example.test").First(&u).Error; err == nil {
			db.Unscoped().Where("user_id = ?", u.ID).Delete(&models.MessageReport{})
			db.Unscoped().Where("user_id = ?", u.ID).Delete(&models.Message{})
		}
		db.Unscoped().Where("name LIKE ?", "wamp-multi-%").Delete(&models.ApiKey{})
		db.Unscoped().Where("name LIKE ?", "wamp-multi-%").Delete(&models.Device{})
		db.Unscoped().Where("email = ?", "wamp-multi@example.test").Delete(&models.User{})
	}
	cleanup()
	defer cleanup()

	u := models.User{Name: "wamp-multi-user", Email: "wamp-multi@example.test"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	rawKey := "wamp-multi-key-1"
	ak := models.ApiKey{UserID: u.ID, Name: "wamp-multi-key", KeyHash: models.HashAPIKey(rawKey), IsActive: true}
	if err := db.Create(&ak).Error; err != nil {
		t.Fatalf("seed api key: %v", err)
	}
	dev := models.Device{UserID: u.ID, Name: "wamp-multi-dev", Phone: "628123450000", Status: "disconnected"}
	if err := db.Create(&dev).Error; err != nil {
		t.Fatalf("seed device: %v", err)
	}

	wm := whatsapp.NewManager(t.TempDir(), db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/send-message", wampSend(db, wm, false))

	// Banyak nomor: 3 entri, 1 duplikat, 1 kosong → 2 nomor unik.
	w := postJSON(r, "/send-message", `{"api_key":"`+rawKey+`","sender":"628123450000","number":"628111000001| 628111000002 ||628111000001","message":"Halo"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("multi nomor: status %d (%s), mau 200", w.Code, w.Body.String())
	}
	var resp struct {
		Status bool   `json:"status"`
		ID     uint   `json:"id"`
		IDs    []uint `json:"ids"`
		Total  int    `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("respons bukan JSON: %v (%s)", err, w.Body.String())
	}
	if !resp.Status || resp.Total != 2 || len(resp.IDs) != 2 || resp.ID != resp.IDs[0] {
		t.Fatalf("respons multi nomor tidak sesuai: %s", w.Body.String())
	}
	var msgs []models.Message
	db.Where("user_id = ?", u.ID).Order("id").Find(&msgs)
	if len(msgs) != 2 || msgs[0].To != "628111000001" || msgs[1].To != "628111000002" {
		t.Fatalf("catatan pesan tidak sesuai: %+v", msgs)
	}
	for _, m := range msgs {
		if m.Content != "Halo" || m.Via != "api" {
			t.Errorf("pesan %d: content/via salah: %q / %q", m.ID, m.Content, m.Via)
		}
	}

	// Pengiriman background: jeda acak 2-5 dtk antar nomor; tunggu keduanya "failed".
	deadline := time.Now().Add(20 * time.Second)
	for {
		var failed int64
		db.Model(&models.Message{}).Where("user_id = ? AND status = ?", u.ID, "failed").Count(&failed)
		if failed == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("kedua pesan tidak berstatus failed dalam 20 dtk (baru %d)", failed)
		}
		time.Sleep(300 * time.Millisecond)
	}
	var reports int64
	db.Model(&models.MessageReport{}).Where("user_id = ?", u.ID).Count(&reports)
	if reports != 2 {
		t.Errorf("laporan per nomor = %d, mau 2", reports)
	}

	// Satu nomor: respons tetap seperti kontrak lama (tanpa ids/total).
	w = postJSON(r, "/send-message", `{"api_key":"`+rawKey+`","sender":"628123450000","number":"628111000009","message":"Halo"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("satu nomor: status %d (%s), mau 200", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"ids"`) || strings.Contains(w.Body.String(), `"total"`) {
		t.Errorf("respons satu nomor tidak boleh berisi ids/total: %s", w.Body.String())
	}

	// Tunggu goroutine pengiriman terakhir selesai sebelum cleanup, supaya
	// tidak menulis laporan setelah data test dihapus.
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		var failed int64
		db.Model(&models.Message{}).Where("user_id = ? AND status = ?", u.ID, "failed").Count(&failed)
		if failed == 3 {
			break
		}
	}
	time.Sleep(300 * time.Millisecond) // recordReport berjalan tepat setelah status diperbarui
}

// Format Slackersender: penerima boleh nomor personal ATAU ID grup
// ("120363398726060191@g.us", 23 karakter) dan keduanya boleh dicampur dengan "|".
// Regression: kolom messages.to dulu varchar(20) sehingga ID grup gagal disimpan (500).
func TestWampSendGroupJIDDB(t *testing.T) {
	db := auditTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.ApiKey{}, &models.Device{}, &models.Message{}, &models.MessageReport{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	cleanup := func() {
		var u models.User
		if err := db.Unscoped().Where("email = ?", "wamp-group@example.test").First(&u).Error; err == nil {
			db.Unscoped().Where("user_id = ?", u.ID).Delete(&models.MessageReport{})
			db.Unscoped().Where("user_id = ?", u.ID).Delete(&models.Message{})
		}
		db.Unscoped().Where("name LIKE ?", "wamp-group-%").Delete(&models.ApiKey{})
		db.Unscoped().Where("name LIKE ?", "wamp-group-%").Delete(&models.Device{})
		db.Unscoped().Where("email = ?", "wamp-group@example.test").Delete(&models.User{})
	}
	cleanup()
	defer cleanup()

	u := models.User{Name: "wamp-group-user", Email: "wamp-group@example.test"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	rawKey := "wamp-group-key-1"
	ak := models.ApiKey{UserID: u.ID, Name: "wamp-group-key", KeyHash: models.HashAPIKey(rawKey), IsActive: true}
	if err := db.Create(&ak).Error; err != nil {
		t.Fatalf("seed api key: %v", err)
	}
	dev := models.Device{UserID: u.ID, Name: "wamp-group-dev", Phone: "628123460000", Status: "disconnected"}
	if err := db.Create(&dev).Error; err != nil {
		t.Fatalf("seed device: %v", err)
	}

	wm := whatsapp.NewManager(t.TempDir(), db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/send-message", wampSend(db, wm, false))

	const group = "120363398726060191@g.us"
	w := postJSON(r, "/send-message", `{"api_key":"`+rawKey+`","sender":"628123460000","number":"628111000003|`+group+`","message":"Halo"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("nomor + ID grup: status %d (%s), mau 200", w.Code, w.Body.String())
	}
	var msgs []models.Message
	db.Where("user_id = ?", u.ID).Order("id").Find(&msgs)
	if len(msgs) != 2 || msgs[0].To != "628111000003" || msgs[1].To != group {
		t.Fatalf("catatan pesan tidak sesuai: %+v", msgs)
	}

	// Tunggu goroutine pengiriman selesai sebelum cleanup.
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		var failed int64
		db.Model(&models.Message{}).Where("user_id = ? AND status = ?", u.ID, "failed").Count(&failed)
		if failed == 2 {
			break
		}
	}
	time.Sleep(300 * time.Millisecond)
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

// Alias "/api/..." harus terdaftar sama persis dengan path root: klien yang
// base URL-nya diisi "<domain>/api" sebelumnya kena 404 (log produksi).
// db/wm nil aman — body kosong ditolak 400 sebelum menyentuh DB.
func TestWampCompatRoutesRegisteredWithAPIAlias(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerWampCompatRoutes(r, nil, nil)

	paths := []string{"/send-message", "/send-media", "/send-location", "/info-device", "/check-number"}
	for _, prefix := range []string{"", "/api"} {
		for _, p := range paths {
			for _, method := range []string{http.MethodPost, http.MethodGet} {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(method, prefix+p, strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				r.ServeHTTP(w, req)
				if w.Code != http.StatusBadRequest {
					t.Errorf("%s %s%s: status %d (%s), mau 400 (route terdaftar, validasi menolak)", method, prefix, p, w.Code, w.Body.String())
				}
			}
		}
	}
}

func TestNormalizeWampPhone(t *testing.T) {
	cases := map[string]string{
		"089520001337":  "6289520001337",
		"6289520001337": "6289520001337",
		"89520001337":   "89520001337",
		"0":             "0",
		"":              "",
	}
	for in, want := range cases {
		if got := normalizeWampPhone(in); got != want {
			t.Errorf("normalizeWampPhone(%q) = %q, mau %q", in, got, want)
		}
	}
}
