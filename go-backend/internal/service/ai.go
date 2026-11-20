package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/config"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/security"
)

// AIProvider represents an AI provider
type AIProvider string

const (
	ProviderOpenAI     AIProvider = "openai"
	ProviderGemini     AIProvider = "gemini"
	ProviderAnthropic  AIProvider = "anthropic"
	ProviderDeepSeek   AIProvider = "deepseek"
	ProviderZAI        AIProvider = "zai"     // Zhipu AI (z.ai), OpenAI-compatible
	ProviderNineRouter AIProvider = "9router" // 9router, OpenAI-compatible, endpoint bisa diganti
	ProviderCustom     AIProvider = "custom"  // OpenAI-compatible endpoint bebas
)

// ProviderDefaults: model bawaan + base URL tiap provider.
var ProviderDefaults = map[AIProvider]struct {
	BaseURL string
	Model   string
}{
	ProviderOpenAI:    {"https://api.openai.com/v1", "gpt-4o-mini"},
	ProviderGemini:    {"https://generativelanguage.googleapis.com/v1beta", "gemini-2.0-flash"},
	ProviderAnthropic: {"https://api.anthropic.com/v1", "claude-3-5-haiku-20241022"},
	ProviderDeepSeek:  {"https://api.deepseek.com", "deepseek-chat"},
	ProviderZAI:       {"https://api.z.ai/api/paas/v4", "glm-4.5"},
	// 9router: base URL & model diambil dari config (AI_DEFAULT_BASE_URL /
	// AI_DEFAULT_MODEL) lewat baseURLDefaultFor/DefaultModelFor — nilai di
	// sini hanya placeholder karena map dibuat sebelum config dimuat.
	ProviderNineRouter: {"", ""},
	ProviderCustom:     {"", "default"},
}

// defaultBaseURLFor mengembalikan base URL bawaan provider; 9router mengikuti config.
func defaultBaseURLFor(p AIProvider) string {
	if p == ProviderNineRouter {
		return config.AIDefaultBaseURL()
	}
	return ProviderDefaults[p].BaseURL
}

// CuratedModels: daftar model yang ditawarkan di dropdown UI per provider.
// Pengguna tetap bisa mengetik nama model lain secara manual.
var CuratedModels = map[AIProvider][]string{
	ProviderOpenAI:     {"gpt-4o-mini", "gpt-4o", "gpt-4.1-mini", "gpt-4.1"},
	ProviderGemini:     {"gemini-2.0-flash", "gemini-2.5-flash", "gemini-1.5-flash"},
	ProviderAnthropic:  {"claude-3-5-haiku-20241022", "claude-sonnet-4-20250514"},
	ProviderDeepSeek:   {"deepseek-chat", "deepseek-reasoner"},
	ProviderZAI:        {"glm-4.5", "glm-4.5-air", "glm-4"},
	ProviderNineRouter: {},
	ProviderCustom:     {},
}

// baseURLEditable: provider yang base URL-nya boleh diubah user di UI.
// (custom wajib isi; lainnya opsional — kosong = pakai bawaan.)
var baseURLEditable = map[AIProvider]bool{
	ProviderDeepSeek:   true,
	ProviderZAI:        true,
	ProviderNineRouter: true,
	ProviderCustom:     true,
}

// IsBaseURLEditable melaporkan apakah base URL provider bisa diubah user.
func IsBaseURLEditable(p AIProvider) bool {
	return baseURLEditable[p]
}

