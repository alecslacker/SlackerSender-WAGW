package service

// Pencarian Z.AI lewat server MCP "web_search_prime" (khusus pemilik GLM Coding Plan:
// kuota dipotong dari kredit paket, bukan saldo API). Protokol: MCP streamable HTTP —
// initialize -> notifications/initialized -> tools/call. Respons berupa SSE
// (`event:message` / `data:{json-rpc}`) atau JSON biasa.
//
// Terbukti live (2026-11-20): endpoint, header Authorization Bearer, sesi lewat
// Mcp-Session-Id, nama tool `web_search_prime`, argumen `search_query`, dan galat
// berbentuk result.isError=true dengan teks "MCP error -429: {...code 1113...}".
// BELUM terbukti: isi respons sukses. Sumber pihak ketiga menyebut teksnya JSON yang
// dienkode dua kali berisi array {title, link, content, refer}; parser di bawah
// toleran terhadap beberapa bentuk, dan bentuk yang tak dikenal menjadi galat
// ("belum bisa dipastikan"), BUKAN "tidak bekerja".
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

const (
	zaiMCPURL  = "https://api.z.ai/api/mcp/web_search_prime/mcp"
	zaiMCPTool = "web_search_prime"
)

// mcpErrPrefix: "MCP error -429: {...}" -> kode 429 + sisa teks.
var mcpErrPrefix = regexp.MustCompile(`^MCP error -?(\d+):\s*(.*)$`)

type rpcMessage struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// rpcPost mengirim satu pesan JSON-RPC; mengembalikan badan (terpotong 1 MB), status, dan
// header respons. Galat jaringan dibungkus tanpa membocorkan kunci.
func (h *httpSearcher) rpcPost(ctx context.Context, session string, payload map[string]interface{}) ([]byte, *http.Response, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.endpoint(zaiMCPURL), bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("gagal menyusun permintaan pencarian")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+h.cfg.APIKey)
	if session != "" {
		req.Header.Set("Mcp-Session-Id", session)
	}
	resp, err := h.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("pencarian gagal dihubungi: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, searchBodyMax))
	return raw, resp, nil
}

// parseRPC mengambil pesan JSON-RPC dari badan SSE ("data:" per baris) atau JSON biasa.
// Bila ada beberapa pesan, dipilih yang membawa result/error.
func parseRPC(raw []byte) (*rpcMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("respons kosong")
	}
	try := func(b []byte) *rpcMessage {
		var m rpcMessage
		if json.Unmarshal(bytes.TrimSpace(b), &m) != nil {
			return nil
		}
		if len(m.Result) == 0 && m.Error == nil {
			return nil
		}
		return &m
	}
	if trimmed[0] == '{' {
		if m := try(trimmed); m != nil {
			return m, nil
		}
	}
	for _, line := range strings.Split(string(trimmed), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		if m := try([]byte(strings.TrimPrefix(line, "data:"))); m != nil {
			return m, nil
		}
	}
	return nil, fmt.Errorf("respons tidak dapat dibaca")
}

// searchZAIMCP menjalankan satu pencarian lewat MCP web_search_prime.
func (h *httpSearcher) searchZAIMCP(ctx context.Context, query string) ([]SearchResult, error) {
	// 1) initialize — sesi (bila ada) dikirim balik lewat header Mcp-Session-Id.
	raw, resp, err := h.rpcPost(ctx, "", map[string]interface{}{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]interface{}{"name": "slackersender", "version": "1"},
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search error %d: %s", resp.StatusCode, shortErr(string(raw)))
	}
	if m, perr := parseRPC(raw); perr == nil && m.Error != nil {
		return nil, fmt.Errorf("search error %d: %s", http.StatusBadRequest, shortErr(m.Error.Message))
	}
	session := resp.Header.Get("Mcp-Session-Id")

	// 2) initialized — status diabaikan (server boleh menjawab 200/202); kegagalan
	// sebenarnya akan tampak di tools/call.
	_, _, _ = h.rpcPost(ctx, session, map[string]interface{}{"jsonrpc": "2.0", "method": "notifications/initialized"})

	// 3) tools/call. location "us" = di luar wilayah Tiongkok (default server "cn").
	raw, resp, err = h.rpcPost(ctx, session, map[string]interface{}{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]interface{}{
			"name":      zaiMCPTool,
			"arguments": map[string]interface{}{"search_query": query, "location": "us"},
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search error %d: %s", resp.StatusCode, shortErr(string(raw)))
	}
	msg, err := parseRPC(raw)
	if err != nil {
		return nil, fmt.Errorf("respons Z.AI MCP tak terbaca: %s", shortErr(string(raw)))
	}
	if msg.Error != nil {
		return nil, fmt.Errorf("Z.AI MCP error %d: %s", msg.Error.Code, shortErr(msg.Error.Message))
	}
	var call struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(msg.Result, &call); err != nil {
		return nil, fmt.Errorf("respons Z.AI MCP tak terbaca: %s", shortErr(string(msg.Result)))
	}
	var text strings.Builder
	for _, c := range call.Content {
		if c.Type == "" || c.Type == "text" {
			text.WriteString(c.Text)
		}
	}
	if call.IsError {
		t := strings.TrimSpace(text.String())
		if m := mcpErrPrefix.FindStringSubmatch(t); m != nil {
			code, _ := strconv.Atoi(m[1])
			return nil, fmt.Errorf("search error %d: %s", code, shortErr(m[2]))
		}
		return nil, fmt.Errorf("Z.AI MCP gagal: %s", shortErr(t))
	}
	res, perr := parseSearchResponse(SearchZAI, []byte(text.String()))
	if perr != nil && !errors.Is(perr, ErrSearchEmpty) {
		// Sertakan potongan isi agar bentuk tak terduga bisa didiagnosis dari UI.
		return nil, fmt.Errorf("%v: %s", perr, shortErr(text.String()))
	}
	return res, perr
}
