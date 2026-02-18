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
	"ota/domain/user"
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

	// 프로덕션 환경에서는 Gin 릴리즈 모드로 실행 (디버그 로그 및 라우트 목록 출력 억제)
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
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

	// Collection schedule: 4 AM, 5 AM, 6 AM KST (19:00, 20:00, 21:00 UTC)
	// Multiple attempts to ensure data is ready by 6 AM
	_, err = scheduler.AddFunc("0 19 * * *", collectFunc) // 4 AM KST
	if err != nil {
		log.Fatalf("failed to schedule 4 AM collection: %v", err)
	}
	_, err = scheduler.AddFunc("0 20 * * *", collectFunc) // 5 AM KST
	if err != nil {
		log.Fatalf("failed to schedule 5 AM collection: %v", err)
	}
	_, err = scheduler.AddFunc("0 21 * * *", collectFunc) // 6 AM KST (final attempt)
	if err != nil {
		log.Fatalf("failed to schedule 6 AM collection: %v", err)
	}

	scheduler.Start()
	log.Println("scheduler started (collection at 4 AM, 5 AM, 6 AM KST)")

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

	// Delivery schedule
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

	// Primary delivery at 7:00 AM KST (22:00 UTC)
	_, err = scheduler.AddFunc("0 22 * * *", deliverFunc)
	if err != nil {
		log.Fatalf("failed to schedule primary delivery: %v", err)
	}

	// Fallback delivery at 7:15 AM KST (22:15 UTC) if collection was late
	_, err = scheduler.AddFunc("15 22 * * *", deliverFunc)
	if err != nil {
		log.Fatalf("failed to schedule fallback delivery: %v", err)
	}
	log.Println("delivery scheduler added (7:00 AM + 7:15 AM KST fallback)")

	// Handlers
	userRepo := storage.NewUserRepository(pool)
	subscriptionRepo := storage.NewSubscriptionRepository(pool)
	kakaoClient := kakao.NewClient(cfg.KakaoClientID, cfg.KakaoClientSecret, cfg.KakaoRedirectURI)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)
	stateStore := auth.NewStateStore()
	authHandler := handler.NewAuthHandler(kakaoClient, jwtManager, stateStore, userRepo, cfg.FrontendURL)
	adminHandler := handler.NewAdminHandler(collectorService)
	deliveryHandler := api.NewDeliveryHandler(deliveryService)
	userDeliveryChannelsHandler := handler.NewUserDeliveryChannelsHandler(deliveryRepo)
	subscriptionHandler := handler.NewSubscriptionHandler(subscriptionRepo, api.AuthMiddleware(jwtManager))

	// Email verification
	emailVerificationRepo := storage.NewEmailVerificationRepository(pool)
	emailVerificationService := user.NewEmailVerificationService(emailVerificationRepo, userRepo)
	emailVerificationHandler := handler.NewEmailVerificationHandler(emailVerificationService, emailSender)

	// Context history
	historyRepo := storage.NewHistoryRepository(pool)
	contextHistoryHandler := handler.NewContextHistoryHandler(historyRepo, api.AuthMiddleware(jwtManager))

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
		{
			GroupName:   "user",
			Handler:     userDeliveryChannelsHandler,
			Middlewares: []gin.HandlerFunc{api.AuthMiddleware(jwtManager)},
		},
		{
			GroupName:   "subscriptions",
			Handler:     subscriptionHandler,
			Middlewares: []gin.HandlerFunc{},
		},
		{
			GroupName:   "email-verification",
			Handler:     emailVerificationHandler,
			Middlewares: []gin.HandlerFunc{api.AuthMiddleware(jwtManager)},
		},
		{
			GroupName:   "context",
			Handler:     contextHistoryHandler,
			Middlewares: []gin.HandlerFunc{},
		},
	})

	log.Printf("server starting on :%s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