// AIService handles AI chat completions
type AIService struct {
	OpenAIKey    string
	AnthropicKey string
	// Koneksi milik user (menggantikan key env bila diisi).
	Provider   AIProvider
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// aiHTTPClient: client anti-SSRF (menolak dial ke IP internal, tidak mengikuti
// redirect). Base URL AI boleh diubah user, jadi tidak boleh bisa diarahkan
// ke 127.0.0.1 / jaringan privat / metadata cloud. Bila HTTPS_PROXY terisi,
// SafeClient otomatis lewat proxy tersebut.
//
// Batas waktu total 150 detik: jawaban panjang (ribuan token) butuh lebih dari
// 60 detik. Batas dial tetap 60 detik agar host yang mati tak menahan lama.
func aiHTTPClient() *http.Client {
	c := security.NewSafeClient(60 * time.Second)
	c.Timeout = 150 * time.Second
	return c
}

// NewAIService creates a new AI service (legacy: keys dari env)
func NewAIService(openaiKey, anthropicKey string) *AIService {
	return &AIService{
		OpenAIKey:    openaiKey,
		AnthropicKey: anthropicKey,
		HTTPClient:   aiHTTPClient(),
	}
}

// NewAIServiceForConnection membuat AI service dari koneksi milik user.
func NewAIServiceForConnection(provider AIProvider, apiKey, baseURL string) *AIService {
	return &AIService{
		Provider:   provider,
		APIKey:     apiKey,
		BaseURL:    baseURL,
		HTTPClient: aiHTTPClient(),
	}
}

// baseURLFor mengembalikan base URL efektif (custom bila diisi, default bila kosong).
func (s *AIService) baseURLFor(p AIProvider) string {
	if s.BaseURL != "" {
		return strings.TrimSuffix(s.BaseURL, "/")
	}
	if _, ok := ProviderDefaults[p]; ok {
		return defaultBaseURLFor(p)
	}
	return ""
}

// apiKeyFor mengembalikan API key efektif: dari koneksi user bila provider-nya
// cocok, sonst fallback ke key env (legacy).
func (s *AIService) apiKeyFor(p AIProvider) string {
	if s.Provider == p && s.APIKey != "" {
		return s.APIKey
	}
	switch p {
	case ProviderOpenAI, ProviderCustom, ProviderDeepSeek, ProviderZAI, ProviderNineRouter:
		return s.OpenAIKey
	case ProviderAnthropic:
		return s.AnthropicKey
	}
	return ""
}

// ChatMessage represents a message in conversation
type ChatMessage struct {
	Role    string `json:"role"` // system, user, assistant
	Content string `json:"content"`
	// Images: gambar yang menyertai pesan (vision). json:"-" agar pemakai lama
	// yang meng-marshal ChatMessage langsung tetap menghasilkan body yang sama;
	// rangkaian per provider ada di ai_vision.go.
	Images []ChatImage `json:"-"`
}

// AIReplyMaxTokens: batas token jawaban AI Reply. Hanya batas atas — panjang
// balasan ke pelanggan tetap dipotong MaxReplyLength. Dibuat longgar karena
// model reasoning (GLM-5.x, Gemini 2.5, dsb.) menghitung token berpikir ke
// dalam batas ini; 500 terlalu kecil → content kosong, balasan hilang.
const AIReplyMaxTokens = 2048

// AIReplyMaxTokensCeiling: langit-langit anggaran token jawaban. Banyak model
// membatasi keluaran di kisaran 8 ribu token; melewatinya memicu HTTP 400.
const AIReplyMaxTokensCeiling = 8192

// AIReplyTokensFor menghitung batas token jawaban dari batas karakter balasan:
// ±3 karakter per token (Indonesia, perkiraan konservatif) ditambah 2048 ruang
// berpikir untuk model reasoning. Tidak pernah di bawah AIReplyMaxTokens dan
// tidak melewati AIReplyMaxTokensCeiling. Hanya batas atas — token yang tak
// terpakai tidak ditagih.
func AIReplyTokensFor(maxReplyChars int) int {
	t := maxReplyChars/3 + 2048
	if t < AIReplyMaxTokens {
		t = AIReplyMaxTokens
	}
	if t > AIReplyMaxTokensCeiling {
		t = AIReplyMaxTokensCeiling
	}
	return t
}

// ChatRequest holds parameters for AI completion
type ChatRequest struct {
	Provider    AIProvider
	Model       string
	Messages    []ChatMessage
	MaxTokens   int
	Temperature float64
	// ReasoningEffort (opsional): tingkat berpikir model reasoning, dikirim
	// apa adanya ke provider OpenAI-compatible. Kosong = tidak dikirim.
	// Lihat ReasoningEffortFor untuk kapan aman dipakai.
	ReasoningEffort string
	// WebSearch: izinkan pencarian web bawaan provider (ai_websearch.go).
	// Diabaikan provider yang tidak didukung (lihat WebSearchSupported).
	WebSearch bool
	// DowngradeMessages (opsional): pesan pengganti yang dipakai CompleteTolerant
	// saat mengulang tanpa fitur tambahan — mis. system prompt tanpa kalimat
	// "alat pencarian tersedia" agar model tak mengaku mencari padahal tidak.
	DowngradeMessages []ChatMessage
}

// ChatResponse holds the AI response
type ChatResponse struct {
	Content string
	Tokens  int
	// FinishReason & Reasoning hanya terisi dari provider OpenAI-compatible;
	// dipakai untuk menjelaskan jawaban kosong (lihat EmptyHint).
	FinishReason string
	Reasoning    bool // respons membawa reasoning_content (model berpikir)
	// Searched: provider benar-benar mencari di web; Sources: label situs
	// sumber (untuk log, tidak dikirim ke pelanggan).
	Searched bool
	Sources  []string
	// Downgraded: provider menolak request berfitur tambahan (HTTP 400) lalu
	// CompleteTolerant mengulang tanpa reasoning_effort & pencarian web.
	Downgraded bool
}

// EmptyHint menjelaskan kemungkinan sebab jawaban kosong untuk log/diagnosis.
// Kasus khas: model reasoning (mis. GLM-5.x) menghabiskan seluruh max_tokens
// untuk berpikir sehingga content kosong.
func (r *ChatResponse) EmptyHint() string {
	switch {
	case r.FinishReason == "length" && r.Reasoning:
		return "token habis dipakai berpikir (model reasoning) — naikkan max_tokens atau pakai model non-reasoning"
	case r.FinishReason == "length":
		return "terpotong batas max_tokens"
	case r.FinishReason != "":
		return "finish_reason=" + r.FinishReason
	}
	return "provider tidak mengirim finish_reason"
}

// ReasoningEffortFor: tingkat berpikir yang aman untuk AI Reply. Hanya Z.AI
// langsung dengan GLM-5.3 / GLM-5.3-FLASH (selalu berpikir, bawaan "max"; docs
// Z.AI menyebut reasoning_effort low|high|max untuk model ini) yang dikirimi
// "low" agar balasan chat cepat & tidak menghabiskan token. Model GLM lain dan
// provider lain (termasuk 9router) tidak dikirimi — belum terdokumentasi
// menerima field itu.
func ReasoningEffortFor(p AIProvider, model string) string {
	if p == ProviderZAI && strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "glm-5.3") {
		return "low"
	}
	return ""
}

