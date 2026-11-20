package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/security"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/service"
	"gorm.io/gorm"
)

// searchServerTiruan: penyedia pencarian tiruan; klien bawaan menolak loopback,
// jadi NewSearcher disuntik klien server tiruan lewat titik suntik newSearcher.
func searchServerTiruan(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	old := newSearcher
	newSearcher = func(cfg service.SearcherConfig) (service.Searcher, error) {
		cfg.HTTPClient = srv.Client()
		cfg.BaseURL = srv.URL // SearXNG; Brave/Z.AI memakai URL tetap -> lihat catatan di bawah
		return &fixedSearcher{inner: cfg, url: srv.URL, client: srv.Client()}, nil
	}
	t.Cleanup(func() { newSearcher = old })
	return srv
}

// fixedSearcher: membungkus SearcherConfig agar endpoint Brave/Z.AI (URL tetap di
// produksi) diarahkan ke server tiruan. Hanya dipakai tes.
type fixedSearcher struct {
	inner  service.SearcherConfig
	url    string
	client *http.Client
}

func (f *fixedSearcher) Search(ctx context.Context, q string) ([]service.SearchResult, error) {
	cfg := f.inner
	cfg.Provider = service.SearchSearXNG // jalur GET JSON generik ke server tiruan
	cfg.BaseURL = f.url
	cfg.HTTPClient = f.client
	s, err := service.NewSearcher(cfg)
	if err != nil {
		return nil, err
	}
	return s.Search(ctx, q)
}

func seedSearchConn(t *testing.T, db *gorm.DB, provider string) uint {
	t.Helper()
	enc, err := security.Encrypt("kunci-pencarian")
	if err != nil {
		t.Fatal(err)
	}
	c := models.AISearchConnection{UserID: 5, Provider: provider, APIKeyEnc: enc, KeyHint: "aran", IsActive: true}
	if err := db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	return c.ID
}

const hasilBrave = `{"results":[{"title":"Soekarno","url":"https://www.wikipedia.org/s","content":"Presiden pertama"}]}`

func TestSearchTestLulusDisimpanYes(t *testing.T) {
	db := capDB(t)
	id := seedSearchConn(t, db, "brave")
	searchServerTiruan(t, 200, hasilBrave)
	code, out := postCheck(endpointRouter(db), "/ai-search/test", `{}`)
	if code != 200 || out["status"] != "yes" || out["ok"] != true || out["saved"] != true || out["count"] != float64(1) {
		t.Fatalf("harus yes & tersimpan: %d %v", code, out)
	}
	var c models.AISearchConnection
	db.First(&c, id)
	if c.SearchStatus != "yes" || c.CheckedProvider != "brave" || c.CheckedAt == nil || !c.SearchChecked() {
		t.Errorf("tersimpan salah: %+v", c)
	}
	// ganti penyedia -> status lama otomatis tak berlaku
	db.Model(&c).Update("provider", "zai")
	db.First(&c, id)
	if c.SearchChecked() {
		t.Error("ganti penyedia harus membatalkan status")
	}
}

func TestSearchTestGalatSementaraTidakDisimpan(t *testing.T) {
	for _, status := range []int{401, 429, 500} {
		db := capDB(t)
		id := seedSearchConn(t, db, "brave")
		searchServerTiruan(t, status, `{"error":"x"}`)
		code, out := postCheck(endpointRouter(db), "/ai-search/test", `{}`)
		if code != 200 || out["status"] != service.ProbeUnknown || out["saved"] != false {
			t.Errorf("HTTP %d: harus unknown & tak disimpan: %d %v", status, code, out)
		}
		var c models.AISearchConnection
		db.First(&c, id)
		if c.SearchStatus != "" {
			t.Errorf("HTTP %d: status tak boleh tersimpan: %q", status, c.SearchStatus)
		}
		if msg, _ := out["message"].(string); strings.Contains(msg, "kunci-pencarian") {
			t.Errorf("pesan membocorkan kunci: %q", msg)
		}
	}
}

func TestSearchTestKosongDisimpanNo(t *testing.T) {
	db := capDB(t)
	id := seedSearchConn(t, db, "brave")
	searchServerTiruan(t, 200, `{"results":[]}`)
	_, out := postCheck(endpointRouter(db), "/ai-search/test", `{}`)
	if out["status"] != "no" || out["saved"] != true {
		t.Fatalf("sehat tapi kosong harus no: %v", out)
	}
	var c models.AISearchConnection
	db.First(&c, id)
	if c.SearchStatus != "no" {
		t.Errorf("harus tersimpan no: %q", c.SearchStatus)
	}
}

func TestSearchTestNilaiFormTidakMenempel(t *testing.T) {
	db := capDB(t)
	id := seedSearchConn(t, db, "brave")
	searchServerTiruan(t, 200, hasilBrave)
	_, out := postCheck(endpointRouter(db), "/ai-search/test", `{"provider":"zai","apiKey":"kunci-form"}`)
	if out["saved"] != false {
		t.Errorf("uji nilai form tak boleh menyimpan: %v", out)
	}
	var c models.AISearchConnection
	db.First(&c, id)
	if c.SearchStatus != "" {
		t.Errorf("koneksi tersimpan tak boleh berubah: %q", c.SearchStatus)
	}
}

func TestSearchTestTanpaKoneksi400DanSSRF(t *testing.T) {
	r := endpointRouter(capDB(t))
	if code, out := postCheck(r, "/ai-search/test", `{}`); code != http.StatusBadRequest || out["ok"] != false {
		t.Errorf("tanpa koneksi harus 400: %d %v", code, out)
	}
	// SearXNG di alamat privat ditolak SEBELUM dihubungi
	_, out := postCheck(r, "/ai-search/test", `{"provider":"searxng","baseUrl":"http://127.0.0.1:8888"}`)
	if out["ok"] != false || out["status"] != service.ProbeUnknown {
		t.Errorf("alamat privat harus ditolak: %v", out)
	}
}
