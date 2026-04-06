package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
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
	"ota/domain/quiz"
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

	// -- Structured logging (slog) --------------------------------------------
	var slogHandler slog.Handler
	if cfg.AppEnv == "production" {
		slogHandler = slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{Level: slog.LevelInfo})
		gin.SetMode(gin.ReleaseMode)
	} else {
		slogHandler = slog.NewTextHandler(multiWriter, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	slog.SetDefault(slog.New(slogHandler))

	ctx := context.Background()

	if err := storage.RunMigrations(cfg.DatabaseURL(), "migrations"); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("migrations completed")

	pool, err := storage.NewPool(ctx, cfg.DatabaseURL(), storage.PoolConfig{
		MaxConns:        cfg.DBMaxConns,
		MinConns:        cfg.DBMinConns,
		MaxConnLifetime: cfg.DBMaxConnLifetime,
		MaxConnIdleTime: cfg.DBMaxConnIdleTime,
	})
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("database connected")

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

	// Recover stale "running" collection runs from previous unclean shutdown
	if n, err := collectorRepo.FailStaleRuns(ctx); err != nil {
		slog.Error("failed to recover stale collection runs", "error", err)
	} else if n > 0 {
		slog.Warn("recovered stale collection runs", "count", n)
	}

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
		slog.Warn("redis unavailable for rate limit store, using in-memory", "error", err)
		rateLimitStore = memory.NewStore()
	} else {
		rateLimitStore = rls
		slog.Info("rate limit store connected to redis")
	}
	if closer, ok := rateLimitStore.(io.Closer); ok {
		defer closer.Close()
	}

	var earnCache cache.Cache
	var redisPinger *cache.RedisCache // non-nil when Redis is reachable
	if rc, err := cache.NewRedisCache(redisCfg, "earn:"); err != nil {
		slog.Warn("redis unavailable for earn cache, using in-process", "error", err)
		earnCache = cache.NewInProcess()
	} else {
		earnCache = rc
		redisPinger = rc
		slog.Info("earn cache connected to redis")
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
		slog.Info("loaded news sources from DB", "count", len(newsTopics))
	} else {
		slog.Warn("using default news sources", "db_load_error", err)
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
		slog.Error("IMAGE_GENERATION_MODEL is required")
		os.Exit(1)
	}
	imgClient, imgErr := gemini.NewImageClient(ctx, cfg.GeminiAPIKey, cfg.ImageGenerationModel)
	if imgErr != nil {
		slog.Error("failed to initialize image generation", "error", imgErr)
		os.Exit(1)
	}
	imageGen := collector.NewImageGenerator(imgClient, imageBaseDir)
	slog.Info("image generation initialized", "model", cfg.ImageGenerationModel)

	articleFetcher := collector.NewHTTPArticleFetcher()
	collectorService := collector.NewService(aiClient, collectorRepo, aggregator, trendingRepo, brainCategoryRepo, googlenews.ReplaceArticleURLs, articleFetcher, imageGen)
	collectorService.WithCategoryRepo(categoryRepo)
	if fallbackAIClient != nil {
		collectorService.WithFallback(fallbackAIClient)
		slog.Info("collector service initialized", "provider", cfg.AIProvider, "model", cfg.GeminiModel, "fallback", cfg.GeminiModelFallback)
	} else {
		slog.Info("collector service initialized", "provider", cfg.AIProvider)
	}
	slog.Info("structured source pipeline initialized", "sources", "google_trends+google_news")

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
	slog.Info("delivery service initialized")

	// Level
	levelRepo := storage.NewLevelRepository(pool)
	levelCfg := level.NewLevelConfig(cfg.CoinCap, cfg.CoinsPerLevel)
	levelService := level.NewService(levelRepo, levelCfg, cfg.DailyCoinLimit, cfg.ExtraCoinLimitPerLevel)

	// Withdrawal
	withdrawalRepo := storage.NewWithdrawalRepository(pool).WithEncryptionKey(cfg.BankAccountEncryptionKey)
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
	withdrawalService := withdrawal.NewService(withdrawalRepo, coinManager, cfg.MinWithdrawalAmount, cfg.WithdrawalUnitAmount)

	// Scheduler — shutdownCtx is cancelled on SIGINT/SIGTERM so in-progress
	// collection gets cancelled and its run is marked as failed.
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	sched := scheduler.New(collectorService, deliveryService, shutdownCtx)
	if err := sched.Start(); err != nil {
		slog.Error("failed to start scheduler", "error", err)
		os.Exit(1)
	}
	defer sched.Stop()
	slog.Info("scheduler started", "schedule", "collection 4-6AM delivery 7AM retry 7:30-8:30AM KST")

	// Terms of service
	termsRepo := storage.NewTermsRepository(pool)
	termsService := terms.NewService(termsRepo)
	var signupCache cache.Cache
	if rc, err := cache.NewRedisCache(redisCfg, "signup:"); err != nil {
		slog.Warn("redis unavailable for signup cache, using in-process", "error", err)
		signupCache = cache.NewInProcess()
	} else {
		signupCache = rc
		slog.Info("signup cache connected to redis")
	}
	defer signupCache.Close()

	// Refresh token repository
	refreshTokenRepo := storage.NewRefreshTokenRepository(pool)

	// Handlers
	userRepo := storage.NewUserRepository(pool)
	subscriptionRepo := storage.NewSubscriptionRepository(pool)
	kakaoClient := kakao.NewClient(cfg.KakaoClientID, cfg.KakaoClientSecret, cfg.KakaoRedirectURI)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)
	// OAuth state store: Redis-backed when available, in-memory fallback
	var stateStore auth.StateStorer
	redisAddr := fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort)
	if rs, err := auth.NewRedisStateStore(redisAddr, cfg.RedisPassword, cfg.RedisDB); err != nil {
		slog.Warn("redis unavailable for oauth state store, using in-memory", "error", err)
		stateStore = auth.NewStateStore()
	} else {
		stateStore = rs
		slog.Info("oauth state store connected to redis")
	}
	authHandler := handler.NewAuthHandler(kakaoClient, jwtManager, stateStore, userRepo, deliveryService, levelRepo, cfg.SignupBonusCoins, cfg.FrontendURL, signupCache, termsService).
		WithWithdrawalChecker(withdrawalRepo).
		WithRefreshTokenStore(refreshTokenRepo)
	termsHandler := handler.NewTermsHandler(termsService)
	termsAdminHandler := handler.NewTermsAdminHandler(termsService)
	brainCategoryHandler := handler.NewBrainCategoryHandler(brainCategoryRepo)
	deliveryHandler := api.NewDeliveryHandler(deliveryService, api.AuthMiddleware(jwtManager), api.AdminMiddleware(userRepo))
	userDeliveryChannelsHandler := handler.NewUserDeliveryChannelsHandler(deliveryRepo, deliveryService, userRepo)
	subscriptionHandler := handler.NewSubscriptionHandler(subscriptionRepo, api.AuthMiddleware(jwtManager))

	// Email verification
	emailVerificationRepo := storage.NewEmailVerificationRepository(pool)
	emailVerificationService := user.NewEmailVerificationService(emailVerificationRepo, userRepo)
	emailVerificationHandler := handler.NewEmailVerificationHandler(emailVerificationService, emailSender)

	// Quiz
	quizRepo := storage.NewQuizRepository(pool)
	quizService := quiz.NewService(quizRepo, levelRepo, levelCfg, cfg.QuizMaxBonusCoins)
	collectorService.WithQuizRepo(quizRepo)

	// Context history
	historyRepo := storage.NewHistoryRepository(pool)
	contextHistoryHandler := handler.NewContextHistoryHandler(historyRepo, api.AuthMiddleware(jwtManager))
	contextHistoryHandler.WithCategoryRepo(categoryRepo, brainCategoryRepo)
	contextHistoryHandler.WithQuizService(quizService, api.OptionalAuthMiddleware(jwtManager))

	// Level
	earnMinDuration := time.Duration(cfg.EarnMinDurationSec) * time.Second
	levelHandler := handler.NewLevelHandler(
		levelService,
		historyRepo,
		subscriptionRepo,
		earnCache,
		cfg.EarnCacheRetries,
		earnMinDuration,
		cfg.TurnstileSecretKey,
		api.AuthMiddleware(jwtManager),
	)
	deliveryService.WithLevelProvider(&levelServiceAdapter{svc: levelService})
	levelHandler.WithQuizStatusGetter(quizRepo)

	quizHandler := handler.NewQuizHandler(quizService, historyRepo, api.AuthMiddleware(jwtManager))

	withdrawalHandler := handler.NewWithdrawalHandler(withdrawalService, api.AuthMiddleware(jwtManager))
	withdrawalAdminHandler := handler.NewWithdrawalAdminHandler(withdrawalService)

	mypageHandler := handler.NewMypageHandler(levelService, api.AuthMiddleware(jwtManager))

	adminCoinHandler := handler.NewAdminCoinHandler(userRepo, levelService)

	adminHandler := handler.NewAdminHandler(collectorService, cfg.SlackWebhookURL, brainCategoryHandler).
		WithLevelService(levelService).
		WithMockItemCreator(levelRepo).
		WithDeliveryService(deliveryService)

	healthHandler := handler.NewHealthHandler(pool)
	if redisPinger != nil {
		healthHandler = healthHandler.WithRedisPinger(redisPinger)
	}

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
		{
			GroupName:   "quiz",
			Handler:     quizHandler,
			Middlewares: []gin.HandlerFunc{},
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
		slog.Info("server starting", "port", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server listen error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutdown signal received, draining requests")

	// Cancel scheduler's parent context so in-progress collection is
	// interrupted and its run is marked as failed via failRun().
	shutdownCancel()

	httpShutdownCtx, httpCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer httpCancel()
	if err := srv.Shutdown(httpShutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server exited cleanly")
}