// CompleteTolerant memanggil Complete; bila provider menolak (HTTP 400) request
// yang membawa fitur tambahan (ReasoningEffort / WebSearch), ulangi SEKALI
// tanpa keduanya — jaring pengaman bila provider/model tak mengenal fitur itu,
// agar balasan tak hilang. Hasil ulangan ditandai Downgraded.
func (s *AIService) CompleteTolerant(req ChatRequest) (*ChatResponse, error) {
	resp, err := s.Complete(req)
	// Batas token besar (jawaban panjang) ditolak sebagian model dengan HTTP 400:
	// ulangi sekali dengan batas lama agar balasan yang dulu berhasil tidak hilang.
	if isBadRequest(err) && req.MaxTokens > AIReplyMaxTokens {
		req.MaxTokens = AIReplyMaxTokens
		resp, err = s.Complete(req)
		if resp != nil {
			resp.Downgraded = true
		}
	}
	if isBadRequest(err) && (req.ReasoningEffort != "" || req.WebSearch) {
		req.ReasoningEffort = ""
		req.WebSearch = false
		if req.DowngradeMessages != nil {
			req.Messages = req.DowngradeMessages
		}
		resp, err = s.Complete(req)
		if resp != nil {
			resp.Downgraded = true
		}
	}
	return resp, err
}

// NormalizeChatHistory merapikan riwayat percakapan (terurut kronologis,
// tanpa system prompt) agar diterima semua provider:
//   - pesan berurutan dengan role sama digabung (Gemini menuntut alternasi
//     user/model yang ketat saat dikonversi dari format OpenAI)
//   - pesan assistant di ujung dibuang — itu balasan yang sudah terkirim,
//     bukan yang perlu dibalas; Gemini menolak request yang berakhir di
//     model turn ("Requests ending with a model turn are not supported")
func NormalizeChatHistory(msgs []ChatMessage) []ChatMessage {
	merged := make([]ChatMessage, 0, len(msgs))
	for _, m := range msgs {
		if m.Content == "" {
			continue
		}
		if n := len(merged); n > 0 && merged[n-1].Role == m.Role {
			merged[n-1].Content += "\n" + m.Content
			continue
		}
		merged = append(merged, m)
	}
	for len(merged) > 0 && merged[len(merged)-1].Role == "assistant" {
		merged = merged[:len(merged)-1]
	}
	return merged
}

// Complete sends a chat completion request to the configured provider
func (s *AIService) Complete(req ChatRequest) (*ChatResponse, error) {
	switch req.Provider {
	case ProviderAnthropic:
		return s.completeAnthropic(req)
	case ProviderGemini:
		return s.completeGemini(req)
	case ProviderCustom, ProviderDeepSeek, ProviderZAI, ProviderNineRouter:
		return s.completeOpenAICompatible(req)
	default:
		return s.completeOpenAI(req)
	}
}

func (s *AIService) defaultsFor(req ChatRequest) (model string, maxTokens int, temp float64) {
	model = req.Model
	maxTokens = req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 500
	}
	temp = req.Temperature
	if temp == 0 {
		temp = 0.7
	}
	return model, maxTokens, temp
}

