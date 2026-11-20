package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/security"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func searchRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userID", uint(5)); c.Next() })
	r.GET("/ai-search", getAISearchConnection(db))
	r.PUT("/ai-search", upsertAISearchConnection(db))
	r.DELETE("/ai-search", deleteAISearchConnection(db))
	return r
}

func searchDoJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestSearchDTOTanpaKunci(t *testing.T) {
	c := &models.AISearchConnection{Provider: "brave", KeyHint: "wxyz", APIKeyEnc: "RAHASIA-ENKRIPSI",
		SearchStatus: "yes", CheckedProvider: "brave"}
	raw, _ := json.Marshal(searchConnectionDTO(c))
	s := string(raw)
	if strings.Contains(s, "RAHASIA-ENKRIPSI") || !strings.Contains(s, `"searchStatus":"yes"`) ||
		!strings.Contains(s, `"keyHint":"••••wxyz"`) {
		t.Errorf("DTO salah/bocor: %s", s)
	}
	// ganti penyedia -> status lama TIDAK berlaku
	c.Provider = "zai"
	raw, _ = json.Marshal(searchConnectionDTO(c))
	if !strings.Contains(string(raw), `"searchStatus":""`) {
		t.Errorf("status harus kosong setelah ganti penyedia: %s", raw)
	}
	if got, _ := json.Marshal(searchConnectionDTO(nil)); string(got) != `{"configured":false}` {
		t.Errorf("nil: %s", got)
	}
}

func TestUpsertSearchKunciDienkripsiDanTakBocor(t *testing.T) {
	db := capDB(t)
	r := searchRouter(db)
	w := searchDoJSON(r, http.MethodPut, "/ai-search", `{"provider":"brave","apiKey":"BSA-rahasia-1234"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "BSA-rahasia") {
		t.Errorf("respons membocorkan kunci: %s", w.Body.String())
	}
	var conn models.AISearchConnection
	if err := db.First(&conn, "user_id = ?", 5).Error; err != nil {
		t.Fatal(err)
	}
	if conn.APIKeyEnc == "" || strings.Contains(conn.APIKeyEnc, "BSA-rahasia") {
		t.Errorf("kunci harus tersimpan TERENKRIPSI: %q", conn.APIKeyEnc)
	}
	if dec, err := security.Decrypt(conn.APIKeyEnc); err != nil || dec != "BSA-rahasia-1234" {
		t.Errorf("dekripsi gagal: %q %v", dec, err)
	}
	if conn.KeyHint != "1234" {
		t.Errorf("hint 4 karakter terakhir: %q", conn.KeyHint)
	}
}

func TestUpsertSearchValidasi(t *testing.T) {
	r := searchRouter(capDB(t))
	cases := map[string]string{
		"penyedia asing":         `{"provider":"bing","apiKey":"k"}`,
		"brave tanpa kunci":      `{"provider":"brave"}`,
		"zai tanpa kunci":        `{"provider":"zai","apiKey":"  "}`,
		"searxng tanpa url":      `{"provider":"searxng"}`,
		"searxng url bukan http": `{"provider":"searxng","baseUrl":"ftp://x"}`,
		"searxng loopback":       `{"provider":"searxng","baseUrl":"http://127.0.0.1:8888"}`,
		"searxng metadata cloud": `{"provider":"searxng","baseUrl":"http://169.254.169.254"}`,
		"json rusak":             `{`,
	}
	for nama, body := range cases {
		if w := searchDoJSON(r, http.MethodPut, "/ai-search", body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: harus 400, dapat %d (%s)", nama, w.Code, w.Body.String())
		}
	}
}

func TestUpsertSearchGantiPenyediaTidakMewarisiKunciLama(t *testing.T) {
	db := capDB(t)
	r := searchRouter(db)
	if w := searchDoJSON(r, http.MethodPut, "/ai-search", `{"provider":"brave","apiKey":"kunci-brave"}`); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	// pindah ke Z.AI TANPA kunci baru -> tak boleh memakai kunci Brave
	if w := searchDoJSON(r, http.MethodPut, "/ai-search", `{"provider":"zai"}`); w.Code != http.StatusBadRequest {
		t.Errorf("ganti penyedia tanpa kunci harus 400, dapat %d", w.Code)
	}
	// update penyedia SAMA tanpa kunci -> kunci lama dipertahankan
	if w := searchDoJSON(r, http.MethodPut, "/ai-search", `{"provider":"brave","isActive":false}`); w.Code != 200 {
		t.Errorf("update tanpa kunci (penyedia sama) harus 200: %d %s", w.Code, w.Body.String())
	}
	var conn models.AISearchConnection
	db.First(&conn, "user_id = ?", 5)
	if dec, _ := security.Decrypt(conn.APIKeyEnc); dec != "kunci-brave" || conn.IsActive {
		t.Errorf("kunci lama harus bertahan & isActive=false: %q %v", dec, conn.IsActive)
	}
}

func TestClassifySearchProbe(t *testing.T) {
	ok := []service.SearchResult{{Title: "Soekarno", Domain: "x.com"}}
	if s, _ := classifySearchProbe(ok, nil); s != models.CapabilityYes {
		t.Errorf("ada hasil harus yes: %s", s)
	}
	if s, _ := classifySearchProbe(nil, service.ErrSearchEmpty); s != models.CapabilityNo {
		t.Errorf("sehat tapi kosong harus no: %s", s)
	}
	for _, e := range []string{"search error 401: x", "search error 429: x", "search error 500: x",
		"pencarian gagal dihubungi: timeout", "respons Brave tidak dapat dibaca"} {
		if s, _ := classifySearchProbe(nil, errors.New(e)); s != service.ProbeUnknown {
			t.Errorf("%q harus unknown (jangan menandai gagal permanen), dapat %s", e, s)
		}
	}
}

func TestFriendlySearchErrorTidakMembocorkanIsiBesar(t *testing.T) {
	long := "search error 400: " + strings.Repeat("rahasia ", 100)
	if got := friendlySearchError(errors.New(long)); len([]rune(got)) > 220 {
		t.Errorf("pesan harus dipotong: %d", len([]rune(got)))
	}
	if got := friendlySearchError(errors.New("search error 429: x")); !strings.Contains(got, "429") {
		t.Errorf("429 harus ramah: %q", got)
	}
}

func TestFriendlySearchErrorSaldoZAIBukanRateLimit(t *testing.T) {
	err := errors.New(`search error 429: {"error":{"code":"1113","message":"Insufficient balance or no resource package. Please recharge."}}`)
	msg := friendlySearchError(err)
	if !strings.Contains(msg, "paket") || strings.Contains(msg, "batas penggunaan") {
		t.Errorf("1113 harus dijelaskan sebagai paket/saldo, bukan rate limit: %q", msg)
	}
}
