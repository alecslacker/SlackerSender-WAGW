package service

// Tiga implementasi Searcher. Semua memakai klien anti-SSRF (aiHTTPClient) dan
// memotong badan respons (batas baca 1 MB) agar respons raksasa tak membebani memori.
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	braveSearchURL = "https://api.search.brave.com/res/v1/web/search"
	searchBodyMax  = 1 << 20 // 1 MB
)

// SearcherConfig: konfigurasi satu penyedia. BaseURL hanya untuk SearXNG; untuk
// Z.AI/Brave URL tetap (kecuali diganti tes lewat endpointOverride).
type SearcherConfig struct {
	Provider string
	APIKey   string
	BaseURL  string
	// shortcut: endpointOverride hanya untuk tes (server tiruan); produksi memakai
	// konstanta di atas. Pindahkan ke konfigurasi bila perlu proxy khusus.
	endpointOverride string
	HTTPClient       *http.Client
}

// NewSearcher membuat Searcher sesuai penyedia; galat bila konfigurasi tak lengkap.
func NewSearcher(cfg SearcherConfig) (Searcher, error) {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = aiHTTPClient()
	}
	switch cfg.Provider {
	case SearchZAI, SearchBrave:
		if strings.TrimSpace(cfg.APIKey) == "" {
			return nil, fmt.Errorf("API key penyedia pencarian belum diisi")
		}
	case SearchSearXNG:
		if strings.TrimSpace(cfg.BaseURL) == "" {
			return nil, fmt.Errorf("URL instance SearXNG belum diisi")
		}
	default:
		return nil, fmt.Errorf("penyedia pencarian tidak dikenal: %q", cfg.Provider)
	}
	return &httpSearcher{cfg: cfg}, nil
}

type httpSearcher struct{ cfg SearcherConfig }

func (h *httpSearcher) Search(ctx context.Context, query string) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("kueri pencarian kosong")
	}
	if r := []rune(query); len(r) > 300 {
		query = string(r[:300])
	}
	var (
		req *http.Request
		err error
	)
	switch h.cfg.Provider {
	case SearchZAI:
		// Z.AI lewat server MCP web_search_prime (jalur GLM Coding Plan, kuota paket).
		// REST /paas/v4/web_search sengaja tidak dipakai: ia memotong saldo API biasa.
		return h.searchZAIMCP(ctx, query)
	case SearchBrave:
		q := url.Values{"q": {query}, "count": {fmt.Sprint(maxSearchResults)}}
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, h.endpoint(braveSearchURL)+"?"+q.Encode(), nil)
		if err == nil {
			req.Header.Set("X-Subscription-Token", h.cfg.APIKey)
			req.Header.Set("Accept", "application/json")
		}
	case SearchSearXNG:
		base := strings.TrimRight(h.cfg.BaseURL, "/")
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, base+"/search?"+url.Values{"q": {query}, "format": {"json"}}.Encode(), nil)
		if err == nil {
			req.Header.Set("Accept", "application/json")
		}
	}
	if err != nil {
		return nil, fmt.Errorf("gagal menyusun permintaan pencarian")
	}
	resp, err := h.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pencarian gagal dihubungi: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, searchBodyMax))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search error %d: %s", resp.StatusCode, shortErr(string(raw)))
	}
	return parseSearchResponse(h.cfg.Provider, raw)
}

func (h *httpSearcher) endpoint(def string) string {
	if h.cfg.endpointOverride != "" {
		return h.cfg.endpointOverride
	}
	return def
}

// parseSearchResponse menormalkan respons tiap penyedia. Toleran: bentuk tak
// terduga menghasilkan galat yang jelas, bukan panik.
func parseSearchResponse(provider string, raw []byte) ([]SearchResult, error) {
	var out []SearchResult
	switch provider {
	case SearchZAI:
		items, err := zaiItems(raw)
		if err != nil {
			return nil, err
		}
		for _, x := range items {
			link := x.Link
			if link == "" {
				link = x.URL
			}
			s := x.Snippet
			if s == "" {
				s = x.Content
			}
			if s == "" {
				s = x.Summary
			}
			if n, ok := normalizeResult(x.Title, link, s, x.Media); ok {
				out = append(out, n)
			}
		}
	case SearchBrave:
		var r struct {
			Web struct {
				Results []struct{ Title, URL, Description string } `json:"results"`
			} `json:"web"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("respons Brave tidak dapat dibaca")
		}
		for _, x := range r.Web.Results {
			if n, ok := normalizeResult(x.Title, x.URL, x.Description, ""); ok {
				out = append(out, n)
			}
		}
	case SearchSearXNG:
		var r struct {
			Results []struct{ Title, URL, Content string } `json:"results"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("respons SearXNG tidak dapat dibaca (format JSON mungkin belum dinyalakan di instance)")
		}
		for _, x := range r.Results {
			if n, ok := normalizeResult(x.Title, x.URL, x.Content, ""); ok {
				out = append(out, n)
			}
		}
	default:
		return nil, fmt.Errorf("penyedia pencarian tidak dikenal")
	}
	out = capResults(out)
	if len(out) == 0 {
		return nil, ErrSearchEmpty
	}
	return out, nil
}

type zaiItem struct {
	Title, Link, URL, Snippet, Content, Summary, Media string
}

// zaiItems membaca hasil Z.AI dalam bentuk yang diketahui/dilaporkan: array objek,
// objek berisi search_result/results, dan teks MCP yang dienkode JSON dua kali
// (string yang isinya JSON). Bentuk lain -> galat jelas (bukan "kosong").
func zaiItems(raw []byte) ([]zaiItem, error) {
	b := bytes.TrimSpace(raw)
	for depth := 0; depth < 3 && len(b) > 0; depth++ {
		switch b[0] {
		case '"': // JSON string yang membungkus JSON lain
			var s string
			if err := json.Unmarshal(b, &s); err != nil {
				return nil, fmt.Errorf("respons Z.AI tidak dapat dibaca")
			}
			b = bytes.TrimSpace([]byte(s))
		case '[':
			var items []zaiItem
			if err := json.Unmarshal(b, &items); err != nil {
				return nil, fmt.Errorf("respons Z.AI tidak dapat dibaca")
			}
			return items, nil
		case '{':
			var obj struct {
				SearchResult []zaiItem `json:"search_result"`
				Results      []zaiItem `json:"results"`
			}
			if err := json.Unmarshal(b, &obj); err != nil {
				return nil, fmt.Errorf("respons Z.AI tidak dapat dibaca")
			}
			if len(obj.SearchResult) > 0 {
				return obj.SearchResult, nil
			}
			return obj.Results, nil
		default:
			return nil, fmt.Errorf("respons Z.AI tidak dapat dibaca (bukan JSON)")
		}
	}
	return nil, fmt.Errorf("respons Z.AI tidak dapat dibaca")
}
