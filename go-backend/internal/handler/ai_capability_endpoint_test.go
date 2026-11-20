package handler

import (
	"encoding/json"
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

func endpointRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userID", uint(5)); c.Next() })
	r.POST("/ai-connection/vision-check", visionCheck(db))
	r.POST("/ai-search/test", testAISearchConnection(db))
	r.PUT("/ai-search", upsertAISearchConnection(db))
	return r
}

// visionServer: model tiruan OpenAI-compatible yang menjawab sesuai skenario.
func visionServer(t *testing.T, status int, answer string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// pakaiServiceTiruan mengganti titik suntik dengan layanan yang memakai klien server tiruan
// (klien bawaan menolak loopback), lalu mengembalikannya saat tes selesai.
func pakaiServiceTiruan(t *testing.T, srv *httptest.Server, baca func(string) string) {
	t.Helper()
	old := newVisionService
	newVisionService = func(p service.AIProvider, key, base string) *service.AIService {
		s := service.NewAIServiceForConnection(p, key, srv.URL)
		s.HTTPClient = srv.Client()
		return s
	}
	t.Cleanup(func() { newVisionService = old })
	_ = baca
}

func seedConn(t *testing.T, db *gorm.DB, model string) uint {
	t.Helper()
	enc, err := security.Encrypt("kunci-uji")
	if err != nil {
		t.Fatal(err)
	}
	c := models.AIConnection{UserID: 5, Provider: "openai", Model: model,
		APIKeyEnc: enc, KeyHint: "-uji", IsActive: true}
	if err := db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func postCheck(r *gin.Engine, path, body string) (int, map[string]interface{}) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestVisionCheckModelMelihatDisimpan(t *testing.T) {
	db := capDB(t)
	id := seedConn(t, db, "gpt-lihat")
	// pemilih tetap: atas=merah (indeks 0), bawah=biru (indeks 1 setelah digeser)
	old := pickIndex
	pickIndex = func(int) int { return 0 }
	t.Cleanup(func() { pickIndex = old })
	srv := visionServer(t, 200, `{"choices":[{"message":{"content":"ATAS=merah, BAWAH=biru"}}]}`)
	pakaiServiceTiruan(t, srv, nil)

	code, out := postCheck(endpointRouter(db), "/ai-connection/vision-check", `{}`)
	if code != 200 || out["status"] != "yes" || out["ok"] != true || out["saved"] != true {
		t.Fatalf("model yang melihat harus yes & tersimpan: %d %v", code, out)
	}
	var c models.AIConnection
	db.First(&c, id)
	if c.VisionStatus != "yes" || c.VisionModel != "gpt-lihat" || c.VisionCheckedAt == nil || !c.VisionChecked() {
		t.Errorf("tersimpan salah: %+v", c)
	}
}

func TestVisionCheckModelMenebakDisimpanNo(t *testing.T) {
	db := capDB(t)
	id := seedConn(t, db, "gpt-buta")
	old := pickIndex
	pickIndex = func(int) int { return 0 }
	t.Cleanup(func() { pickIndex = old })
	// proxy menjawab 200 tetapi isinya tebakan salah (model tak melihat gambar)
	srv := visionServer(t, 200, `{"choices":[{"message":{"content":"ATAS=hijau, BAWAH=kuning"}}]}`)
	pakaiServiceTiruan(t, srv, nil)
	_, out := postCheck(endpointRouter(db), "/ai-connection/vision-check", `{}`)
	if out["status"] != "no" || out["saved"] != true {
		t.Fatalf("tebakan salah harus no & tersimpan: %v", out)
	}
	var c models.AIConnection
	db.First(&c, id)
	if c.VisionStatus != "no" {
		t.Errorf("harus tersimpan no: %q", c.VisionStatus)
	}
}

func TestVisionCheckGalatSementaraTidakDisimpan(t *testing.T) {
	for _, status := range []int{429, 500, 401} {
		db := capDB(t)
		id := seedConn(t, db, "gpt-x")
		pakaiServiceTiruan(t, visionServer(t, status, `{"error":"x"}`), nil)
		code, out := postCheck(endpointRouter(db), "/ai-connection/vision-check", `{}`)
		if code != 200 || out["status"] != service.ProbeUnknown || out["saved"] != false {
			t.Errorf("HTTP %d: harus unknown & tak disimpan: %d %v", status, code, out)
		}
		var c models.AIConnection
		db.First(&c, id)
		if c.VisionStatus != "" {
			t.Errorf("HTTP %d: status tak boleh tersimpan: %q", status, c.VisionStatus)
		}
	}
}

func TestVisionCheckModelTeksMenolakDisimpanNo(t *testing.T) {
	db := capDB(t)
	id := seedConn(t, db, "model-teks")
	pakaiServiceTiruan(t, visionServer(t, 400, `{"error":"no image"}`), nil)
	_, out := postCheck(endpointRouter(db), "/ai-connection/vision-check", `{}`)
	if out["status"] != "no" || out["saved"] != true {
		t.Fatalf("400 harus no & tersimpan: %v", out)
	}
	var c models.AIConnection
	db.First(&c, id)
	if c.VisionStatus != "no" || c.VisionModel != "model-teks" || !c.VisionChecked() {
		t.Errorf("tak tersimpan benar: %+v", c)
	}
	// ganti model -> status lama otomatis tak berlaku
	db.Model(&c).Update("model", "model-lain")
	db.First(&c, id)
	if c.VisionChecked() {
		t.Error("ganti model harus membatalkan status")
	}
}

func TestVisionCheckNilaiFormTidakMenempelKeKoneksiTersimpan(t *testing.T) {
	db := capDB(t)
	id := seedConn(t, db, "model-tersimpan")
	pakaiServiceTiruan(t, visionServer(t, 400, `{"error":"x"}`), nil)
	// uji nilai FORM (provider+kunci diisi) -> tak boleh menyimpan ke koneksi tersimpan
	_, out := postCheck(endpointRouter(db), "/ai-connection/vision-check",
		`{"provider":"openai","apiKey":"kunci-form","model":"model-form"}`)
	if out["saved"] != false {
		t.Errorf("uji nilai form tak boleh menyimpan: %v", out)
	}
	var c models.AIConnection
	db.First(&c, id)
	if c.VisionStatus != "" {
		t.Errorf("status koneksi tersimpan tak boleh berubah: %q", c.VisionStatus)
	}
}

func TestVisionCheckTanpaKoneksi400(t *testing.T) {
	code, out := postCheck(endpointRouter(capDB(t)), "/ai-connection/vision-check", `{}`)
	if code != http.StatusBadRequest || out["ok"] != false {
		t.Errorf("tanpa koneksi harus 400: %d %v", code, out)
	}
}
