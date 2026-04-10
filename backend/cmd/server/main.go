package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	fiberSwagger "github.com/gofiber/swagger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/saas-payment-platform/backend/internal/config"
	"github.com/saas-payment-platform/backend/internal/handler"
	"github.com/saas-payment-platform/backend/internal/middleware"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/internal/pkg/logger"
	"github.com/saas-payment-platform/backend/internal/repository"
	"github.com/saas-payment-platform/backend/internal/service"
	"github.com/saas-payment-platform/backend/internal/worker"

	_ "github.com/saas-payment-platform/backend/docs"
)

// @title           SaaS Payment Platform API
// @version         1.0
// @description     SaaS Payment Platform — API untuk developer melakukan integrasi pembayaran. Menyediakan autentikasi JWT, manajemen API key, simulasi payment gateway, webhook notification, subscription billing, invoice management, rate limiting, dan admin dashboard.
// @termsOfService  https://example.com/terms

// @contact.name   Platform Support
// @contact.url    https://example.com/support
// @contact.email  support@example.com

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter your JWT token with the `Bearer ` prefix, e.g. "Bearer eyJhbGci..."

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description Enter your API key, e.g. "sk_test_abc123..."

// Build-time variables injected via ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Load configuration from environment variables.
	cfg := config.Load()

	// Setup structured JSON logger.
	logger.Setup()

	slog.Info("starting server",
		"version", Version,
		"build_time", BuildTime,
		"git_commit", GitCommit,
	)

	// ---------------------------------------------------------------
	// 1. Infrastructure: Database + Redis connections
	// ---------------------------------------------------------------
	db, err := database.NewPostgresDB(&cfg.Database)
	if err != nil {
		slog.Error("failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	slog.Info("PostgreSQL connected")

	rdb, err := database.NewRedisClient(&cfg.Redis)
	if err != nil {
		slog.Error("failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	slog.Info("Redis connected")

	// Transaction manager for service-layer DB transactions.
	txManager := database.NewTransactionManager(db)

	// ---------------------------------------------------------------
	// 2. Repositories
	// ---------------------------------------------------------------
	userRepo := repository.NewUserRepository(db)
	apiKeyRepo := repository.NewAPIKeyRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	invoiceRepo := repository.NewInvoiceRepository(db)
	webhookEndpointRepo := repository.NewWebhookEndpointRepository(db)
	webhookDeliveryRepo := repository.NewWebhookDeliveryRepository(db)
	productRepo := repository.NewProductRepository(db)
	planRepo := repository.NewPlanRepository(db)
	subscriptionRepo := repository.NewSubscriptionRepository(db)

	// Stats adapters for admin service.
	txStatsRepo := repository.NewTransactionStatsAdapter(db)
	userStatsRepo := repository.NewUserStatsAdapter(db)
	webhookStatsRepo := repository.NewWebhookDeliveryStatsAdapter(db)

	// ---------------------------------------------------------------
	// 3. Services
	// ---------------------------------------------------------------
	userService := service.NewUserService(userRepo, txManager, cfg.JWT.Secret, cfg.JWT.ExpirationHours)
	apiKeyService := service.NewAPIKeyService(apiKeyRepo, txManager)
	webhookService := service.NewWebhookService(webhookEndpointRepo, webhookDeliveryRepo, txManager)
	transactionService := service.NewTransactionService(transactionRepo, txManager, webhookService)
	invoiceService := service.NewInvoiceService(invoiceRepo, txManager)
	subscriptionService := service.NewSubscriptionService(productRepo, planRepo, subscriptionRepo, transactionService, invoiceService, txManager)
	adminService := service.NewAdminService(txStatsRepo, userStatsRepo, webhookStatsRepo)

	// ---------------------------------------------------------------
	// 4. Handlers
	// ---------------------------------------------------------------
	userHandler := handler.NewUserHandler(userService)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyService)
	transactionHandler := handler.NewTransactionHandler(transactionService)
	invoiceHandler := handler.NewInvoiceHandler(invoiceService)
	webhookHandler := handler.NewWebhookHandler(webhookService)
	subscriptionHandler := handler.NewSubscriptionHandler(subscriptionService)
	adminHandler := handler.NewAdminHandler(adminService)

	// ---------------------------------------------------------------
	// 5. Middlewares
	// ---------------------------------------------------------------
	authMW := middleware.NewAuthMiddleware(cfg.JWT.Secret)
	apiKeyMW := middleware.NewAPIKeyMiddleware(apiKeyService)
	redisAdapter := middleware.NewGoRedisAdapter(rdb)
	rateLimiter := middleware.NewRateLimiter(redisAdapter, cfg.RateLimit.RequestsPerHour, cfg.RateLimit.WindowSize)

	// ---------------------------------------------------------------
	// 6. Fiber app + base middlewares
	// ---------------------------------------------------------------
	app := fiber.New(fiber.Config{
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		AppName:      fmt.Sprintf("saas-payment-platform %s", Version),
	})

	app.Use(middleware.RequestID())
	app.Use(middleware.CORS("*"))
	app.Use(middleware.Logger())

	// Operational endpoints (no auth).
	registerHealthEndpoints(app, db, rdb)

	// Swagger UI.
	app.Get("/swagger/*", fiberSwagger.HandlerDefault)

	// ---------------------------------------------------------------
	// 7. API v1 routes
	// ---------------------------------------------------------------
	v1 := app.Group("/api/v1")

	// Apply rate limiter to all API v1 routes.
	v1.Use(rateLimiter.Limit())

	// Public auth routes (no JWT/API key required).
	userHandler.RegisterRoutes(v1, authMW)

	// JWT-protected routes.
	apiKeyHandler.RegisterRoutes(v1, authMW)
	invoiceHandler.RegisterRoutes(v1, authMW)
	webhookHandler.RegisterRoutes(v1, authMW)
	subscriptionHandler.RegisterRoutes(v1, authMW)

	// API Key-protected routes.
	transactionHandler.RegisterRoutes(v1, apiKeyMW)

	// Admin-only routes (JWT + admin role).
	adminHandler.RegisterRoutes(v1, authMW)

	// ---------------------------------------------------------------
	// 8. Webhook Worker (background goroutine)
	// ---------------------------------------------------------------
	webhookWorkerCfg := worker.DefaultWebhookWorkerConfig()
	webhookWorker := worker.NewWebhookWorker(webhookService, webhookDeliveryRepo, webhookEndpointRepo, webhookWorkerCfg)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	go func() {
		slog.Info("starting webhook worker")
		if err := webhookWorker.Start(workerCtx); err != nil {
			slog.Error("webhook worker error", "error", err)
		}
	}()

	// ---------------------------------------------------------------
	// 9. Graceful shutdown
	// ---------------------------------------------------------------
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		addr := fmt.Sprintf(":%s", cfg.Server.Port)
		slog.Info("listening", "addr", addr)
		if err := app.Listen(addr); err != nil {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	sig := <-quit
	slog.Info("shutting down", "signal", sig.String())

	// 1. Stop accepting new requests.
	if err := app.Shutdown(); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	// 2. Stop webhook worker (waits for in-flight deliveries).
	slog.Info("stopping webhook worker")
	workerCancel()
	webhookWorker.Stop()
	slog.Info("webhook worker stopped")

	// 3. Close Redis connection.
	if err := rdb.Close(); err != nil {
		slog.Error("redis close error", "error", err)
	}

	// 4. Close database connection.
	sqlDB, err := db.DB()
	if err == nil {
		if err := sqlDB.Close(); err != nil {
			slog.Error("database close error", "error", err)
		}
	}

	slog.Info("server stopped")
}

// registerHealthEndpoints wires the /health, /ready, and /metrics endpoints.
func registerHealthEndpoints(app *fiber.App, db *gorm.DB, rdb *redis.Client) {
	// Liveness — returns 200 if the process is alive.
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":     "ok",
			"version":    Version,
			"build_time": BuildTime,
			"git_commit": GitCommit,
		})
	})

	// Readiness — checks PostgreSQL and Redis connectivity.
	app.Get("/ready", func(c *fiber.Ctx) error {
		checks := fiber.Map{}
		ready := true

		// PostgreSQL check.
		if db != nil {
			if err := database.PostgresHealthCheck(db); err != nil {
				checks["postgres"] = err.Error()
				ready = false
			} else {
				checks["postgres"] = "ok"
			}
		} else {
			checks["postgres"] = "not_configured"
			ready = false
		}

		// Redis check.
		if rdb != nil {
			ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
			defer cancel()
			if err := rdb.Ping(ctx).Err(); err != nil {
				checks["redis"] = err.Error()
				ready = false
			} else {
				checks["redis"] = "ok"
			}
		} else {
			checks["redis"] = "not_configured"
			ready = false
		}

		status := fiber.StatusOK
		if !ready {
			status = fiber.StatusServiceUnavailable
		}

		return c.Status(status).JSON(fiber.Map{
			"status": map[bool]string{true: "ready", false: "not_ready"}[ready],
			"checks": checks,
		})
	})

	// Prometheus-compatible metrics endpoint.
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))
}
