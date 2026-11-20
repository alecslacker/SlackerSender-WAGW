package service

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNormalizeResult(t *testing.T) {
	r, ok := normalizeResult("  Judul\x1b[31m  Panjang \n baris ", "https://www.Kompas.com/x?a=1", "Ringkasan\x00 isi", "kompas")
	if !ok {
		t.Fatal("harus layak")
	}
	if r.Title != "Judul[31m Panjang baris" || strings.ContainsRune(r.Title, 0x1b) {
		t.Errorf("judul tak dibersihkan: %q", r.Title)
	}
	if r.Domain != "kompas.com" {
		t.Errorf("domain tanpa www & huruf kecil: %q", r.Domain)
	}
	if r.Snippet != "Ringkasan isi" {
		t.Errorf("ringkasan: %q", r.Snippet)
	}
	// tanpa URL valid -> pakai media
	if r2, _ := normalizeResult("t", "javascript:alert(1)", "s", "Detik"); r2.Domain != "Detik" {
		t.Errorf("URL non-http harus ditolak, pakai media: %q", r2.Domain)
	}
	// tanpa judul dan ringkasan -> dibuang
	if _, ok := normalizeResult("  ", "https://a.com", " ", ""); ok {
		t.Error("tanpa isi harus dibuang")
	}
	// pemotongan
	long := strings.Repeat("x", 1000)
	if r3, _ := normalizeResult("t", "https://a.com", long, ""); len([]rune(r3.Snippet)) != maxSnippetRunes+1 {
		t.Errorf("ringkasan harus dipotong + elipsis: %d", len([]rune(r3.Snippet)))
	}
}

func TestCapResultsDedupDanBatas(t *testing.T) {
	var in []SearchResult
	for i := 0; i < 12; i++ {
		in = append(in, SearchResult{Title: string(rune('a' + i)), Domain: "d.com"})
	}
	in = append(in, SearchResult{Title: "A", Domain: "d.com"}) // duplikat (huruf besar)
	out := capResults(in)
	if len(out) != maxSearchResults {
		t.Fatalf("harus %d, dapat %d", maxSearchResults, len(out))
	}
	dup := capResults([]SearchResult{{Title: "X", Domain: "a"}, {Title: "x", Domain: "a"}, {Title: "x", Domain: "b"}})
	if len(dup) != 2 {
		t.Errorf("duplikat (judul+domain sama) harus digabung: %d", len(dup))
	}
}

func TestParseSearchResponsePerPenyedia(t *testing.T) {
	cases := []struct {
		nama, prov, body string
		domain, title    string
	}{
		{"Z.AI REST", SearchZAI,
			`{"search_result":[{"title":"Z","link":"https://www.detik.com/a","content":"ringkas z","media":"detikcom"}]}`,
			"detik.com", "Z"},
		{"Z.AI snippet", SearchZAI,
			`{"search_result":[{"title":"Z","link":"https://a.com","snippet":"s"}]}`, "a.com", "Z"},
		{"Z.AI MCP array", SearchZAI,
			`[{"title":"M","link":"https://www.kompas.com/x","content":"isi m","refer":"ref_1"}]`, "kompas.com", "M"},
		{"Z.AI MCP url+summary", SearchZAI,
			`[{"title":"M","url":"https://b.com","summary":"ringkas"}]`, "b.com", "M"},
		{"Z.AI MCP dienkode dua kali", SearchZAI,
			`"[{\"title\":\"D\",\"link\":\"https://c.com\",\"content\":\"dua kali\"}]"`, "c.com", "D"},
		{"Z.AI MCP objek results", SearchZAI,
			`{"results":[{"title":"R","link":"https://d.com","content":"x"}]}`, "d.com", "R"},
		{"Brave", SearchBrave,
			`{"web":{"results":[{"title":"B","url":"https://www.bbc.com/x","description":"desk b"}]}}`,
			"bbc.com", "B"},
		{"SearXNG", SearchSearXNG,
			`{"results":[{"title":"S","url":"https://www.wikipedia.org/w","content":"isi s"}]}`,
			"wikipedia.org", "S"},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			res, err := parseSearchResponse(c.prov, []byte(c.body))
			if err != nil || len(res) != 1 {
				t.Fatalf("res=%v err=%v", res, err)
			}
			// membuktikan pemetaan field tanpa tag (Title/Link/URL/Content) benar
			if res[0].Domain != c.domain || res[0].Title != c.title || res[0].Snippet == "" {
				t.Errorf("salah baca: %+v", res[0])
			}
		})
	}
}

