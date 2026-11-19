package config

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// FallbackPublicURL dipakai bila APP_URL tidak diisi. Sengaja localhost:
// URL pada pesan ke pelanggan / redirect OAuth / invoice TIDAK boleh
// diam-diam menunjuk ke domain pihak lain. Production wajib set APP_URL.
const FallbackPublicURL = "http://localhost:8080"

var publicURL = FallbackPublicURL

// SetPublicURL menyimpan URL publik aplikasi (dipanggil sekali di Load).
func SetPublicURL(u string) {
	u = strings.TrimRight(strings.TrimSpace(u), "/")
	if u == "" {
		u = FallbackPublicURL
	}
	publicURL = u
}

// PublicURL mengembalikan URL publik tanpa garis miring akhir, mis.
// "https://wasender.slackercode.us". Dipakai pesan kuota, pengingat
// langganan, invoice PDF, redirect OAuth, dan webhook pembayaran.
func PublicURL() string { return publicURL }

// PublicHost mengembalikan host dari URL publik (tanpa skema), mis.
// "wasender.slackercode.us" — untuk tampilan ringkas seperti kop invoice.
func PublicHost() string {
	h := publicURL
	if i := strings.Index(h, "://"); i >= 0 {
		h = h[i+3:]
	}
	if i := strings.IndexAny(h, "/?#"); i >= 0 {
		h = h[:i]
	}
	return h
}

// Default backend AI (9router milik Duta Corpora). Bisa diganti lewat env
// AI_DEFAULT_BASE_URL / AI_DEFAULT_MODEL tanpa mengubah kode.
const (
	FallbackAIBaseURL = "https://9router.dutacorpora.co.id/v1"
	FallbackAIModel   = "ag/gemini-3.8-flash-low"
)

var (
	aiBaseURL = FallbackAIBaseURL
	aiModel   = FallbackAIModel
)

// SetAIDefaults menyimpan base URL (tanpa garis miring akhir) dan model bawaan
// backend AI (dipanggil sekali di Load).
func SetAIDefaults(baseURL, model string) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = FallbackAIBaseURL
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = FallbackAIModel
	}
	aiBaseURL, aiModel = baseURL, model
}

// AIDefaultBaseURL mengembalikan base URL bawaan backend AI, mis.
// "https://9router.dutacorpora.co.id/v1".
func AIDefaultBaseURL() string { return aiBaseURL }

// AIDefaultModel mengembalikan model bawaan backend AI.
func AIDefaultModel() string { return aiModel }

type Config struct {
	// Server
	Port    string
	GinMode string
	AppName string
	AppURL  string

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// JWT
	JWTSecret string
	JWTExpiry time.Duration

	// WhatsApp
	WASessionsDir string

	// Email
	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	SMTPFrom string

	// AI
	OpenAIKey    string
	AnthropicKey string

	// Payment
	PaymentKey    string
	PaymentSecret string

	// Clipku Pay — payment gateway untuk langganan (m.clipku.com)
	ClipkuPayAPIKey     string
	ClipkuPayBaseURL    string
	ClipkuPayWebhookURL string

	// Telegram
	TelegramBotToken string
	TelegramChatID   string
}

func Load() *Config {
	_ = godotenv.Load()

	expiry, _ := time.ParseDuration(getEnv("JWT_EXPIRY", "24h"))

	jwtSecret := getEnv("JWT_SECRET", "change-me-in-production")
	if jwtSecret == "change-me-in-production" {
		log.Fatal("FATAL: JWT_SECRET masih bernilai default \"change-me-in-production\". " +
			"Set JWT_SECRET yang kuat (acak, min 32 karakter) di environment/.env sebelum menjalankan server.")
	}

	SetPublicURL(getEnv("APP_URL", FallbackPublicURL))
	SetAIDefaults(getEnv("AI_DEFAULT_BASE_URL", FallbackAIBaseURL), getEnv("AI_DEFAULT_MODEL", FallbackAIModel))

	return &Config{
		Port:    getEnv("PORT", "8080"),
		GinMode: getEnv("GIN_MODE", "debug"),
		AppName: getEnv("APP_NAME", "SlackerSender"),
		AppURL:  getEnv("APP_URL", "http://localhost:8080"),

		DatabaseURL: getEnv("DATABASE_URL", "postgres://user:password@localhost:5432/wagataway?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", ""),

		JWTSecret: jwtSecret,
		JWTExpiry: expiry,

		WASessionsDir: getEnv("WA_SESSIONS_DIR", "./wa-sessions"),

		SMTPHost: getEnv("SMTP_HOST", ""),
		SMTPPort: getEnv("SMTP_PORT", "587"),
		SMTPUser: getEnv("SMTP_USER", ""),
		SMTPPass: getEnv("SMTP_PASS", ""),
		SMTPFrom: getEnv("SMTP_FROM", ""),

		OpenAIKey:    getEnv("OPENAI_API_KEY", ""),
		AnthropicKey: getEnv("ANTHROPIC_API_KEY", ""),

		PaymentKey:    getEnv("PAYMENT_GATEWAY_KEY", ""),
		PaymentSecret: getEnv("PAYMENT_GATEWAY_SECRET", ""),

		ClipkuPayAPIKey:     getEnv("CLIPKUPAY_API_KEY", ""),
		ClipkuPayBaseURL:    strings.TrimRight(getEnv("CLIPKUPAY_BASE_URL", "https://m.clipku.com"), "/"),
		// Default mengikuti APP_URL (bukan domain pihak lain).
		ClipkuPayWebhookURL: getEnv("CLIPKUPAY_WEBHOOK_URL", PublicURL()+"/api/billing/clipkupay/webhook"),

		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:   getEnv("TELEGRAM_CHAT_ID", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
