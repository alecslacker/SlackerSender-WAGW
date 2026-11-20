package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// Bentuk permintaan/respons di sini diambil dari dokumentasi/kode RESMI tiap penyedia
// (Tavily: docs.tavily.com; BoCha: github.com/BochaAI; Querit: querit.ai/en/docs + SDK).
// Respons sukses nyata BoCha/Querit belum terbukti live — tes ini mengunci apa yang
// dokumentasi katakan, bukan hasil panggilan sungguhan.

func bodyOf(t *testing.T, c *capture) map[string]interface{} {
	t.Helper()
	var b map[string]interface{}
	if err := json.Unmarshal(c.body, &b); err != nil {
		t.Fatalf("body bukan JSON: %v (%s)", err, c.body)
	}
	return b
}

func TestSearchTavilyRequest(t *testing.T) {
	srv, c := searchServer(t, 200, `{"query":"x","results":[{"title":"T","url":"https://www.kompas.com/a","content":"isi","score":0.9}],"response_time":1.1}`)
	s := newTestSearcher(t, SearcherConfig{Provider: SearchTavily, APIKey: "tvly-KUNCI"}, srv)
	res, err := s.Search(context.Background(), " harga emas ")
	if err != nil || len(res) != 1 || res[0].Domain != "kompas.com" || res[0].Title != "T" {
		t.Fatalf("res=%v err=%v", res, err)
	}
	if c.method != http.MethodPost || c.header.Get("Authorization") != "Bearer tvly-KUNCI" {
		t.Errorf("method/auth salah: %s %q", c.method, c.header.Get("Authorization"))
	}
	b := bodyOf(t, c)
	if b["query"] != "harga emas" || b["max_results"] != float64(maxSearchResults) || b["search_depth"] != "basic" || b["topic"] != "general" {
		t.Errorf("body salah: %s", c.body)
	}
}

func TestSearchBochaRequest(t *testing.T) {
	srv, c := searchServer(t, 200, `{"code":200,"data":{"webPages":{"value":[{"name":"B","url":"https://www.detik.com/x","snippet":"s","summary":"ringkasan panjang","siteName":"detikcom"}]}}}`)
	s := newTestSearcher(t, SearcherConfig{Provider: SearchBocha, APIKey: "sk-KUNCI"}, srv)
	res, err := s.Search(context.Background(), "berita")
	if err != nil || len(res) != 1 || res[0].Domain != "detik.com" || res[0].Snippet != "ringkasan panjang" {
		t.Fatalf("res=%v err=%v (summary harus diutamakan atas snippet)", res, err)
	}
	if c.method != http.MethodPost || c.header.Get("Authorization") != "Bearer sk-KUNCI" {
		t.Errorf("method/auth salah: %s %q", c.method, c.header.Get("Authorization"))
	}
	b := bodyOf(t, c)
	if b["query"] != "berita" || b["summary"] != true || b["freshness"] != "noLimit" || b["count"] != float64(maxSearchResults) {
		t.Errorf("body salah: %s", c.body)
	}
}

func TestSearchBochaTanpaPembungkusData(t *testing.T) {
	res, err := parseSearchResponse(SearchBocha, []byte(`{"webPages":{"value":[{"name":"B","url":"https://a.com","snippet":"s"}]}}`))
	if err != nil || len(res) != 1 {
		t.Fatalf("webPages di level atas harus tetap terbaca: %v %v", res, err)
	}
}

func TestSearchQueritRequest(t *testing.T) {
	srv, c := searchServer(t, 200, `{"took":"12ms","error_code":200,"error_msg":"","search_id":1,"results":{"result":[{"url":"https://www.bbc.com/n","title":"Q","snippet":"isi q","site_name":"BBC","page_age":"2026-11-01"}]}}`)
	s := newTestSearcher(t, SearcherConfig{Provider: SearchQuerit, APIKey: "KUNCI"}, srv)
	res, err := s.Search(context.Background(), "news")
	if err != nil || len(res) != 1 || res[0].Domain != "bbc.com" || res[0].Title != "Q" {
		t.Fatalf("res=%v err=%v", res, err)
	}
	if c.method != http.MethodPost || c.header.Get("Authorization") != "Bearer KUNCI" {
		t.Errorf("method/auth salah: %s %q", c.method, c.header.Get("Authorization"))
	}
	b := bodyOf(t, c)
	if b["query"] != "news" || b["count"] != float64(maxSearchResults) {
		t.Errorf("body salah: %s", c.body)
	}
}

func TestSearchQueritGalatDiBadanDiteruskan(t *testing.T) {
	// Bentuk galat nyata (terukur lewat probe tanpa kunci): error_code string + error_msg.
	for _, body := range []string{
		`{"error_code":"401","error_msg":"Missing API KEY","search_id":4923195912258632478}`,
		`{"error_code":429,"error_msg":"rate limit","search_id":1}`,
	} {
		_, err := parseSearchResponse(SearchQuerit, []byte(body))
		if err == nil || !strings.Contains(err.Error(), "search error 4") || errors.Is(err, ErrSearchEmpty) {
			t.Errorf("%s -> harus galat berkode, bukan kosong: %v", body, err)
		}
	}
}

func TestSearchGalatHTTPPenyediaBaru(t *testing.T) {
	cases := map[string]string{
		SearchTavily: `{"detail":{"error":"Unauthorized: missing or invalid API key."}}`,
		SearchBocha:  `{"code":"401","log_id":"x","message":"Invalid API KEY"}`,
		SearchQuerit: `{"error_code":"401","error_msg":"Invalid authorization header format.","search_id":1}`,
	}
	for prov, body := range cases {
		srv, _ := searchServer(t, 401, body)
		s := newTestSearcher(t, SearcherConfig{Provider: prov, APIKey: "k"}, srv)
		_, err := s.Search(context.Background(), "a")
		if err == nil || !strings.Contains(err.Error(), "search error 401") || errors.Is(err, ErrSearchEmpty) {
			t.Errorf("%s: 401 harus galat berkode: %v", prov, err)
		}
	}
}

func TestPenyediaBaruButuhKunciDanTerdaftar(t *testing.T) {
	for _, p := range []string{SearchTavily, SearchBocha, SearchQuerit} {
		if !IsValidSearchProvider(p) {
			t.Errorf("%s harus terdaftar", p)
		}
		if _, err := NewSearcher(SearcherConfig{Provider: p, APIKey: "  "}); err == nil {
			t.Errorf("%s tanpa kunci harus galat", p)
		}
		if _, err := NewSearcher(SearcherConfig{Provider: p, APIKey: "k"}); err != nil {
			t.Errorf("%s dengan kunci harus jadi: %v", p, err)
		}
		if _, err := parseSearchResponse(p, []byte(`bukan json`)); err == nil {
			t.Errorf("%s: JSON rusak harus galat", p)
		}
		if _, err := parseSearchResponse(p, []byte(`{}`)); !errors.Is(err, ErrSearchEmpty) {
			t.Errorf("%s: {} harus ErrSearchEmpty, dapat %v", p, err)
		}
	}
}
