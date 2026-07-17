package middleware

import (
	"log/slog"
	"net"
	"os"
	"time"

	"RepoUniqueNormalisedIdentifier/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// CorrelationIDHeader carries a per-request id in and out of the service so
	// logs and downstream calls can be stitched together (the Go analogue of the
	// .NET LogContextEnrichment CorrelationId).
	CorrelationIDHeader = "X-Correlation-Id"
	// CorrelationIDKey is the gin context key the id is stored under.
	CorrelationIDKey = "correlation_id"
)

// LoggingMiddleware logs each request completion with slog, enriched with a
// correlation id (taken from the inbound header or generated, and echoed on the
// response) — the Go analogue of .NET's LogContextEnrichment +
// UseSerilogRequestLogging.
//
// The attribute names are a CONTRACT with the Pellerex portal's Logs tab: its
// KQL reads customDimensions.RequestMethod / RequestPath / StatusCode /
// Elapsed / CorrelationId / Environment / UserAgent / ClientIPAddress /
// ClientPort / Port / MachineName by exact name. Renaming any of them blanks
// the matching column in the portal.
func LoggingMiddleware(logger *slog.Logger, cfg *config.Config) gin.HandlerFunc {
	machineName, _ := os.Hostname()

	return func(c *gin.Context) {
		start := time.Now()

		correlationID := c.GetHeader(CorrelationIDHeader)
		if correlationID == "" {
			correlationID = uuid.NewString()
		}
		c.Set(CorrelationIDKey, correlationID)
		c.Writer.Header().Set(CorrelationIDHeader, correlationID)

		c.Next()

		_, clientPort, _ := net.SplitHostPort(c.Request.RemoteAddr)

		logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "HTTP request",
			slog.Int("StatusCode", c.Writer.Status()),
			slog.String("RequestMethod", c.Request.Method),
			slog.String("RequestPath", c.Request.URL.Path),
			slog.Float64("Elapsed", float64(time.Since(start).Microseconds())/1000.0),
			slog.String("CorrelationId", correlationID),
			slog.String("Environment", cfg.Environment),
			slog.String("UserAgent", c.Request.UserAgent()),
			slog.String("ClientIPAddress", c.ClientIP()),
			slog.String("ClientPort", clientPort),
			slog.String("Port", cfg.Port),
			slog.String("MachineName", machineName),
		)
	}
}

// CORSMiddleware creates a CORS middleware with configurable origins
func CORSMiddleware() gin.HandlerFunc {
	config := cors.Config{
		AllowOrigins:     []string{"*"}, // This will be overridden by environment-specific config
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	return cors.New(config)
}

// HealthCheckMiddleware ensures health check endpoints respond quickly
func HealthCheckMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health/live" ||
			c.Request.URL.Path == "/health/ready" ||
			c.Request.URL.Path == "/health/startup" {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}
		c.Next()
	}
}