func TestParseZAIBentukAsingBukanKosong(t *testing.T) {
	// Teks bebas / bukan JSON harus galat "tak terbaca", BUKAN ErrSearchEmpty
	// (kosong = vonis "tidak bekerja"; tak terbaca = belum bisa dipastikan).
	for _, body := range []string{`Berikut hasilnya: ...`, `12345`, `"bukan json di dalam"`} {
		_, err := parseSearchResponse(SearchZAI, []byte(body))
		if err == nil || errors.Is(err, ErrSearchEmpty) {
			t.Errorf("%q: harus galat tak terbaca, dapat %v", body, err)
		}
	}
}

func TestParseSearchResponseBentukBuruk(t *testing.T) {
	for _, prov := range ValidSearchProviders {
		if _, err := parseSearchResponse(prov, []byte(`bukan json`)); err == nil {
			t.Errorf("%s: JSON rusak harus error", prov)
		}
		if _, err := parseSearchResponse(prov, []byte(`{}`)); !errors.Is(err, ErrSearchEmpty) {
			t.Errorf("%s: tanpa hasil harus ErrSearchEmpty, dapat %v", prov, err)
		}
	}
	if _, err := parseSearchResponse("asing", []byte(`{}`)); err == nil {
		t.Error("penyedia asing harus error")
	}
}

func TestIsValidSearchProvider(t *testing.T) {
	for _, p := range []string{"zai", "brave", "searxng"} {
		if !IsValidSearchProvider(p) {
			t.Errorf("%s harus valid", p)
		}
	}
	for _, p := range []string{"", "google", "ZAI", "bing"} {
		if IsValidSearchProvider(p) {
			t.Errorf("%q tak boleh valid", p)
		}
	}
}

func TestSearchGuardCacheDanTTL(t *testing.T) {
	g := NewSearchGuard(10*time.Minute, 60)
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return now }
	res := []SearchResult{{Title: "A"}}
	g.Put(5, "brave", "Berita  Hari Ini", res)
	if got, ok := g.Get(5, "brave", "berita hari ini"); !ok || got[0].Title != "A" {
		t.Error("kunci cache harus tak peka huruf & spasi ganda")
	}
	if _, ok := g.Get(6, "brave", "berita hari ini"); ok {
		t.Error("cache TIDAK boleh bocor antar akun")
	}
	if _, ok := g.Get(5, "zai", "berita hari ini"); ok {
		t.Error("cache TIDAK boleh bocor antar penyedia")
	}
	now = now.Add(11 * time.Minute)
	if _, ok := g.Get(5, "brave", "berita hari ini"); ok {
		t.Error("harus kedaluwarsa setelah TTL")
	}
}

func TestSearchGuardBatasPerJam(t *testing.T) {
	g := NewSearchGuard(time.Minute, 3)
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if !g.Allow(5) {
			t.Fatalf("pencarian ke-%d harus lolos", i+1)
		}
	}
	if g.Allow(5) {
		t.Error("ke-4 harus ditolak")
	}
	if !g.Allow(6) {
		t.Error("batas per AKUN, akun lain tak terpengaruh")
	}
	now = now.Add(61 * time.Minute)
	if !g.Allow(5) {
		t.Error("setelah satu jam harus pulih")
	}
}
