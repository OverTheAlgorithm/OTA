package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	KakaoClientID     string
	KakaoClientSecret string
	KakaoRedirectURI  string

	JWTSecret string

	AIProvider string // "gemini" or "openai"

	GeminiAPIKey        string
	GeminiModel         string
	GeminiModelFallback string // used when primary model returns 5xx after all retries

	OpenAIAPIKey string
	OpenAIModel  string

	// Vertex AI (express / API-key mode). Selected via AI_PROVIDER=vertex (text)
	// and/or IMAGE_PROVIDER=vertex (image). Key/model are config-only so the
	// adapter can be re-pointed without code changes.
	VertexAPIKey            string // VERTEX_API_KEY: express-mode API key (genai vertexai=true)
	VertexTextModel         string // VERTEX_TEXT_MODEL: text model; default gemini-2.5-flash
	VertexTextModelFallback string // VERTEX_TEXT_MODEL_FALLBACK: optional fallback text model on 5xx; default empty (disabled)
	VertexImageModel        string // VERTEX_IMAGE_MODEL: image model; default gemini-3.1-flash-image

	ImageProvider        string // IMAGE_PROVIDER: gemini | vertex; default gemini. Selected independently of AI_PROVIDER.
	ImageGenerationModel string // IMAGE_GENERATION_MODEL: Gemini model for thumbnails (used when IMAGE_PROVIDER=gemini)

	ImageGenThrottle    time.Duration // IMAGE_GEN_THROTTLE: delay between thumbnail generations to stay under per-minute quota (e.g. "20s"); default 0 (off)
	ImageGenMaxAttempts int           // IMAGE_GEN_MAX_ATTEMPTS: total attempts per thumbnail on retryable errors (429/5xx); default 1 (no retry)
	ImageGenBackoff     time.Duration // IMAGE_GEN_BACKOFF: initial backoff between thumbnail retries (doubles each attempt); default 5s

	TurnstileSecretKey string // Cloudflare Turnstile Secret Key

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	SlackWebhookURL string // optional; used for async admin notifications

	DailyCoinLimit int // max coins a user can earn per day (KST); 0 = unlimited

	EarnMinDurationSec int // EARN_MIN_DURATION_SEC: minimum seconds user must stay on page before earning; default 10
	EarnCacheRetries   int // EARN_CACHE_RETRIES: max retry attempts for earn cache writes; default 3

	MinWithdrawalAmount    int // MIN_WITHDRAWAL_AMOUNT: minimum coins required to request withdrawal; default 1000
	WithdrawalUnitAmount   int // WITHDRAWAL_UNIT_AMOUNT: withdrawal amount must be a multiple of this; default 1000
	ExtraCoinLimitPerLevel int // EXTRA_COIN_LIMIT_PER_LEVEL: additional daily coins per level; default 0
	SignupBonusCoins       int // SIGNUP_BONUS_COINS: coins granted to new users on registration; default 0
	CoinCap                int // COIN_CAP: maximum coins a user can hold; default 5000
	CoinsPerLevel          int // COINS_PER_LEVEL: coins needed per level transition; default 1000
	RateLimitPerMin        int // RATE_LIMIT_PER_MIN: max requests per minute per user/IP; default 300
	QuizMaxBonusCoins      int // QUIZ_MAX_BONUS_COINS: max random bonus coins for correct quiz answer (range: 1~N); default 3
	MinReferences          int // MIN_REFERENCES: minimum source URLs per topic to surface in user-facing lists, email, sitemap; default 1 (matches existing pipeline drop threshold). Set 2+ to hide single-source topics from new exposure. Topic detail page and personal history bypass this filter.
	MaxCollectedItems      int // MAX_COLLECTED_ITEMS: maximum number of articles to collect in a single run; default 8
	CTMinTagScore          float64 // CT_MIN_TAG_SCORE: community-trend conservative threshold — min score for a topic tag to surface in stats; default 3.0 (decisions.md D-002).

	RedisHost     string // REDIS_HOST: Redis server hostname; default "redis"
	RedisPort     string // REDIS_PORT: Redis server port; default "6379"
	RedisPassword string // REDIS_PASSWORD: Redis auth password; default ""
	RedisDB       int    // REDIS_DB: Redis logical database index; default 0

	DBMaxConns        int           // DB_MAX_CONNS: max open connections in pool; default 20
	DBMinConns        int           // DB_MIN_CONNS: min idle connections in pool; default 5
	DBMaxConnLifetime time.Duration // DB_MAX_CONN_LIFETIME: max connection lifetime; default 30m
	DBMaxConnIdleTime time.Duration // DB_MAX_CONN_IDLE_TIME: max connection idle time; default 5m

	BankAccountEncryptionKey string // BANK_ACCOUNT_ENCRYPTION_KEY: 32-byte hex key for AES-256-GCM encryption; empty = no encryption

	ServerPort  string
	FrontendURL string
	AppEnv      string // "development" | "production"
}

