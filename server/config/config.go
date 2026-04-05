package config

import (
	"fmt"
	"os"
	"strconv"
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

	ImageGenerationModel string // Gemini model for topic thumbnail generation

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

		ImageGenerationModel: getEnv("IMAGE_GENERATION_MODEL", ""),

		TurnstileSecretKey: getEnv("TURNSTILE_SECRET_KEY", "1x0000000000000000000000000000000AA"), // Default to test key if missing

		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnvInt("SMTP_PORT", 587),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", ""),

		SlackWebhookURL: getEnv("SLACK_WEBHOOK_URL", ""),

		DailyCoinLimit:         getEnvInt("DAILY_COIN_LIMIT", 10),
		EarnMinDurationSec:    getEnvInt("EARN_MIN_DURATION_SEC", 10),
		EarnCacheRetries:      getEnvInt("EARN_CACHE_RETRIES", 3),
		MinWithdrawalAmount:   getEnvInt("MIN_WITHDRAWAL_AMOUNT", 1000),
		WithdrawalUnitAmount:   getEnvInt("WITHDRAWAL_UNIT_AMOUNT", 1000),
		ExtraCoinLimitPerLevel: getEnvInt("EXTRA_COIN_LIMIT_PER_LEVEL", 0),
		SignupBonusCoins:       getEnvInt("SIGNUP_BONUS_COINS", 0),
		CoinCap:                getEnvInt("COIN_CAP", 5000),
		CoinsPerLevel:          getEnvInt("COINS_PER_LEVEL", 1000),
		RateLimitPerMin:        getEnvInt("RATE_LIMIT_PER_MIN", 300),

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
	switch cfg.AIProvider {
	case "gemini":
		if cfg.GeminiAPIKey == "" {
			return cfg, fmt.Errorf("GEMINI_API_KEY is required when AI_PROVIDER=gemini")
		}
	case "openai":
		if cfg.OpenAIAPIKey == "" {
			return cfg, fmt.Errorf("OPENAI_API_KEY is required when AI_PROVIDER=openai")
		}
	default:
		return cfg, fmt.Errorf("unsupported AI_PROVIDER: %s (must be \"gemini\" or \"openai\")", cfg.AIProvider)
	}

	if cfg.WithdrawalUnitAmount <= 0 {
		return cfg, fmt.Errorf("WITHDRAWAL_UNIT_AMOUNT must be greater than 0")
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