func (s *AIService) completeOpenAI(req ChatRequest) (*ChatResponse, error) {
	key := s.apiKeyFor(ProviderOpenAI)
	if key == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}
	model, maxTokens, temp := s.defaultsFor(req)
	if model == "" {
		model = ProviderDefaults[ProviderOpenAI].Model
	}
	return s.postChatCompletions(s.baseURLFor(ProviderOpenAI)+"/chat/completions", key, model, req.Messages, maxTokens, temp)
}

// completeOpenAICompatible dipakai untuk provider OpenAI-compatible:
// custom, deepseek, zai, 9router (format /chat/completions + Bearer).
func (s *AIService) completeOpenAICompatible(req ChatRequest) (*ChatResponse, error) {
	key := s.apiKeyFor(req.Provider)
	base := s.baseURLFor(req.Provider)
	if base == "" {
		return nil, fmt.Errorf("custom AI base URL not configured")
	}
	model, maxTokens, temp := s.defaultsFor(req)
	if model == "" {
		model = "default"
	}
	extra := map[string]interface{}{}
	if req.ReasoningEffort != "" {
		extra["reasoning_effort"] = req.ReasoningEffort
	}
	// Pencarian web hanya untuk Z.AI (proxy lain belum tentu meneruskan tools).
	if req.WebSearch && req.Provider == ProviderZAI {
		extra["tools"] = zaiWebSearchTool()
	}
	return s.postChatCompletions(base+"/chat/completions", key, model, req.Messages, maxTokens, temp, extra)
}

// postChatCompletions: extra (opsional) = field tambahan body, mis. reasoning_effort.
func (s *AIService) postChatCompletions(url, key, model string, messages []ChatMessage, maxTokens int, temp float64, extra ...map[string]interface{}) (*ChatResponse, error) {
	body := map[string]interface{}{
		"model":       model,
		"messages":    OpenAIMessages(messages),
		"max_tokens":  maxTokens,
		"temperature": temp,
	}
	for _, e := range extra {
		for k, v := range e {
			body[k] = v
		}
	}

	jsonBody, _ := json.Marshal(body)
	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	httpReq.Header.Set("Content-Type", "application/json")
	if key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := s.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("AI error %d: %s", resp.StatusCode, truncateAIError(string(respBody)))
	}

	// Beberapa proxy OpenAI-compatible mengembalikan SSE stream ("data: {...}")
	// alih-alih satu JSON utuh — tangani keduanya.
	if looksLikeSSE(respBody) {
		return parseSSEChatResponse(respBody)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
		// Hasil pencarian web Z.AI (level atas respons). RawMessage agar bentuk
		// tak terduga (objek, string) dari proxy/provider lain TIDAK memecahkan
		// decode jawaban normal; diurai toleran di bawah.
		WebSearch json.RawMessage `json:"web_search"`
	}
	// Decoder (bukan json.Unmarshal): sebagian proxy (mis. 9router) mengirim satu
	// objek chat.completion utuh LALU ekor SSE ("data: [DONE]"). Unmarshal menolak
	// teks apa pun setelah objek ("invalid character 'd' after top-level value");
	// Decoder cukup membaca satu objek JSON pertama dan mengabaikan sisanya.
	if err := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(respBody))).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %v (body: %s)", err, truncateAIError(string(respBody)))
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("AI returned no choices")
	}

	c := result.Choices[0]
	out := &ChatResponse{
		Content:      c.Message.Content,
		Tokens:       result.Usage.TotalTokens,
		FinishReason: c.FinishReason,
		Reasoning:    strings.TrimSpace(c.Message.ReasoningContent) != "",
	}
	var hits []struct {
		Title string `json:"title"`
		Link  string `json:"link"`
		Media string `json:"media"`
	}
	// Bukan array / gagal diurai -> abaikan (tak mencari), jawaban tetap utuh.
	if len(result.WebSearch) > 0 && json.Unmarshal(result.WebSearch, &hits) == nil {
		out.Searched = len(hits) > 0
		for _, w := range hits {
			out.Sources = addSource(out.Sources, w.Media, w.Link, w.Title)
		}
	}
	return out, nil
}

// looksLikeSSE melaporkan apakah body tampak seperti Server-Sent Events.
func looksLikeSSE(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	return bytes.HasPrefix(trimmed, []byte("data:"))
}

