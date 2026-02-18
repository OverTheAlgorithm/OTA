package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"

	"ota/api"
	"ota/api/handler"
	"ota/auth"
	"ota/config"
	"ota/domain/collector"
	"ota/domain/delivery"
	"ota/platform/email"
	"ota/platform/gemini"
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
	var aiClient collector.AIClient
	switch cfg.AIProvider {
	case "gemini":
		aiClient = gemini.NewClient(cfg.GeminiAPIKey, cfg.GeminiModel)
	case "openai":
		aiClient = openai.NewClient(cfg.OpenAIAPIKey, cfg.OpenAIModel)
	}
	collectorRepo := storage.NewCollectorRepository(pool)
	collectorService := collector.NewService(aiClient, collectorRepo)
	log.Printf("collector service initialized (provider: %s)", cfg.AIProvider)

	// Schedule collection with distributed coordination
	scheduler := cron.New(cron.WithLocation(time.UTC))

	collectFunc := func() {
		log.Println("checking if collection is needed")
		result, err := collectorService.CollectIfNeeded(context.Background())
		if err != nil {
			log.Printf("collection failed: %v", err)
			return
		}
		if result == nil {
			log.Println("collection already completed today or in progress, skipping")
			return
		}
		log.Printf("collection completed: run_id=%s, items=%d", result.Run.ID, len(result.Items))
	}

	// Daily at 7 AM KST (10 PM UTC previous day)
	_, err = scheduler.AddFunc("0 22 * * *", collectFunc)
	if err != nil {
		log.Fatalf("failed to schedule daily collection: %v", err)
	}

	// Hourly check (retry if failed)
	_, err = scheduler.AddFunc("0 * * * *", collectFunc)
	if err != nil {
		log.Fatalf("failed to schedule hourly check: %v", err)
	}

	scheduler.Start()
	log.Println("scheduler started (daily at 7 AM KST / 10 PM UTC + hourly retry)")

	// Message delivery system
	emailSender := email.NewSMTPSender(email.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
	})
	deliveryRepo := storage.NewDeliveryRepository(pool)
	collectorAdapter := storage.NewCollectorServiceAdapter(pool)
	deliveryService := delivery.NewService(deliveryRepo, emailSender, collectorAdapter)
	log.Println("delivery service initialized")

	// Schedule delivery 15 minutes after collection (7:15 AM KST / 10:15 PM UTC)
	deliverFunc := func() {
		log.Println("starting message delivery")
		result, err := deliveryService.DeliverAll(context.Background())
		if err != nil {
			log.Printf("delivery failed: %v", err)
			return
		}
		log.Printf("delivery completed: total=%d, success=%d, failed=%d, skipped=%d",
			result.TotalUsers, result.SuccessCount, result.FailureCount, result.SkippedCount)
	}

	_, err = scheduler.AddFunc("15 22 * * *", deliverFunc)
	if err != nil {
		log.Fatalf("failed to schedule delivery: %v", err)
	}
	log.Println("delivery scheduler added (daily at 7:15 AM KST / 10:15 PM UTC)")

	// Handlers
	userRepo := storage.NewUserRepository(pool)
	kakaoClient := kakao.NewClient(cfg.KakaoClientID, cfg.KakaoClientSecret, cfg.KakaoRedirectURI)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)
	stateStore := auth.NewStateStore()
	authHandler := handler.NewAuthHandler(kakaoClient, jwtManager, stateStore, userRepo, cfg.FrontendURL)
	adminHandler := handler.NewAdminHandler(collectorService)
	deliveryHandler := api.NewDeliveryHandler(deliveryService)

	// Router
	r := api.NewRouter("api", "v1", cfg.FrontendURL, []api.RouteModule{
		{
			GroupName:   "auth",
			Handler:     authHandler,
			Middlewares: []gin.HandlerFunc{},
		},
		{
			GroupName:   "admin",
			Handler:     adminHandler,
			Middlewares: []gin.HandlerFunc{},
		},
		{
			GroupName:   "delivery",
			Handler:     deliveryHandler,
			Middlewares: []gin.HandlerFunc{},
		},
	})

	log.Printf("server starting on :%s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
