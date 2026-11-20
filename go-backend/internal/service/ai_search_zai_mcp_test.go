package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// mcpFake meniru server MCP Z.AI sesuai perilaku yang teramati live: respons SSE,
// sesi lewat header Mcp-Session-Id, notifications/initialized dijawab 202.
type mcpFake struct {
	mu       sync.Mutex
	methods  []string
	sessions []string
	auth     []string
	callArgs map[string]interface{}
	callName string
	callText string // isi result.content[0].text
	isError  bool
}

func (f *mcpFake) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var m struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		} `json:"params"`
	}
	_ = json.Unmarshal(body, &m)
	f.mu.Lock()
	f.methods = append(f.methods, m.Method)
	f.sessions = append(f.sessions, r.Header.Get("Mcp-Session-Id"))
	f.auth = append(f.auth, r.Header.Get("Authorization"))
	f.mu.Unlock()
	switch m.Method {
	case "initialize":
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Mcp-Session-Id", "sesi-123")
		_, _ = w.Write([]byte("id:1\nevent:message\ndata:{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2024-11-05\"}}\n\n"))
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
	case "tools/call":
		f.mu.Lock()
		f.callName, f.callArgs = m.Params.Name, m.Params.Arguments
		f.mu.Unlock()
		res, _ := json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0", "id": 2,
			"result": map[string]interface{}{
				"content": []map[string]string{{"type": "text", "text": f.callText}},
				"isError": f.isError,
			},
		})
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("id:1\nevent:message\ndata:" + string(res) + "\n\n"))
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func newMCPSearcher(t *testing.T, f *mcpFake) Searcher {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(srv.Close)
	return newTestSearcher(t, SearcherConfig{Provider: SearchZAI, APIKey: "KUNCI"}, srv)
}

func TestSearchZAIMCPAlurLengkap(t *testing.T) {
	f := &mcpFake{callText: `"[{\"title\":\"T\",\"link\":\"https://a.com\",\"content\":\"isi\",\"refer\":\"ref_1\"}]"`}
	s := newMCPSearcher(t, f)
	res, err := s.Search(context.Background(), "  harga emas hari ini ")
	if err != nil || len(res) != 1 || res[0].Domain != "a.com" {
		t.Fatalf("res=%v err=%v", res, err)
	}
	if got := strings.Join(f.methods, ","); got != "initialize,notifications/initialized,tools/call" {
		t.Errorf("urutan MCP salah: %s", got)
	}
	for i, a := range f.auth {
		if a != "Bearer KUNCI" {
			t.Errorf("pesan %d tanpa Authorization benar: %q", i, a)
		}
	}
	if f.sessions[0] != "" || f.sessions[1] != "sesi-123" || f.sessions[2] != "sesi-123" {
		t.Errorf("sesi harus dikirim balik setelah initialize: %v", f.sessions)
	}
	if f.callName != "web_search_prime" || f.callArgs["search_query"] != "harga emas hari ini" {
		t.Errorf("tools/call salah: %s %v", f.callName, f.callArgs)
	}
	if f.callArgs["location"] != "us" {
		t.Errorf("location harus us (default server cn): %v", f.callArgs)
	}
}

func TestSearchZAIMCPGalatSaldoDiteruskanDenganKode(t *testing.T) {
	// Persis teks yang teramati live untuk kunci tanpa paket.
	f := &mcpFake{isError: true,
		callText: `MCP error -429: {"error":{"code":"1113","message":"Insufficient balance or no resource package. Please recharge."}}`}
	s := newMCPSearcher(t, f)
	_, err := s.Search(context.Background(), "tes")
	if err == nil || !strings.Contains(err.Error(), "search error 429") || !strings.Contains(err.Error(), "1113") {
		t.Fatalf("galat harus memuat kode 429 dan 1113: %v", err)
	}
	if errors.Is(err, ErrSearchEmpty) {
		t.Error("galat saldo BUKAN 'penyedia menjawab tanpa hasil'")
	}
}

func TestSearchZAIMCPTeksTakTerbacaBukanKosong(t *testing.T) {
	f := &mcpFake{callText: `Berikut hasil pencarian: ... (teks bebas)`}
	s := newMCPSearcher(t, f)
	_, err := s.Search(context.Background(), "tes")
	if err == nil || errors.Is(err, ErrSearchEmpty) {
		t.Fatalf("teks tak terbaca harus galat biasa (belum bisa dipastikan): %v", err)
	}
	if !strings.Contains(err.Error(), "teks bebas") {
		t.Errorf("potongan isi harus ikut agar bisa didiagnosis: %v", err)
	}
}

func TestSearchZAIMCPKosongAdalahErrSearchEmpty(t *testing.T) {
	f := &mcpFake{callText: `[]`}
	s := newMCPSearcher(t, f)
	if _, err := s.Search(context.Background(), "tes"); !errors.Is(err, ErrSearchEmpty) {
		t.Fatalf("array kosong harus ErrSearchEmpty: %v", err)
	}
}

func TestParseRPCSSEdanJSONBiasa(t *testing.T) {
	sse := "id:1\nevent:message\ndata:{\"jsonrpc\":\"2.0\",\"id\":3,\"result\":{\"ok\":true}}\n\n"
	if m, err := parseRPC([]byte(sse)); err != nil || len(m.Result) == 0 {
		t.Errorf("SSE: %v %v", m, err)
	}
	if m, err := parseRPC([]byte(`{"jsonrpc":"2.0","id":3,"result":{"ok":true}}`)); err != nil || len(m.Result) == 0 {
		t.Errorf("JSON biasa: %v %v", m, err)
	}
	if _, err := parseRPC([]byte(`{"code":1001,"msg":"Authentication parameter not received","success":false}`)); err == nil {
		t.Error("galat otentikasi non-JSON-RPC harus galat")
	}
	if _, err := parseRPC([]byte("")); err == nil {
		t.Error("kosong harus galat")
	}
}
