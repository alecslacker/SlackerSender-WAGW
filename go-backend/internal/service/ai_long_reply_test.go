package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAIReplyTokensFor(t *testing.T) {
	cases := []struct{ chars, want int }{
		{0, 2048},                        // tak pernah di bawah batas lama
		{300, 300/3 + 2048},              // 2148
		{1000, 1000/3 + 2048},            // 2381
		{4000, 4000/3 + 2048},            // 3381
		{16000, 16000/3 + 2048},          // 7381
		{99999, AIReplyMaxTokensCeiling}, // dibatasi langit-langit
	}
	for _, c := range cases {
		if got := AIReplyTokensFor(c.chars); got != c.want {
			t.Errorf("AIReplyTokensFor(%d)=%d, mau %d", c.chars, got, c.want)
		}
		if got := AIReplyTokensFor(c.chars); got < AIReplyMaxTokens || got > AIReplyMaxTokensCeiling {
			t.Errorf("AIReplyTokensFor(%d)=%d di luar [%d,%d]", c.chars, got, AIReplyMaxTokens, AIReplyMaxTokensCeiling)
		}
	}
	// Batas karakter maksimum harus muat di anggaran tanpa terpotong langit-langit.
	if AIReplyTokensFor(MaxReplyLengthCap) >= AIReplyMaxTokensCeiling {
		t.Errorf("cap %d karakter tak boleh menyentuh langit-langit token", MaxReplyLengthCap)
	}
}

func TestMaxReplyLengthCap16000(t *testing.T) {
	long := make([]rune, 20000)
	for i := range long {
		long[i] = 'a'
	}
	if got := len([]rune(TruncateReply(string(long), 99999))); got != MaxReplyLengthCap+1 { // +1 = "…"
		t.Errorf("hasil %d rune, mau %d", got, MaxReplyLengthCap+1)
	}
	if got := len([]rune(TruncateReply(string(long[:15000]), 16000))); got != 15000 {
		t.Errorf("15000 karakter harus utuh, dapat %d", got)
	}
}

// Provider menolak max_tokens besar (400) -> ulangi sekali dengan 2048, balasan tetap sampai.
func TestCompleteTolerantRetryMaxTokensBesarBila400(t *testing.T) {
	var got []float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var m map[string]interface{}
		_ = json.Unmarshal(b, &m)
		mt, _ := m["max_tokens"].(float64)
		got = append(got, mt)
		if mt > 4096 {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":"max_tokens too large"}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"jawaban"}}]}`))
	}))
	defer srv.Close()
	s := &AIService{Provider: ProviderZAI, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	resp, err := s.CompleteTolerant(ChatRequest{
		Provider: ProviderZAI, Model: "m", MaxTokens: 7381,
		Messages: []ChatMessage{{Role: "user", Content: "halo"}},
	})
	if err != nil || resp.Content != "jawaban" {
		t.Fatalf("harus pulih: resp=%+v err=%v", resp, err)
	}
	if len(got) != 2 || got[0] != 7381 || got[1] != float64(AIReplyMaxTokens) {
		t.Errorf("urutan max_tokens salah: %v", got)
	}
	if !resp.Downgraded {
		t.Error("harus ditandai Downgraded agar tercatat di log")
	}
}

func TestCompleteTolerantTidakRetryMaxTokensBilaSudahKecil(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	defer srv.Close()
	s := &AIService{Provider: ProviderZAI, APIKey: "k", BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, _ = s.CompleteTolerant(ChatRequest{
		Provider: ProviderZAI, Model: "m", MaxTokens: AIReplyMaxTokens,
		Messages: []ChatMessage{{Role: "user", Content: "halo"}},
	})
	if calls != 1 {
		t.Errorf("batas sudah 2048: tak ada alasan retry, calls=%d", calls)
	}
}
