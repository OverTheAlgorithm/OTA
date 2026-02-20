package config

import (
	"fmt"
	"os"
	"strconv"

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

	GeminiAPIKey string
	GeminiModel  string

	OpenAIAPIKey string
	OpenAIModel  string

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

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

		GeminiAPIKey: getEnv("GEMINI_API_KEY", ""),
		GeminiModel:  getEnv("GEMINI_MODEL", "gemini-3.1-pro-preview"),

		OpenAIAPIKey: getEnv("OPENAI_API_KEY", ""),
		OpenAIModel:  getEnv("OPENAI_MODEL", "gpt-4o"),

		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnvInt("SMTP_PORT", 587),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", ""),

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
