package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"RepoUniqueNormalisedIdentifier/config"
	"RepoUniqueNormalisedIdentifier/handlers"
	"RepoUniqueNormalisedIdentifier/health"
	"RepoUniqueNormalisedIdentifier/middleware"
	"RepoUniqueNormalisedIdentifier/observability"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// @title RepoUniqueNormalisedIdentifier
// @description A minimal, stateless API scaffold built with Go and Gin
// @version 1.0
// @host localhost:<port-number>
// @BasePath /

const (
	serviceName    = "RepoUniqueNormalisedIdentifier"
	serviceVersion = "1.0.0"
)

func main() {
	ctx := context.Background()

	// Bootstrap logger so even config-load warnings are structured JSON. Level +
	// environment come straight from the env; the fully config-driven logger is
	// built once config + telemetry are up. No file/OTel sinks yet (bootstrap).
	bootstrapLogger, _ := observability.NewLogger(observability.LogConfig{
		Level:          os.Getenv("LOG_LEVEL"),
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		Environment:    os.Getenv("ENVIRONMENT"),
	})
	slog.SetDefault(bootstrapLogger)

	// Load configuration (defaults < config.{env}.json < env vars).
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Set Gin mode (G14). Honour the GIN_MODE env var injected per-environment by
	// the Helm values (release in deployed envs, debug locally); fall back to the
	// environment name when GIN_MODE is unset. This is the Go equivalent of
	// Serilog's Microsoft/System level overrides — it silences gin's per-route noise.
	if ginMode := os.Getenv("GIN_MODE"); ginMode != "" {
		gin.SetMode(ginMode)
	} else if cfg.Environment == "development" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialise OpenTelemetry → Azure Monitor (traces + metrics + logs, GO-D7/G6).
	// Auto-disabled when no connection string / OTLP endpoint is configured (e.g.
	// local development). otelEnabled controls whether the logger also ships to
	// App Insights.
	otelShutdown, otelEnabled, err := observability.Init(ctx, observability.Config{
		ServiceName:              serviceName,
		ServiceVersion:           serviceVersion,
		Environment:              cfg.Environment,
		AppInsightsConnectionStr: cfg.Monitoring.AzureApplicationInsightsConnectionString,
		Logger:                   slog.Default(),
	})
	if err != nil {
		slog.Error("Failed to initialise telemetry; continuing without it", "error", err)
		otelShutdown = func(context.Context) error { return nil }
		otelEnabled = false
	}

	// Final config-driven logger fanned out across all sinks: console (stdout) +
	// daily-rolling files + (when enabled) Azure Monitor. Level from config,
	// enriched with service + environment.
	logger, logCleanup := observability.NewLogger(observability.LogConfig{
		Level:          cfg.LogLevel,
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		Environment:    cfg.Environment,
		OTelEnabled:    otelEnabled,
		FileEnabled:    cfg.Logging.FileEnabled,
		FileDirectory:  cfg.Logging.FileDirectory,
		RetentionDays:  cfg.Logging.RetentionDays,
	})
	slog.SetDefault(logger)
	defer logCleanup()

	// The health test: straight away and then once a minute it tests every check
	// given here, and the health addresses answer from its last result. A new app
	// has nothing to test. Add a check when the app gains something it cannot work
	// without, for example its database:
	//
	//	health.NewTester(logger, health.Check{Name: "database", Test: database.PingContext})
	healthTester := health.NewTester(logger)
	healthTestContext, stopHealthTest := context.WithCancel(ctx)
	go healthTester.Run(healthTestContext)

	// Initialize Gin router
	router := newRouter(cfg, logger, healthTester)

	// Create HTTP server with proper timeouts
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting HTTP server",
			"port", cfg.Port,
			"environment", cfg.Environment,
			"service", serviceName,
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start HTTP server", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	stopHealthTest()

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown server gracefully
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	} else {
		logger.Info("Server shutdown completed")
	}

	// Flush and stop telemetry
	if err := otelShutdown(shutdownCtx); err != nil {
		logger.Error("Telemetry shutdown error", "error", err)
	}
}

// newRouter builds the gin engine with its middleware and routes.
//
// The middleware order matters. otelgin is outermost: it starts the request's
// span, and every log call made with the request's context below it carries that
// span's trace id. LoggingMiddleware sits outside RecoveryMiddleware, so a panic
// is turned into a 500 before the request line is written and a request that
// crashed still gets its "HTTP request" record.
func newRouter(cfg *config.Config, logger *slog.Logger, healthTester *health.Tester) *gin.Engine {
	router := gin.New()

	// Add middleware. A call to a health address gets no span: Application Insights
	// keeps each request's span as a request record, and the health test writes its
	// own line.
	router.Use(otelgin.Middleware(serviceName, otelgin.WithFilter(func(request *http.Request) bool {
		return !middleware.IsHealthPath(request.URL.Path)
	})))
	router.Use(middleware.LoggingMiddleware(logger, cfg))
	router.Use(middleware.RecoveryMiddleware(logger))
	router.Use(middleware.CORSMiddleware())

	// Health check endpoints (at root level)
	healthRoutes := router.Group("/health")
	{
		healthRoutes.GET("/startup", handlers.StartupHealthCheck(healthTester))
		healthRoutes.GET("/live", handlers.LivenessHealthCheck(healthTester))
		healthRoutes.GET("/ready", handlers.ReadinessHealthCheck(healthTester))
	}

	// API v1 endpoints (at root level)
	v1 := router.Group("/v1")
	{
		v1.GET("/hello", handlers.HelloHandler(cfg, logger))
	}

	return router
}