// parseSSEChatResponse merangkai potongan delta.content dari SSE stream
// format OpenAI chat completions.
func parseSSEChatResponse(body []byte) (*ChatResponse, error) {
	var sb strings.Builder
	tokens := 0
	finish := ""
	reasoning := false
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(payload) == 0 || string(payload) == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				TotalTokens int `json:"total_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(payload, &chunk); err != nil {
			continue // lewati baris yang bukan JSON valid
		}
		if len(chunk.Choices) > 0 {
			ch := chunk.Choices[0]
			sb.WriteString(ch.Delta.Content)
			if strings.TrimSpace(ch.Delta.ReasoningContent) != "" {
				reasoning = true
			}
			if ch.FinishReason != "" {
				finish = ch.FinishReason
			}
		}
		if chunk.Usage.TotalTokens > 0 {
			tokens = chunk.Usage.TotalTokens
		}
	}
	content := strings.TrimSpace(sb.String())
	if content == "" {
		// Tetap error (perilaku lama, pemanggil tak perlu cek kosong) — tapi kini
		// dengan sebab yang bisa dibaca (mis. token habis dipakai berpikir).
		hint := (&ChatResponse{FinishReason: finish, Reasoning: reasoning}).EmptyHint()
		return nil, fmt.Errorf("AI returned empty SSE stream: %s (body: %s)", hint, truncateAIError(string(body)))
	}
	return &ChatResponse{Content: content, Tokens: tokens, FinishReason: finish, Reasoning: reasoning}, nil
}

// truncateAIError memotong pesan error agar tidak membocorkan seluruh body.
func truncateAIError(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}

func (s *AIService) completeAnthropic(req ChatRequest) (*ChatResponse, error) {
	key := s.apiKeyFor(ProviderAnthropic)
	if key == "" {
		return nil, fmt.Errorf("Anthropic API key not configured")
	}

	model := req.Model
	if model == "" {
		model = ProviderDefaults[ProviderAnthropic].Model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 500
	}

	// Pisahkan system message; pesan bergambar dirangkai di ai_vision.go.
	systemMsg, messages := anthropicMessages(req.Messages)

	body := map[string]interface{}{
		"model":      model,
		"max_tokens": maxTokens,
		"messages":   messages,
	}
	if systemMsg != "" {
		body["system"] = systemMsg
	}
	if req.WebSearch {
		body["tools"] = anthropicWebSearchTool()
	}

	jsonBody, _ := json.Marshal(body)
	httpReq, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", key)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := s.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Anthropic error %d: %s", resp.StatusCode, string(respBody))
	}

	// Gabung semua blok teks (respons pencarian web terpecah ke banyak blok).
	return parseAnthropicResponse(respBody)
}

// completeGemini memanggil Google Generative Language API (generateContent).
// Bentuk API berbeda dari OpenAI: role "assistant" dipetakan ke "model",
// system prompt dikirim via system_instruction, auth via header x-goog-api-key.
func (s *AIService) completeGemini(req ChatRequest) (*ChatResponse, error) {
	key := s.apiKeyFor(ProviderGemini)
	if key == "" {
		return nil, fmt.Errorf("Gemini API key not configured")
	}
	model := req.Model
	if model == "" {
		model = ProviderDefaults[ProviderGemini].Model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 500
	}
	temp := req.Temperature
	if temp == 0 {
		temp = 0.7
	}

	type part struct {
		Text string `json:"text"`
	}
	type content struct {
		Role  string        `json:"role,omitempty"`
		Parts []interface{} `json:"parts"`
	}
	var systemText string
	var contents []content
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			if systemText == "" {
				systemText = m.Content
			} else {
				systemText += "\n" + m.Content
			}
		case "assistant":
			contents = append(contents, content{Role: "model", Parts: geminiParts(m)})
		default:
			contents = append(contents, content{Role: "user", Parts: geminiParts(m)})
		}
	}

	body := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": maxTokens,
			"temperature":     temp,
		},
	}
	if systemText != "" {
		body["system_instruction"] = map[string]interface{}{
			"parts": []part{{Text: systemText}},
		}
	}

	jsonBody, _ := json.Marshal(body)
	url := s.baseURLFor(ProviderGemini) + "/models/" + model + ":generateContent"
	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", key)

	resp, err := s.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Gemini error %d: %s", resp.StatusCode, truncateAIError(string(respBody)))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			TotalTokenCount int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response: %w", err)
	}
	if result.Error.Message != "" {
		return nil, fmt.Errorf("Gemini error: %s", result.Error.Message)
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("Gemini returned no content")
	}

	var sb strings.Builder
	for _, p := range result.Candidates[0].Content.Parts {
		sb.WriteString(p.Text)
	}
	return &ChatResponse{
		Content: strings.TrimSpace(sb.String()),
		Tokens:  result.UsageMetadata.TotalTokenCount,
	}, nil
}
