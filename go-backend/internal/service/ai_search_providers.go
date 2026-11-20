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
	zaiSearchURL   = "https://api.z.ai/api/paas/v4/web_search"
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
		// shortcut: jalur "/web_search" + bentuk respons search_result diambil dari
		// SDK resmi; dokumen API menyebut dua nama mesin (search-prime vs
		// search_pro_jina). Kita TIDAK mengirim search_engine bila kosong — wajib
		// diuji nyata lewat "Cek Pencarian"; ganti bila Z.AI menolak.
		body, _ := json.Marshal(map[string]interface{}{"search_query": query, "count": maxSearchResults})
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, h.endpoint(zaiSearchURL), bytes.NewReader(body))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+h.cfg.APIKey)
		}
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
		var r struct {
			Results []struct {
				Title, Link, Snippet, Content, Media string
			} `json:"search_result"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("respons Z.AI tidak dapat dibaca")
		}
		for _, x := range r.Results {
			s := x.Snippet
			if s == "" {
				s = x.Content
			}
			if n, ok := normalizeResult(x.Title, x.Link, s, x.Media); ok {
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
