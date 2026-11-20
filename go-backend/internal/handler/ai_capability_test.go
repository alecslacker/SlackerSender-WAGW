package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func capDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Device{}, &models.AIReplyConfig{},
		&models.AIConnection{}, &models.AISearchConnection{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestVisionCheckedHanyaUntukModelYangDicek(t *testing.T) {
	now := time.Now()
	c := &models.AIConnection{Model: "m1", VisionStatus: "yes", VisionModel: "m1", VisionCheckedAt: &now}
	if !c.VisionChecked() || visionStatusFor(c) != "yes" {
		t.Fatalf("model sama harus terhitung: %q", visionStatusFor(c))
	}
	c.Model = "m2" // model diganti -> status lama TIDAK berlaku
	if c.VisionChecked() || visionStatusFor(c) != "" {
		t.Errorf("ganti model harus jadi 'belum dicek', dapat %q", visionStatusFor(c))
	}
	if (&models.AIConnection{Model: "m"}).VisionChecked() {
		t.Error("tanpa status = belum dicek")
	}
	var nilConn *models.AIConnection
	if nilConn.VisionChecked() {
		t.Error("nil aman = belum dicek")
	}
}

func TestSearchCheckedHanyaUntukPenyediaYangDicek(t *testing.T) {
	c := &models.AISearchConnection{Provider: "brave", SearchStatus: "yes", CheckedProvider: "brave"}
	if !c.SearchChecked() {
		t.Fatal("penyedia sama harus terhitung")
	}
	c.Provider = "zai"
	if c.SearchChecked() {
		t.Error("ganti penyedia harus jadi 'belum dicek'")
	}
	if (&models.AISearchConnection{Provider: "brave"}).SearchChecked() {
		t.Error("tanpa status = belum dicek")
	}
}

func TestConnectionDTOMemuatStatusGambarTanpaKunci(t *testing.T) {
	now := time.Now()
	conn := &models.AIConnection{Provider: "custom", Model: "m1", KeyHint: "abcd", APIKeyEnc: "RAHASIA-ENKRIPSI",
		VisionStatus: "no", VisionModel: "m1", VisionCheckedAt: &now}
	raw, _ := json.Marshal(connectionDTO(conn))
	s := string(raw)
	if !strings.Contains(s, `"visionStatus":"no"`) || !strings.Contains(s, `"visionModel":"m1"`) {
		t.Errorf("DTO tak memuat status gambar: %s", s)
	}
	if strings.Contains(s, "RAHASIA-ENKRIPSI") {
		t.Errorf("DTO membocorkan kunci terenkripsi: %s", s)
	}
}

func TestCreateAIReplyConfigDefaultBacaGambarAktif(t *testing.T) {
	db := capDB(t)
	db.Create(&models.User{ID: 5, UUID: "00000000-0000-0000-0000-000000000005", Name: "T", Email: "t@test.local", Password: "x"})
	db.Create(&models.Device{ID: 1, UserID: 5, Name: "D", Phone: "628", Status: "connected"})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userID", uint(5)); c.Next() })
	r.POST("/ai-reply", createAIReplyConfig(db))

	post := func(body string) models.AIReplyConfig {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/ai-reply", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		var out struct {
			Config models.AIReplyConfig `json:"config"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out.Config
	}
	if c := post(`{"deviceId":1}`); !c.ReadImages || c.WebSearch {
		t.Errorf("config baru: ReadImages harus true & WebSearch false, dapat %v/%v", c.ReadImages, c.WebSearch)
	}
	if c := post(`{"deviceId":1,"readImages":false}`); c.ReadImages {
		t.Error("pilihan eksplisit false harus dihormati")
	}
}
