package whatsapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func searchFlowDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AISearchConnection{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedSearch(t *testing.T, db *gorm.DB, status, checkedProvider string, active bool) {
	t.Helper()
	now := time.Now()
	c := models.AISearchConnection{UserID: 5, Provider: "brave", IsActive: true, SearchStatus: status,
		CheckedProvider: checkedProvider, CheckedAt: &now}
	if err := db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	if !active { // GORM mengabaikan false pada Create bila ada default; setel eksplisit
		db.Model(&c).Update("is_active", false)
	}
}

// fakeSearcher menghitung panggilan agar cache/batas bisa dibuktikan.
type fakeSearcher struct {
	calls int
	res   []service.SearchResult
	err   error
}

func (f *fakeSearcher) Search(ctx context.Context, q string) ([]service.SearchResult, error) {
	f.calls++
	return f.res, f.err
}

func (f *fakeSearcher) of(*models.AISearchConnection) service.Searcher { return f }

var cfgOn = &models.AIReplyConfig{WebSearch: true}

func jalankan(db *gorm.DB, g *service.SearchGuard, f *fakeSearcher, cfg *models.AIReplyConfig, text string) (string, bool) {
	return webSearchNote(context.Background(), db, g, f.of, 5, cfg, text)
}

func TestWebSearchNoteBerhasilMenyisipkanBlok(t *testing.T) {
	db := searchFlowDB(t)
	seedSearch(t, db, "yes", "brave", true)
	f := &fakeSearcher{res: []service.SearchResult{{Title: "T", Domain: "d.com", Snippet: "s"}}}
	note, used := jalankan(db, service.NewSearchGuard(time.Minute, 60), f, cfgOn, "berita hari ini")
	if !used || f.calls != 1 || note == "" || note == searchUnavailableNote {
		t.Fatalf("harus blok hasil: used=%v calls=%d note=%q", used, f.calls, note)
	}
}

func TestWebSearchNoteTanpaPenyediaLulusUjiTidakMencari(t *testing.T) {
	cases := map[string]func(db *gorm.DB){
		"tanpa koneksi":           func(db *gorm.DB) {},
		"belum dicek":             func(db *gorm.DB) { seedSearch(t, db, "", "", true) },
		"uji gagal (no)":          func(db *gorm.DB) { seedSearch(t, db, "no", "brave", true) },
		"uji untuk penyedia lain": func(db *gorm.DB) { seedSearch(t, db, "yes", "zai", true) },
		"koneksi nonaktif":        func(db *gorm.DB) { seedSearch(t, db, "yes", "brave", false) },
	}
	for nama, seed := range cases {
		t.Run(nama, func(t *testing.T) {
			db := searchFlowDB(t)
			seed(db)
			f := &fakeSearcher{res: []service.SearchResult{{Title: "T", Domain: "d", Snippet: "s"}}}
			note, used := jalankan(db, service.NewSearchGuard(time.Minute, 60), f, cfgOn, "halo")
			if note != "" || used || f.calls != 0 {
				t.Errorf("tak boleh mencari: note=%q used=%v calls=%d", note, used, f.calls)
			}
		})
	}
}

func TestWebSearchNoteFiturMatiAtauTanpaTeks(t *testing.T) {
	db := searchFlowDB(t)
	seedSearch(t, db, "yes", "brave", true)
	f := &fakeSearcher{res: []service.SearchResult{{Title: "T", Domain: "d", Snippet: "s"}}}
	g := service.NewSearchGuard(time.Minute, 60)
	if n, _ := jalankan(db, g, f, &models.AIReplyConfig{}, "halo"); n != "" || f.calls != 0 {
		t.Error("WebSearch mati -> tak mencari")
	}
	if n, _ := jalankan(db, g, f, cfgOn, "   "); n != "" || f.calls != 0 {
		t.Error("tanpa teks (gambar tanpa caption) -> tak mencari")
	}
}

func TestWebSearchNoteGagalTetapMenjawabDenganCatatanJujur(t *testing.T) {
	for nama, f := range map[string]*fakeSearcher{
		"galat":   {err: errors.New("search error 429: x")},
		"kosong":  {err: service.ErrSearchEmpty},
		"nol":     {},
		"timeout": {err: context.DeadlineExceeded},
	} {
		t.Run(nama, func(t *testing.T) {
			db := searchFlowDB(t)
			seedSearch(t, db, "yes", "brave", true)
			note, used := jalankan(db, service.NewSearchGuard(time.Minute, 60), f, cfgOn, "berita")
			if note != searchUnavailableNote || used {
				t.Errorf("gagal harus catatan jujur (bukan diam, bukan mengarang): note=%q used=%v", note, used)
			}
		})
	}
}

func TestWebSearchNoteCacheMenghematPencarian(t *testing.T) {
	db := searchFlowDB(t)
	seedSearch(t, db, "yes", "brave", true)
	f := &fakeSearcher{res: []service.SearchResult{{Title: "T", Domain: "d", Snippet: "s"}}}
	g := service.NewSearchGuard(time.Minute, 60)
	a, _ := jalankan(db, g, f, cfgOn, "Harga  Emas")
	b, usedB := jalankan(db, g, f, cfgOn, "harga emas")
	if f.calls != 1 || a != b || !usedB {
		t.Errorf("kueri sama (tak peka huruf/spasi) harus dari cache: calls=%d", f.calls)
	}
}

func TestWebSearchNoteBatasPerJam(t *testing.T) {
	db := searchFlowDB(t)
	seedSearch(t, db, "yes", "brave", true)
	f := &fakeSearcher{res: []service.SearchResult{{Title: "T", Domain: "d", Snippet: "s"}}}
	g := service.NewSearchGuard(time.Minute, 2)
	for i, q := range []string{"a", "b"} {
		if n, _ := jalankan(db, g, f, cfgOn, q); n == searchUnavailableNote || n == "" {
			t.Fatalf("pencarian ke-%d harus lolos", i+1)
		}
	}
	note, used := jalankan(db, g, f, cfgOn, "c")
	if note != searchUnavailableNote || used || f.calls != 2 {
		t.Errorf("melebihi batas -> catatan jujur tanpa memanggil penyedia: note=%q calls=%d", note, f.calls)
	}
}

func TestWebSearchNotePenyediaTakBisaDibuat(t *testing.T) {
	db := searchFlowDB(t)
	seedSearch(t, db, "yes", "brave", true)
	nilSearcher := func(*models.AISearchConnection) service.Searcher { return nil }
	note, used := webSearchNote(context.Background(), db, service.NewSearchGuard(time.Minute, 60), nilSearcher, 5, cfgOn, "halo")
	if note != searchUnavailableNote || used {
		t.Errorf("kunci tak terbaca -> catatan jujur, bukan panik/diam: %q", note)
	}
}
