package whatsapp

// Klien HTTP untuk backend AI (9router milik Duta Corpora).
// Endpoint, API key, dan model dapat dikonfigurasi via settings:
//   ai_chat_url  (default: <AI_DEFAULT_BASE_URL>/chat/completions)
//   ai_api_key   (Bearer token untuk backend AI)
//   ai_model     (default: AI_DEFAULT_MODEL)

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Aldi1963/wagataway/internal/config"
	"github.com/Aldi1963/wagataway/internal/database/models"
	"github.com/Aldi1963/wagataway/internal/security"
	"gorm.io/gorm"
)

const (
	settingAIChatURL = "ai_chat_url"
	settingAIAPIKey  = "ai_api_key"
	settingAIModel   = "ai_model"
)

func aiSetting(db *gorm.DB, key string) string {
	var s models.Setting
	if err := db.Where("key = ?", key).First(&s).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(s.Value)
}

func aiChatURL(db *gorm.DB) string {
	if v := aiSetting(db, settingAIChatURL); v != "" {
		return v
	}
	return config.AIDefaultBaseURL() + "/chat/completions"
}

// AskAI mengirim systemPrompt + userMessage ke backend AI dan mengembalikan
// teks balasan. Memakai format OpenAI chat completions dengan fallback
// parsing untuk bentuk respon lain (reply/response/text/message).
func AskAI(db *gorm.DB, systemPrompt, userMessage string) (string, error) {
	model := aiSetting(db, settingAIModel)
	if model == "" {
		model = config.AIDefaultModel()
	}
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMessage},
		},
		"max_tokens":  500,
		"temperature": 0.7,
		// 9router membalas SSE bila stream tidak diisi; kita butuh satu JSON utuh.
		"stream": false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	chatURL := aiChatURL(db)
	if err := security.ValidateOutboundURL(chatURL); err != nil {
		return "", fmt.Errorf("URL backend AI ditolak: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, chatURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if key := aiSetting(db, settingAIAPIKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := security.NewSafeClient(60 * time.Second).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("AI backend HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Reply    string `json:"reply"`
		Response string `json:"response"`
		Text     string `json:"text"`
		Message  string `json:"message"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("respon AI tidak valid")
	}
	if len(parsed.Choices) > 0 && strings.TrimSpace(parsed.Choices[0].Message.Content) != "" {
		return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
	}
	for _, alt := range []string{parsed.Reply, parsed.Response, parsed.Text, parsed.Message} {
		if strings.TrimSpace(alt) != "" {
			return strings.TrimSpace(alt), nil
		}
	}
	return "", fmt.Errorf("respon AI kosong")
}

// AICheckHealth memeriksa konektivitas ke backend AI via /api/health
// pada host yang sama dengan ai_chat_url.
func AICheckHealth(db *gorm.DB) (bool, string) {
	u, err := url.Parse(aiChatURL(db))
	if err != nil || u.Host == "" {
		return false, "URL backend AI tidak valid"
	}
	scheme := u.Scheme
	if scheme == "" {
		scheme = "https"
	}
	healthURL := scheme + "://" + u.Host + "/api/health"
	if err := security.ValidateOutboundURL(healthURL); err != nil {
		return false, err.Error()
	}
	resp, err := security.NewSafeClient(15 * time.Second).Get(healthURL)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, "ok"
	}
	return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
}
