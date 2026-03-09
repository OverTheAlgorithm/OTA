package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"ota/api"
	"ota/api/handler"
	"ota/auth"
	"ota/cache"
	"ota/config"
	"ota/domain/collector"
	"ota/domain/delivery"
	"ota/domain/level"
	"ota/domain/terms"
	"ota/domain/user"
	"ota/domain/withdrawal"
	"ota/platform/email"
	"ota/platform/gemini"
	"ota/platform/googlenews"
	"ota/platform/googletrends"
	"ota/platform/kakao"
	"ota/platform/openai"
	"ota/scheduler"
	"ota/storage"
)

// levelServiceAdapter bridges level.Service to delivery.LevelProvider.
type levelServiceAdapter struct {
	svc *level.Service
}

func (a *levelServiceAdapter) GetLevel(ctx context.Context, userID string) (delivery.UserLevelInfo, error) {
	info, err := a.svc.GetLevel(ctx, userID)
	if err != nil {
		return delivery.UserLevelInfo{}, err
	}
	return delivery.UserLevelInfo{
		Level:       info.Level,
		TotalCoins:  info.TotalCoins,
		DailyLimit:  info.DailyLimit,
		CoinCap:     info.CoinCap,
		Thresholds:  info.Thresholds,
		Description: info.Description,
	}, nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

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
	var fallbackAIClient collector.AIClient
	switch cfg.AIProvider {
	case "gemini":
		aiClient = gemini.NewClient(cfg.GeminiAPIKey, cfg.GeminiModel)
		if cfg.GeminiModelFallback != "" {
			fallbackAIClient = gemini.NewClient(cfg.GeminiAPIKey, cfg.GeminiModelFallback)
		}
	case "openai":
		aiClient = openai.NewClient(cfg.OpenAIAPIKey, cfg.OpenAIModel)
	}
	collectorRepo := storage.NewCollectorRepository(pool)
	collectorService := collector.NewService(aiClient, collectorRepo)
	if fallbackAIClient != nil {
		collectorService.WithFallback(fallbackAIClient)
		log.Printf("collector service initialized (provider: %s, model: %s, fallback: %s)", cfg.AIProvider, cfg.GeminiModel, cfg.GeminiModelFallback)
	} else {
		log.Printf("collector service initialized (provider: %s)", cfg.AIProvider)
	}

	earnCache, err := cache.New(10_000)
	if err != nil {
		log.Fatalf("failed to create cache: %v", err)
	}
	defer earnCache.Close()

	// Structured source collectors (Google Trends + Google News)
	trendsCollector := googletrends.NewCollector()
	newsCollector := googlenews.NewCollector(googlenews.DefaultTopics())
	aggregator := collector.NewAggregator([]collector.SourceCollector{trendsCollector, newsCollector})
	trendingRepo := storage.NewTrendingItemRepository(pool)
	collectorService.WithAggregator(aggregator).WithTrendingRepo(trendingRepo).WithURLDecoder(googlenews.ReplaceArticleURLs)
	log.Println("structured source pipeline initialized (google_trends + google_news)")

	// Brain categories (for AI prompt + admin management)
	brainCategoryRepo := storage.NewBrainCategoryRepository(pool)
	collectorService.WithBrainCategoryRepo(brainCategoryRepo)

	// Image generation (optional — only if model is configured)
	const imageBaseDir = "data/images"
	if cfg.ImageGenerationModel != "" {
		imgClient, imgErr := gemini.NewImageClient(ctx, cfg.GeminiAPIKey, cfg.ImageGenerationModel)
		if imgErr != nil {
			log.Printf("warning: image generation disabled — %v", imgErr)
		} else {
			imageGen := collector.NewImageGenerator(imgClient, imageBaseDir)
			collectorService.WithImageGenerator(imageGen)
			log.Printf("image generation initialized (model: %s)", cfg.ImageGenerationModel)
		}
	} else {
		log.Printf("warning: IMAGE_GENERATION_MODEL not set — thumbnail generation disabled")
	}

	// Message delivery
	emailSender := email.NewSMTPSender(email.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
	})
	deliveryRepo := storage.NewDeliveryRepository(pool)
	collectorAdapter := storage.NewCollectorServiceAdapter(pool)
	deliveryService := delivery.NewService(deliveryRepo, emailSender, collectorAdapter, brainCategoryRepo, cfg.FrontendURL)
	log.Println("delivery service initialized")

	// Level
	levelRepo := storage.NewLevelRepository(pool)
	levelCfg := level.NewLevelConfig(cfg.CoinCap, cfg.CoinsPerLevel)
	levelService := level.NewService(levelRepo, levelCfg, cfg.DailyCoinLimit, cfg.ExtraCoinLimitPerLevel)

	// Withdrawal
	withdrawalRepo := storage.NewWithdrawalRepository(pool)
	coinManager := withdrawal.NewCoinManager(
		func(ctx context.Context, userID string) (int, error) {
			uc, err := levelRepo.GetUserCoins(ctx, userID)
			if err != nil {
				return 0, err
			}
			return uc.Coins, nil
		},
		levelRepo.DeductCoins,
		levelRepo.RestoreCoins,
	)
	withdrawalService := withdrawal.NewService(withdrawalRepo, coinManager, cfg.MinWithdrawalAmount)

	// Scheduler
	sched := scheduler.New(collectorService, deliveryService)
	if err := sched.Start(); err != nil {
		log.Fatalf("failed to start scheduler: %v", err)
	}
	defer sched.Stop()
	log.Println("scheduler started (collection 4-6 AM, delivery 7:00-7:15 AM, retry 7:30-8:30 AM KST)")

	// Terms of service
	termsRepo := storage.NewTermsRepository(pool)
	termsService := terms.NewService(termsRepo)
	signupCache, err := cache.New(1_000)
	if err != nil {
		log.Fatalf("failed to create signup cache: %v", err)
	}
	defer signupCache.Close()

	// Handlers
	userRepo := storage.NewUserRepository(pool)
	subscriptionRepo := storage.NewSubscriptionRepository(pool)
	kakaoClient := kakao.NewClient(cfg.KakaoClientID, cfg.KakaoClientSecret, cfg.KakaoRedirectURI)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)
	stateStore := auth.NewStateStore()
	authHandler := handler.NewAuthHandler(kakaoClient, jwtManager, stateStore, userRepo, deliveryService, levelRepo, cfg.SignupBonusCoins, cfg.FrontendURL, signupCache, termsService)
	termsHandler := handler.NewTermsHandler(termsService)
	termsAdminHandler := handler.NewTermsAdminHandler(termsService)
	brainCategoryHandler := handler.NewBrainCategoryHandler(brainCategoryRepo)
	deliveryHandler := api.NewDeliveryHandler(deliveryService, api.AuthMiddleware(jwtManager))
	userDeliveryChannelsHandler := handler.NewUserDeliveryChannelsHandler(deliveryRepo, deliveryService, userRepo)
	subscriptionHandler := handler.NewSubscriptionHandler(subscriptionRepo, api.AuthMiddleware(jwtManager))

	// Email verification
	emailVerificationRepo := storage.NewEmailVerificationRepository(pool)
	emailVerificationService := user.NewEmailVerificationService(emailVerificationRepo, userRepo)
	emailVerificationHandler := handler.NewEmailVerificationHandler(emailVerificationService, emailSender)

	// Context history
	historyRepo := storage.NewHistoryRepository(pool)
	contextHistoryHandler := handler.NewContextHistoryHandler(historyRepo, api.AuthMiddleware(jwtManager))

	// Level
	earnMinDuration := time.Duration(cfg.EarnMinDurationSec) * time.Second
	levelHandler := handler.NewLevelHandler(
		levelService,
		historyRepo,
		subscriptionRepo,
		earnCache,
		earnMinDuration,
		cfg.TurnstileSecretKey,
		api.AuthMiddleware(jwtManager),
	)
	deliveryService.WithLevelProvider(&levelServiceAdapter{svc: levelService})

	withdrawalHandler := handler.NewWithdrawalHandler(withdrawalService, api.AuthMiddleware(jwtManager))
	withdrawalAdminHandler := handler.NewWithdrawalAdminHandler(withdrawalService)

	mypageHandler := handler.NewMypageHandler(levelService, api.AuthMiddleware(jwtManager))

	adminCoinHandler := handler.NewAdminCoinHandler(userRepo, levelService)

	adminHandler := handler.NewAdminHandler(collectorService, cfg.SlackWebhookURL, brainCategoryHandler).
		WithLevelService(levelService).
		WithMockItemCreator(levelRepo).
		WithDeliveryService(deliveryService)

	// Router
	r := api.NewRouter("api", "v1", cfg.FrontendURL, jwtManager, []api.RouteModule{
		{
			GroupName:   "auth",
			Handler:     authHandler,
			Middlewares: []gin.HandlerFunc{},
		},
		{
			GroupName:   "admin",
			Handler:     adminHandler,
			Middlewares: []gin.HandlerFunc{api.AuthMiddleware(jwtManager), api.AdminMiddleware(userRepo)},
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
		{
			GroupName:   "brain-categories",
			Handler:     brainCategoryHandler,
			Middlewares: []gin.HandlerFunc{},
		},
		{
			GroupName:   "level",
			Handler:     levelHandler,
			Middlewares: []gin.HandlerFunc{},
		},
		{
			GroupName:   "withdrawal",
			Handler:     withdrawalHandler,
			Middlewares: []gin.HandlerFunc{},
		},
		{
			GroupName:   "admin/withdrawals",
			Handler:     withdrawalAdminHandler,
			Middlewares: []gin.HandlerFunc{api.AuthMiddleware(jwtManager), api.AdminMiddleware(userRepo)},
		},
		{
			GroupName:   "terms",
			Handler:     termsHandler,
			Middlewares: []gin.HandlerFunc{},
		},
		{
			GroupName:   "admin/terms",
			Handler:     termsAdminHandler,
			Middlewares: []gin.HandlerFunc{api.AuthMiddleware(jwtManager), api.AdminMiddleware(userRepo)},
		},
		{
			GroupName:   "mypage",
			Handler:     mypageHandler,
			Middlewares: []gin.HandlerFunc{},
		},
		{
			GroupName:   "admin/coins",
			Handler:     adminCoinHandler,
			Middlewares: []gin.HandlerFunc{api.AuthMiddleware(jwtManager), api.AdminMiddleware(userRepo)},
		},
	})

	// Serve generated images from local disk
	r.Static("/api/v1/images", imageBaseDir)

	log.Printf("server starting on :%s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