func (c Config) DatabaseURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode,
	)
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "ota"),
		DBPassword: getEnv("DB_PASSWORD", "ota_dev_password"),
		DBName:     getEnv("DB_NAME", "ota"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		KakaoClientID:     getEnv("KAKAO_CLIENT_ID", ""),
		KakaoClientSecret: getEnv("KAKAO_CLIENT_SECRET", ""),
		KakaoRedirectURI:  getEnv("KAKAO_REDIRECT_URI", "http://localhost:8080/api/v1/auth/kakao/callback"),

		JWTSecret: getEnv("JWT_SECRET", ""),

		AIProvider: getEnv("AI_PROVIDER", "gemini"),

		GeminiAPIKey:        getEnv("GEMINI_API_KEY", ""),
		GeminiModel:         getEnv("GEMINI_MODEL", "gemini-3.1-pro-preview"),
		GeminiModelFallback: getEnv("GEMINI_MODEL_FALLBACK", "gemini-3-flash-preview"),

		OpenAIAPIKey: getEnv("OPENAI_API_KEY", ""),
		OpenAIModel:  getEnv("OPENAI_MODEL", "gpt-4o"),

		VertexAPIKey:            getEnv("VERTEX_API_KEY", ""),
		VertexTextModel:         getEnv("VERTEX_TEXT_MODEL", "gemini-2.5-flash"),
		VertexTextModelFallback: getEnv("VERTEX_TEXT_MODEL_FALLBACK", ""),
		VertexImageModel:        getEnv("VERTEX_IMAGE_MODEL", "gemini-3.1-flash-image"),

		ImageProvider:        getEnv("IMAGE_PROVIDER", "gemini"),
		ImageGenerationModel: getEnv("IMAGE_GENERATION_MODEL", ""),

		ImageGenThrottle:    getEnvDuration("IMAGE_GEN_THROTTLE", 0),
		ImageGenMaxAttempts: getEnvInt("IMAGE_GEN_MAX_ATTEMPTS", 1),
		ImageGenBackoff:     getEnvDuration("IMAGE_GEN_BACKOFF", 5*time.Second),

		TurnstileSecretKey: getEnv("TURNSTILE_SECRET_KEY", "1x0000000000000000000000000000000AA"), // Default to test key if missing

		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnvInt("SMTP_PORT", 587),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", ""),

		SlackWebhookURL: getEnv("SLACK_WEBHOOK_URL", ""),

		CTMinTagScore:          getEnvFloat64("CT_MIN_TAG_SCORE", 3.0),
		DailyCoinLimit:         getEnvInt("DAILY_COIN_LIMIT", 10),
		EarnMinDurationSec:     getEnvInt("EARN_MIN_DURATION_SEC", 10),
		EarnCacheRetries:       getEnvInt("EARN_CACHE_RETRIES", 3),
		MinWithdrawalAmount:    getEnvInt("MIN_WITHDRAWAL_AMOUNT", 1000),
		WithdrawalUnitAmount:   getEnvInt("WITHDRAWAL_UNIT_AMOUNT", 1000),
		ExtraCoinLimitPerLevel: getEnvInt("EXTRA_COIN_LIMIT_PER_LEVEL", 0),
		SignupBonusCoins:       getEnvInt("SIGNUP_BONUS_COINS", 0),
		CoinCap:                getEnvInt("COIN_CAP", 5000),
		CoinsPerLevel:          getEnvInt("COINS_PER_LEVEL", 1000),
		RateLimitPerMin:        getEnvInt("RATE_LIMIT_PER_MIN", 300),
		QuizMaxBonusCoins:      getEnvInt("QUIZ_MAX_BONUS_COINS", 3),
		MinReferences:          getEnvInt("MIN_REFERENCES", 1),
		MaxCollectedItems:      getEnvInt("MAX_COLLECTED_ITEMS", 8),

		RedisHost:     getEnv("REDIS_HOST", "redis"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		DBMaxConns:        getEnvInt("DB_MAX_CONNS", 20),
		DBMinConns:        getEnvInt("DB_MIN_CONNS", 5),
		DBMaxConnLifetime: getEnvDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute),
		DBMaxConnIdleTime: getEnvDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute),

		BankAccountEncryptionKey: getEnv("BANK_ACCOUNT_ENCRYPTION_KEY", ""),

		ServerPort:  getEnv("SERVER_PORT", "8080"),
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:5173"),
		AppEnv:      getEnv("APP_ENV", "development"),
	}

	if cfg.KakaoClientID == "" {
		return cfg, fmt.Errorf("KAKAO_CLIENT_ID is required")
	}
	if cfg.JWTSecret == "" {
		return cfg, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.AppEnv == "production" && len(cfg.JWTSecret) < 32 {
		return cfg, fmt.Errorf("JWT_SECRET must be at least 32 characters in production (got %d)", len(cfg.JWTSecret))
	}
	switch cfg.AIProvider {
	case "gemini":
		if cfg.GeminiAPIKey == "" {
			return cfg, fmt.Errorf("GEMINI_API_KEY is required when AI_PROVIDER=gemini")
		}
	case "openai":
		if cfg.OpenAIAPIKey == "" {
			return cfg, fmt.Errorf("OPENAI_API_KEY is required when AI_PROVIDER=openai")
		}
	case "vertex":
		if cfg.VertexAPIKey == "" {
			return cfg, fmt.Errorf("VERTEX_API_KEY is required when AI_PROVIDER=vertex")
		}
	default:
		return cfg, fmt.Errorf("unsupported AI_PROVIDER: %s (must be \"gemini\", \"openai\" or \"vertex\")", cfg.AIProvider)
	}

	switch cfg.ImageProvider {
	case "gemini":
		if cfg.GeminiAPIKey == "" {
			return cfg, fmt.Errorf("GEMINI_API_KEY is required when IMAGE_PROVIDER=gemini")
		}
		if cfg.ImageGenerationModel == "" {
			return cfg, fmt.Errorf("IMAGE_GENERATION_MODEL is required when IMAGE_PROVIDER=gemini")
		}
	case "vertex":
		if cfg.VertexAPIKey == "" {
			return cfg, fmt.Errorf("VERTEX_API_KEY is required when IMAGE_PROVIDER=vertex")
		}
	default:
		return cfg, fmt.Errorf("unsupported IMAGE_PROVIDER: %s (must be \"gemini\" or \"vertex\")", cfg.ImageProvider)
	}

	if cfg.AppEnv == "production" && (cfg.TurnstileSecretKey == "" ||
		cfg.TurnstileSecretKey == "dummy-secret-key" ||
		strings.HasPrefix(cfg.TurnstileSecretKey, "1x000000000000000000000000000000")) {
		return cfg, fmt.Errorf("TURNSTILE_SECRET_KEY must be a valid production key (test/dummy keys are not allowed in production)")
	}

	if cfg.AppEnv == "production" && cfg.BankAccountEncryptionKey == "" {
		return cfg, fmt.Errorf("BANK_ACCOUNT_ENCRYPTION_KEY is required in production (empty key stores bank accounts in plaintext)")
	}

	if cfg.WithdrawalUnitAmount <= 0 {
		return cfg, fmt.Errorf("WITHDRAWAL_UNIT_AMOUNT must be greater than 0")
	}
	if cfg.MinReferences < 0 {
		return cfg, fmt.Errorf("MIN_REFERENCES must be >= 0 (got %d)", cfg.MinReferences)
	}
	if cfg.MaxCollectedItems <= 0 {
		return cfg, fmt.Errorf("MAX_COLLECTED_ITEMS must be greater than 0")
	}
	if cfg.MinWithdrawalAmount%cfg.WithdrawalUnitAmount != 0 {
		return cfg, fmt.Errorf("MIN_WITHDRAWAL_AMOUNT (%d) must be a multiple of WITHDRAWAL_UNIT_AMOUNT (%d)", cfg.MinWithdrawalAmount, cfg.WithdrawalUnitAmount)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvFloat64(key string, fallback float64) float64 {
	if val := os.Getenv(key); val != "" {
		if floatVal, err := strconv.ParseFloat(val, 64); err == nil {
			return floatVal
		}
	}
	return fallback
}
