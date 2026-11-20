package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// capture: server uji yang merekam request terakhir.
type capture struct {
	method, path, rawQuery string
	header                 http.Header
	body                   []byte
}

func searchServer(t *testing.T, code int, resp string) (*httptest.Server, *capture) {
	t.Helper()
	c := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.method, c.path, c.rawQuery, c.header = r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Clone()
		c.body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(resp))
	}))
	t.Cleanup(srv.Close)
	return srv, c
}

func newTestSearcher(t *testing.T, cfg SearcherConfig, srv *httptest.Server) Searcher {
	t.Helper()
	cfg.HTTPClient = srv.Client()
	cfg.endpointOverride = srv.URL
	s, err := NewSearcher(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSearchZAIRequest(t *testing.T) {
	srv, c := searchServer(t, 200, `{"search_result":[{"title":"T","link":"https://a.com","content":"isi"}]}`)
	s := newTestSearcher(t, SearcherConfig{Provider: SearchZAI, APIKey: "KUNCI"}, srv)
	res, err := s.Search(context.Background(), "  harga emas hari ini ")
	if err != nil || len(res) != 1 {
		t.Fatalf("res=%v err=%v", res, err)
	}
	if c.method != http.MethodPost || c.header.Get("Authorization") != "Bearer KUNCI" {
		t.Errorf("method/auth salah: %s %q", c.method, c.header.Get("Authorization"))
	}
	var b map[string]interface{}
	if err := json.Unmarshal(c.body, &b); err != nil {
		t.Fatal(err)
	}
	if b["search_query"] != "harga emas hari ini" || b["count"] != float64(maxSearchResults) {
		t.Errorf("body salah: %s", c.body)
	}
	// Z.AI menolak (error 1214) bila search_engine kosong — harus selalu terkirim.
	if b["search_engine"] != "search-prime" {
		t.Errorf("search_engine harus search-prime: %s", c.body)
	}
}

func TestSearchBraveRequest(t *testing.T) {
	srv, c := searchServer(t, 200, `{"web":{"results":[{"title":"B","url":"https://b.com","description":"d"}]}}`)
	s := newTestSearcher(t, SearcherConfig{Provider: SearchBrave, APIKey: "TOKEN"}, srv)
	if _, err := s.Search(context.Background(), "berita & cuaca"); err != nil {
		t.Fatal(err)
	}
	if c.method != http.MethodGet || c.header.Get("X-Subscription-Token") != "TOKEN" {
		t.Errorf("method/token salah: %s %q", c.method, c.header.Get("X-Subscription-Token"))
	}
	if !strings.Contains(c.rawQuery, "q=berita+%26+cuaca") || !strings.Contains(c.rawQuery, "count=5") {
		t.Errorf("kueri harus di-escape: %q", c.rawQuery)
	}
	if strings.Contains(c.header.Get("Authorization"), "TOKEN") {
		t.Error("token Brave tak boleh lewat Authorization")
	}
}

func TestSearchSearXNGRequest(t *testing.T) {
	srv, c := searchServer(t, 200, `{"results":[{"title":"S","url":"https://s.org","content":"c"}]}`)
	s, err := NewSearcher(SearcherConfig{Provider: SearchSearXNG, BaseURL: srv.URL + "/", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Search(context.Background(), "uji"); err != nil {
		t.Fatal(err)
	}
	if c.path != "/search" || !strings.Contains(c.rawQuery, "format=json") || !strings.Contains(c.rawQuery, "q=uji") {
		t.Errorf("jalur/kueri salah: %s?%s", c.path, c.rawQuery)
	}
}

func TestSearchSearXNGFormatJSONDimatikan403(t *testing.T) {
	srv, _ := searchServer(t, 403, `<html>Forbidden</html>`)
	s, _ := NewSearcher(SearcherConfig{Provider: SearchSearXNG, BaseURL: srv.URL, HTTPClient: srv.Client()})
	_, err := s.Search(context.Background(), "uji")
	if err == nil || !strings.Contains(err.Error(), "search error 403") {
		t.Errorf("403 harus menjadi galat bersebab: %v", err)
	}
}

func TestSearchGalatHTTPDipotongDanBerkode(t *testing.T) {
	long := strings.Repeat("x", 5000)
	for _, code := range []int{401, 422, 429, 500} {
		srv, _ := searchServer(t, code, long)
		s := newTestSearcher(t, SearcherConfig{Provider: SearchBrave, APIKey: "k"}, srv)
		_, err := s.Search(context.Background(), "a")
		if err == nil || !strings.Contains(err.Error(), "search error") {
			t.Fatalf("HTTP %d harus galat: %v", code, err)
		}
		if len(err.Error()) > 250 {
			t.Errorf("HTTP %d: pesan galat tak dipotong (%d)", code, len(err.Error()))
		}
	}
}

func TestSearchKueriKosongDanKonfigurasi(t *testing.T) {
	srv, _ := searchServer(t, 200, `{}`)
	s := newTestSearcher(t, SearcherConfig{Provider: SearchBrave, APIKey: "k"}, srv)
	if _, err := s.Search(context.Background(), "   "); err == nil {
		t.Error("kueri kosong harus galat tanpa memanggil penyedia")
	}
	for nama, cfg := range map[string]SearcherConfig{
		"zai tanpa key":     {Provider: SearchZAI},
		"brave tanpa key":   {Provider: SearchBrave, APIKey: "  "},
		"searxng tanpa url": {Provider: SearchSearXNG},
		"penyedia asing":    {Provider: "bing", APIKey: "k"},
	} {
		if _, err := NewSearcher(cfg); err == nil {
			t.Errorf("%s: harus galat", nama)
		}
	}
}

func TestSearchKonteksDibatalkan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()
	s := newTestSearcher(t, SearcherConfig{Provider: SearchBrave, APIKey: "k"}, srv)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := s.Search(ctx, "uji"); err == nil {
		t.Error("konteks habis harus menghentikan pencarian")
	}
}

// Klien BAWAAN (anti-SSRF) harus menolak instance di alamat privat/loopback —
// tanpa ini, pengguna bisa mengarahkan "SearXNG" ke jaringan internal server.
func TestSearchSearXNGDiAlamatPrivatDitolakKlienBawaan(t *testing.T) {
	srv, c := searchServer(t, 200, `{"results":[{"title":"x","url":"https://a.com","content":"y"}]}`)
	// HTTPClient sengaja TIDAK diisi -> NewSearcher memakai aiHTTPClient() (SafeClient).
	s, err := NewSearcher(SearcherConfig{Provider: SearchSearXNG, BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Search(context.Background(), "uji"); err == nil {
		t.Fatal("alamat loopback (127.0.0.1) harus ditolak klien anti-SSRF")
	}
	if c.method != "" {
		t.Errorf("server internal TIDAK boleh tersentuh sama sekali, tetapi menerima %s", c.method)
	}
}

func TestSearchKosongBukanGalatJaringan(t *testing.T) {
	srv, _ := searchServer(t, 200, `{"web":{"results":[]}}`)
	s := newTestSearcher(t, SearcherConfig{Provider: SearchBrave, APIKey: "k"}, srv)
	if _, err := s.Search(context.Background(), "uji"); !errors.Is(err, ErrSearchEmpty) {
		t.Errorf("0 hasil harus ErrSearchEmpty, dapat %v", err)
	}
}
