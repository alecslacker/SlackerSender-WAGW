package service

// "Cari di Web" untuk chat AI — pencarian BAWAAN provider (tanpa MCP, tanpa
// loop tool di server kita): request cukup membawa satu tool bawaan, provider
// yang mencari lalu menyusun jawaban.
//
// Provider yang didukung (dokumentasi resmi, 2026-11-20):
//   - Z.AI:      tools=[{type:"web_search", web_search:{enable,search_result}}];
//                hasil di field level-atas "web_search" (title/link/media).
//   - Anthropic: tools=[{type:"web_search_20250305", name:"web_search", max_uses}];
//                jawaban terpecah ke BANYAK blok "text" (sebagian membawa
//                citations), diawali blok server_tool_use/web_search_tool_result.
// Gemini SENGAJA tidak didukung: grounding Google Search mewajibkan tampilan
// "Search Suggestions" (HTML) yang tak bisa ditampilkan di WhatsApp.
// 9router/custom/openai/deepseek tidak didukung: proxy belum tentu meneruskan.
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

// AnthropicWebSearchMaxUses: batas pencarian per balasan (pengendali biaya).
const AnthropicWebSearchMaxUses = 2

// maxSources: jumlah sumber yang disimpan untuk log.
const maxSources = 5

// WebSearchSupported melaporkan apakah provider punya pencarian web bawaan yang
// kita dukung. Dipakai UI (toggle nonaktif) dan service (field diabaikan).
func WebSearchSupported(p AIProvider) bool {
	return p == ProviderZAI || p == ProviderAnthropic
}

// zaiWebSearchTool: bentuk minimal sesuai referensi API Z.AI. search_engine
// sengaja tidak dikirim (dokumentasi tidak konsisten) — pakai bawaan provider.
func zaiWebSearchTool() []map[string]interface{} {
	return []map[string]interface{}{{
		"type": "web_search",
		"web_search": map[string]interface{}{
			"enable":        true,
			"search_result": true,
		},
	}}
}

func anthropicWebSearchTool() []map[string]interface{} {
	return []map[string]interface{}{{
		"type":     "web_search_20250305",
		"name":     "web_search",
		"max_uses": AnthropicWebSearchMaxUses,
	}}
}

// badRequestRe: pesan error tiap jalur berbentuk "<Nama> error 400: <body>".
// Dijangkar di awal agar body error lain yang kebetulan memuat tulisan
// "error 400" (mis. 429 dengan kutipan) tidak ikut cocok.
var badRequestRe = regexp.MustCompile(`^[A-Za-z]+ error 400:`)

// isBadRequest: provider menolak request (HTTP 400) — dipakai jaring pengaman
// CompleteTolerant.
func isBadRequest(err error) bool {
	return err != nil && badRequestRe.MatchString(err.Error())
}

// maxSourceLabel membatasi label sumber (hasil web = input tak tepercaya yang
// masuk log).
const maxSourceLabel = 60

// cleanLabel membuang karakter kontrol (mis. ESC) dan memotong label.
func cleanLabel(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	if r := []rune(s); len(r) > maxSourceLabel {
		s = string(r[:maxSourceLabel])
	}
	return s
}

// addSource menambah label sumber unik (nama situs > domain > judul).
func addSource(list []string, media, link, title string) []string {
	label := cleanLabel(media)
	if label == "" && link != "" {
		if u, err := url.Parse(link); err == nil {
			label = cleanLabel(strings.TrimPrefix(u.Hostname(), "www."))
		}
	}
	if label == "" {
		label = cleanLabel(title)
	}
	if label == "" || len(list) >= maxSources {
		return list
	}
	for _, s := range list {
		if s == label {
			return list
		}
	}
	return append(list, label)
}

// anthropicResult adalah bentuk respons Messages API yang kita baca.
type anthropicResult struct {
	Content []struct {
		Type      string `json:"type"`
		Text      string `json:"text"`
		Citations []struct {
			URL   string `json:"url"`
			Title string `json:"title"`
		} `json:"citations"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// parseAnthropicResponse menggabungkan SEMUA blok teks. Tanpa pencarian,
// respons berisi satu blok teks → hasil identik dengan membaca content[0].
// Dengan pencarian, blok pertama adalah server_tool_use (tanpa teks), jadi
// membaca content[0] saja akan menghasilkan jawaban kosong.
func parseAnthropicResponse(body []byte) (*ChatResponse, error) {
	var result anthropicResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic response: %w", err)
	}
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("Anthropic returned no content")
	}
	var sb strings.Builder
	out := &ChatResponse{
		Tokens:       result.Usage.InputTokens + result.Usage.OutputTokens,
		FinishReason: result.StopReason,
	}
	for _, b := range result.Content {
		switch b.Type {
		case "text", "":
			// Blok teks bersitasi adalah potongan kalimat yang bersambung —
			// digabung tanpa pemisah.
			sb.WriteString(b.Text)
			for _, c := range b.Citations {
				out.Sources = addSource(out.Sources, "", c.URL, c.Title)
			}
		case "server_tool_use", "web_search_tool_result":
			out.Searched = true
			// Teks SEBELUM/di antara pencarian hanyalah narasi model ("Saya cari
			// dulu…") — jangan ikut terkirim ke pelanggan; hanya jawaban akhir
			// (teks setelah hasil pencarian terakhir) yang dipakai.
			sb.Reset()
		}
	}
	out.Content = sb.String()
	// pause_turn: model berhenti di tengah giliran panjang tanpa jawaban. Kita
	// tidak melanjutkan putaran (YAGNI) — jadikan error agar CompleteTolerant
	// mengulang tanpa pencarian dan pelanggan tetap dibalas.
	if out.FinishReason == "pause_turn" && strings.TrimSpace(out.Content) == "" {
		return nil, fmt.Errorf("Anthropic error 400: pause_turn tanpa jawaban (pencarian terlalu panjang)")
	}
	return out, nil
}
