package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"

	limiter "github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"

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

	// -- Log rotation (stdout + file) -----------------------------------------
	fileLogger := &lumberjack.Logger{
		Filename:   "logs/ota.log",
		MaxSize:    100, // MB
		MaxBackups: 10,
		MaxAge:     30, // days
		Compress:   true,
	}
	multiWriter := io.MultiWriter(os.Stdout, fileLogger)
	log.SetOutput(multiWriter)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	gin.DefaultWriter = multiWriter
	gin.DefaultErrorWriter = multiWriter

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

	// -- Redis / in-process cache ------------------------------------------------
	redisCfg := cache.RedisConfig{
		Host:     cfg.RedisHost,
		Port:     cfg.RedisPort,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	// Rate limit store (Redis or in-memory fallback)
	var rateLimitStore limiter.Store
	if rls, err := cache.NewRedisLimiterStore(redisCfg, "ratelimit:"); err != nil {
		log.Printf("redis unavailable for rate limit store, using in-memory: %v", err)
		rateLimitStore = memory.NewStore()
	} else {
		rateLimitStore = rls
		log.Println("rate limit store connected to redis")
	}
	if closer, ok := rateLimitStore.(io.Closer); ok {
		defer closer.Close()
	}

	var earnCache cache.Cache
	if rc, err := cache.NewRedisCache(redisCfg, "earn:"); err != nil {
		log.Printf("redis unavailable for earn cache, using in-process: %v", err)
		earnCache = cache.NewInProcess()
	} else {
		earnCache = rc
		log.Println("earn cache connected to redis")
	}
	defer earnCache.Close()

	// Categories and news sources
	categoryRepo := storage.NewCategoryRepository(pool)

	// Load news sources from DB, fallback to defaults
	newsTopics := googlenews.DefaultTopics()
	if dbSources, err := categoryRepo.GetEnabledNewsSources(ctx); err == nil && len(dbSources) > 0 {
		newsTopics = make([]googlenews.FeedTopic, len(dbSources))
		for i, s := range dbSources {
			newsTopics[i] = googlenews.FeedTopic{Category: s.CategoryKey, URL: s.URL}
		}
		log.Printf("loaded %d news sources from DB", len(newsTopics))
	} else {
		log.Printf("using default news sources (DB load: %v)", err)
	}

	// Structured source collectors (Google Trends + Google News)
	trendsCollector := googletrends.NewCollector()
	newsCollector := googlenews.NewCollector(newsTopics)
	aggregator := collector.NewAggregator(trendsCollector, newsCollector)
	trendingRepo := storage.NewTrendingItemRepository(pool)

	// Brain categories (for AI prompt + admin management)
	brainCategoryRepo := storage.NewBrainCategoryRepository(pool)

	// Image generation
	const imageBaseDir = "data/images"
	if cfg.ImageGenerationModel == "" {
		log.Fatalf("IMAGE_GENERATION_MODEL is required")
	}
	imgClient, imgErr := gemini.NewImageClient(ctx, cfg.GeminiAPIKey, cfg.ImageGenerationModel)
	if imgErr != nil {
		log.Fatalf("failed to initialize image generation: %v", imgErr)
	}
	imageGen := collector.NewImageGenerator(imgClient, imageBaseDir)
	log.Printf("image generation initialized (model: %s)", cfg.ImageGenerationModel)

	articleFetcher := collector.NewHTTPArticleFetcher()
	collectorService := collector.NewService(aiClient, collectorRepo, aggregator, trendingRepo, brainCategoryRepo, googlenews.ReplaceArticleURLs, articleFetcher, imageGen)
	collectorService.WithCategoryRepo(categoryRepo)
	if fallbackAIClient != nil {
		collectorService.WithFallback(fallbackAIClient)
		log.Printf("collector service initialized (provider: %s, model: %s, fallback: %s)", cfg.AIProvider, cfg.GeminiModel, cfg.GeminiModelFallback)
	} else {
		log.Printf("collector service initialized (provider: %s)", cfg.AIProvider)
	}
	log.Println("structured source pipeline initialized (google_trends + google_news)")

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

	// Scheduler + data retention cleanup
	cleanupRepo := storage.NewCleanupRepository(pool)
	sched := scheduler.New(collectorService, deliveryService)
	sched.WithCleanup(cleanupRepo)
	if err := sched.Start(); err != nil {
		log.Fatalf("failed to start scheduler: %v", err)
	}
	defer sched.Stop()
	log.Println("scheduler started (collection 4-6 AM, delivery 7:00-7:15 AM, retry 7:30-8:30 AM, cleanup 3 AM KST)")

	// Terms of service
	termsRepo := storage.NewTermsRepository(pool)
	termsService := terms.NewService(termsRepo)
	var signupCache cache.Cache
	if rc, err := cache.NewRedisCache(redisCfg, "signup:"); err != nil {
		log.Printf("redis unavailable for signup cache, using in-process: %v", err)
		signupCache = cache.NewInProcess()
	} else {
		signupCache = rc
		log.Println("signup cache connected to redis")
	}
	defer signupCache.Close()

	// Refresh token repository
	refreshTokenRepo := storage.NewRefreshTokenRepository(pool)

	// Handlers
	userRepo := storage.NewUserRepository(pool)
	subscriptionRepo := storage.NewSubscriptionRepository(pool)
	kakaoClient := kakao.NewClient(cfg.KakaoClientID, cfg.KakaoClientSecret, cfg.KakaoRedirectURI)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)
	stateStore := auth.NewStateStore()
	authHandler := handler.NewAuthHandler(kakaoClient, jwtManager, stateStore, userRepo, deliveryService, levelRepo, cfg.SignupBonusCoins, cfg.FrontendURL, signupCache, termsService).
		WithWithdrawalChecker(withdrawalRepo).
		WithRefreshTokenStore(refreshTokenRepo)
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
	contextHistoryHandler.WithCategoryRepo(categoryRepo, brainCategoryRepo)

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

	healthHandler := handler.NewHealthHandler(pool)

	// Router
	r := api.NewRouter("api", "v1", cfg.FrontendURL, jwtManager, cfg.RateLimitPerMin, rateLimitStore, []api.RouteModule{
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

	// Health check routes at root level (no auth, no rate limit)
	r.GET("/health", healthHandler.Live)
	r.GET("/health/ready", healthHandler.Ready)

	// Serve generated images from local disk
	r.Static("/api/v1/images", imageBaseDir)

	// -- Graceful shutdown ----------------------------------------------------
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}

	go func() {
		log.Printf("server starting on :%s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server listen error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutdown signal received, draining requests...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server exited cleanly")
}
