package main

import (
	"context"
	"log"

	"ota/internal/auth"
	"ota/internal/config"
	"ota/internal/database"
	"ota/internal/server"
	"ota/internal/user"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()

	if err := database.RunMigrations(cfg.DatabaseURL(), "migrations"); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations completed")

	pool, err := database.NewPool(ctx, cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Println("database connected")

	userRepo := user.NewPostgresRepository(pool)
	kakaoClient := auth.NewKakaoClient(cfg.KakaoClientID, cfg.KakaoClientSecret, cfg.KakaoRedirectURI)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)
	stateStore := auth.NewStateStore()
	authHandler := auth.NewHandler(kakaoClient, jwtManager, stateStore, userRepo, cfg.FrontendURL)

	r := server.New(authHandler, jwtManager, cfg.FrontendURL)

	log.Printf("server starting on :%s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
