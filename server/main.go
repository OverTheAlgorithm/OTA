package main

import (
	"context"
	"log"

	"ota/api"
	"ota/api/handler"
	"ota/auth"
	"ota/config"
	"ota/domain/collector"
	"ota/platform/kakao"
	"ota/platform/openai"
	"ota/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()

	if err := storage.RunMigrations(cfg.DatabaseURL(), "migrations"); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations completed")

	pool, err := storage.NewPool(ctx, cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Println("database connected")

	// Data collection
	aiClient := openai.NewClient(cfg.OpenAIAPIKey, cfg.OpenAIModel)
	collectorRepo := storage.NewCollectorRepository(pool)
	collectorService := collector.NewService(aiClient, collectorRepo)
	_ = collectorService // TODO: wire to scheduler and/or HTTP handler
	log.Println("collector service initialized")

	userRepo := storage.NewUserRepository(pool)
	kakaoClient := kakao.NewClient(cfg.KakaoClientID, cfg.KakaoClientSecret, cfg.KakaoRedirectURI)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)
	stateStore := auth.NewStateStore()
	authHandler := handler.NewAuthHandler(kakaoClient, jwtManager, stateStore, userRepo, cfg.FrontendURL)

	r := api.NewRouter(authHandler, jwtManager, cfg.FrontendURL)

	log.Printf("server starting on :%s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
