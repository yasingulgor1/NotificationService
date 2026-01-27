package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/insider-one/notification-service/internal/config"
	"github.com/insider-one/notification-service/internal/domain"
	"github.com/insider-one/notification-service/internal/handler"
	"github.com/insider-one/notification-service/internal/middleware"
	"github.com/insider-one/notification-service/internal/provider"
	"github.com/insider-one/notification-service/internal/repository/postgres"
	"github.com/insider-one/notification-service/internal/repository/redis"
	"github.com/insider-one/notification-service/internal/service"
	"github.com/insider-one/notification-service/internal/worker"
)

// @title Notification Service API
// @version 1.0
// @description Event-Driven Notification System API
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@insider.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key

func main() {
	// Load configuration
	cfg := config.Load()

	// Setup logger
	logLevel := slog.LevelInfo
	if cfg.App.LogLevel == "debug" {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	logger.Info("starting notification service",
		"env", cfg.App.Env,
		"port", cfg.Server.Port,
	)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize PostgreSQL
	db, err := postgres.New(ctx, cfg.Database)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("connected to PostgreSQL")

	// Initialize Redis
	redisClient, err := redis.New(ctx, cfg.Redis)
	if err != nil {
		logger.Error("failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()
	logger.Info("connected to Redis")

	// Initialize repositories
	notificationRepo := postgres.NewNotificationRepository(db)
	templateRepo := postgres.NewTemplateRepository(db)
	queue := redis.NewQueue(redisClient)
	rateLimiter := redis.NewRateLimiter(redisClient, cfg.Worker.RateLimitPerSec)

	// Initialize provider
	webhookProvider := provider.NewWebhookProvider(cfg.Webhook)

	// Initialize services
	templateService := service.NewTemplateService(templateRepo, logger)
	notificationService := service.NewNotificationService(notificationRepo, templateRepo, queue, logger)
	schedulerService := service.NewSchedulerService(notificationRepo, queue, logger, cfg.Worker.SchedulerInterval)

	// Initialize WebSocket hub
	wsHub := handler.NewWebSocketHub(logger)
	go wsHub.Run()

	// Set up status broadcast
	statusBroadcast := func(n *domain.Notification) {
		wsHub.BroadcastStatus(n)
	}
	notificationService.SetStatusBroadcast(statusBroadcast)

	// Initialize worker processor
	processor := worker.NewProcessor(
		notificationRepo,
		queue,
		rateLimiter,
		webhookProvider,
		logger,
		cfg.Retry,
		cfg.Worker,
	)
	processor.SetStatusBroadcast(statusBroadcast)

	// Initialize handlers
	notificationHandler := handler.NewNotificationHandler(notificationService)
	templateHandler := handler.NewTemplateHandler(templateService)
	healthHandler := handler.NewHealthHandler()
	healthHandler.AddChecker("postgres", db)
	healthHandler.AddChecker("redis", redisClient)

	metrics := handler.NewMetrics()
	metricsHandler := handler.NewMetricsHandler(metrics, queue)
	wsHandler := handler.NewWebSocketHandler(wsHub)

	// Setup router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(middleware.Correlation)
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.Logging(logger))
	r.Use(chimiddleware.Compress(5))

	// Health endpoints
	r.Get("/health", healthHandler.Health)
	r.Get("/health/live", healthHandler.Liveness)
	r.Get("/health/ready", healthHandler.Readiness)

	// Metrics endpoints
	r.Handle("/metrics", metricsHandler.Handler())
	r.Get("/metrics/realtime", metricsHandler.RealtimeMetrics)

	// WebSocket endpoint
	r.Get("/ws", wsHandler.HandleWebSocket)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/notifications", func(r chi.Router) {
			notificationHandler.RegisterRoutes(r)
		})

		r.Route("/templates", func(r chi.Router) {
			templateHandler.RegisterRoutes(r)
		})
	})

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start worker processor
	if err := processor.Start(ctx); err != nil {
		logger.Error("failed to start processor", "error", err)
		os.Exit(1)
	}

	// Start scheduler
	if err := schedulerService.Start(ctx); err != nil {
		logger.Error("failed to start scheduler", "error", err)
		os.Exit(1)
	}

	// Start server in goroutine
	go func() {
		logger.Info("server listening", "port", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	// Stop accepting new requests
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	// Stop scheduler
	schedulerService.Stop()

	// Stop processor (waits for in-flight work)
	processor.Stop()

	// Cancel context
	cancel()

	logger.Info("server stopped")
}
