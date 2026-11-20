package service

// Pencarian web yang dijalankan SERVER kita lewat penyedia terpisah (bukan fitur
// bawaan model), sehingga tidak terikat model/provider AI: hasilnya (judul, domain,
// ringkasan) disisipkan ke prompt sebagai DATA. Tiga penyedia di balik satu
// antarmuka: Z.AI Web Search, Brave Search, SearXNG (instance milik sendiri).
//
// Yang BELUM terbukti (dokumentasi tak konsisten) ditandai "shortcut:" dan wajib
// diuji nyata lewat tombol "Cek Pencarian": jalur endpoint & bentuk respons Z.AI,
// batas gratis Brave, perilaku limiter SearXNG.
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Penyedia pencarian yang dikenali.
const (
	SearchZAI     = "zai"
	SearchBrave   = "brave"
	SearchSearXNG = "searxng"
)

// ValidSearchProviders: urutan tampil di UI.
var ValidSearchProviders = []string{SearchZAI, SearchBrave, SearchSearXNG}

// IsValidSearchProvider memeriksa nama penyedia.
func IsValidSearchProvider(p string) bool {
	for _, v := range ValidSearchProviders {
		if p == v {
			return true
		}
	}
	return false
}

// SearchResult: satu hasil, sudah dinormalkan dan dibersihkan.
type SearchResult struct {
	Title   string
	Domain  string // tanpa "www."; dipakai sebagai label sumber
	Snippet string // dipotong; DATA tak tepercaya
}

const (
	// maxSearchResults: hasil yang disisipkan ke prompt.
	maxSearchResults = 5
	maxSnippetRunes  = 280
	maxTitleRunes    = 100
)

// cleanText membuang karakter kontrol, merapatkan spasi, dan memotong.
func cleanText(s string, max int) string {
	s = cleanLabelRunes(strings.Join(strings.Fields(s), " "))
	if r := []rune(s); len(r) > max {
		return strings.TrimSpace(string(r[:max])) + "…"
	}
	return s
}

// cleanLabelRunes membuang karakter kontrol (ESC, NUL, dsb.).
func cleanLabelRunes(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)
}

// domainOf mengambil host tanpa "www." dari URL; kosong bila tak valid/bukan http(s).
func domainOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// normalizeResult menyusun SearchResult; ok=false bila tak layak dipakai
// (tanpa judul dan tanpa ringkasan, atau URL bukan http/https).
func normalizeResult(title, link, snippet, media string) (SearchResult, bool) {
	r := SearchResult{
		Title:   cleanText(title, maxTitleRunes),
		Snippet: cleanText(snippet, maxSnippetRunes),
		Domain:  domainOf(link),
	}
	if r.Domain == "" {
		r.Domain = cleanText(media, 60)
	}
	if r.Title == "" && r.Snippet == "" {
		return SearchResult{}, false
	}
	return r, true
}

// capResults membatasi jumlah dan membuang duplikat (judul+domain sama).
func capResults(in []SearchResult) []SearchResult {
	seen := map[string]bool{}
	out := make([]SearchResult, 0, maxSearchResults)
	for _, r := range in {
		k := r.Domain + "|" + strings.ToLower(r.Title)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, r)
		if len(out) == maxSearchResults {
			break
		}
	}
	return out
}

// Searcher: satu penyedia pencarian yang sudah terkonfigurasi.
type Searcher interface {
	Search(ctx context.Context, query string) ([]SearchResult, error)
}

// ErrSearchEmpty: penyedia sehat tetapi tak ada hasil (bukan galat jaringan).
var ErrSearchEmpty = fmt.Errorf("pencarian tidak menghasilkan apa pun")

// ── Cache & batas per akun ────────────────────────────────────────────────

// SearchGuard: cache hasil (TTL) + batas pencarian per jam per akun, di memori.
// shortcut: di memori satu proses, hilang saat restart & tak dibagi antar server;
// cukup untuk satu instance, naik ke Redis bila skala-out.
type SearchGuard struct {
	mu       sync.Mutex
	ttl      time.Duration
	perHour  int
	now      func() time.Time
	cache    map[string]cacheEntry
	counters map[uint][]time.Time
}

type cacheEntry struct {
	at      time.Time
	results []SearchResult
}

// NewSearchGuard: ttl cache dan batas pencarian per jam per akun.
func NewSearchGuard(ttl time.Duration, perHour int) *SearchGuard {
	return &SearchGuard{ttl: ttl, perHour: perHour, now: time.Now,
		cache: map[string]cacheEntry{}, counters: map[uint][]time.Time{}}
}

// DefaultSearchGuard dipakai pipeline: cache 10 menit, 60 pencarian/jam/akun.
var DefaultSearchGuard = NewSearchGuard(10*time.Minute, 60)

func cacheKey(userID uint, provider, query string) string {
	return fmt.Sprintf("%d|%s|%s", userID, provider, strings.ToLower(strings.Join(strings.Fields(query), " ")))
}

// Get mengembalikan hasil cache yang masih segar.
func (g *SearchGuard) Get(userID uint, provider, query string) ([]SearchResult, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.cache[cacheKey(userID, provider, query)]
	if !ok || g.now().Sub(e.at) > g.ttl {
		return nil, false
	}
	return e.results, true
}

// Put menyimpan hasil (dan membuang entri kedaluwarsa agar memori tak membengkak).
func (g *SearchGuard) Put(userID uint, provider, query string, res []SearchResult) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	for k, e := range g.cache {
		if now.Sub(e.at) > g.ttl {
			delete(g.cache, k)
		}
	}
	g.cache[cacheKey(userID, provider, query)] = cacheEntry{at: now, results: res}
}

// Allow mencatat satu pencarian baru; false bila batas per jam akun terlampaui.
func (g *SearchGuard) Allow(userID uint) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	keep := g.counters[userID][:0]
	for _, t := range g.counters[userID] {
		if now.Sub(t) < time.Hour {
			keep = append(keep, t)
		}
	}
	if len(keep) >= g.perHour {
		g.counters[userID] = keep
		return false
	}
	g.counters[userID] = append(keep, now)
	return true
}
