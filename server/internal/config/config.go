package config

import (
	"fmt"
	"os"

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

	ServerPort  string
	FrontendURL string
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

		ServerPort:  getEnv("SERVER_PORT", "8080"),
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:5173"),
	}

	if cfg.KakaoClientID == "" {
		return cfg, fmt.Errorf("KAKAO_CLIENT_ID is required")
	}
	if cfg.JWTSecret == "" {
		return cfg, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
