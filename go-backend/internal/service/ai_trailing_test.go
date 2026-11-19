package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Bentuk respons 9router yang membuat "Test gagal: failed to parse AI response:
// invalid character 'd' after top-level value": satu objek chat.completion
// utuh, lalu ekor SSE ("data: [DONE]") di belakangnya.
const nineRouterJSON = `{"id":"chatcmpl-cb1e4bdf-bf20-4ad2-8cf2-7e3f5cbc5c6a","object":"chat.completion","created":1795077050,"model":"claude-sonnet-4","choices":[{"index":0,"message":{"role":"assistant","content":"Halo, ada yang bisa dibantu?"},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":7,"total_tokens":15}}`

func TestPostChatCompletionsToleransiEkorSetelahJSON(t *testing.T) {
	ekor := map[string]string{
		"tanpa ekor":           "",
		"ekor [DONE] SSE":      "\n\ndata: [DONE]\n\n",
		"ekor [DONE] rapat":    "data: [DONE]",
		"ekor baris kosong":    "\n\n\n",
		"ekor teks sembarang":  "\ndone",
		"ekor event SSE":       "\nevent: done\ndata: {}\n\n",
	}
	for nama, e := range ekor {
		t.Run(nama, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(nineRouterJSON + e))
			}))
			defer srv.Close()

			s := &AIService{HTTPClient: srv.Client()}
			resp, err := s.postChatCompletions(srv.URL+"/chat/completions", "k", "m",
				[]ChatMessage{{Role: "user", Content: "halo"}}, 100, 0.5)
			if err != nil {
				t.Fatalf("seharusnya berhasil, dapat error: %v", err)
			}
			if resp.Content != "Halo, ada yang bisa dibantu?" || resp.Tokens != 15 {
				t.Errorf("hasil salah: %+v", resp)
			}
		})
	}
}
