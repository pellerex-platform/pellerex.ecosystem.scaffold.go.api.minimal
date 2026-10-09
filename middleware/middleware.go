package middleware

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"RepoUniqueNormalisedIdentifier/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
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

// RecoveryMiddleware turns a panic inside a request into a 500 response and
// writes it to every log sink, tied to the request it happened in. gin's own
// Recovery only prints the panic to stderr, which never reaches Azure Monitor.
// This keeps gin's recovery (a broken client connection is still left alone) and
// replaces what happens next: one error record with the OpenTelemetry exception
// attributes (exception.type / exception.message / exception.stacktrace), logged
// with the request's context so it shares the request's trace id, and the same
// error recorded on the request's span.
//
// Register it AFTER LoggingMiddleware. The panic is then handled before the
// request line is written, so a request that crashed still gets its
// "HTTP request" record, with StatusCode 500.
func RecoveryMiddleware(logger *slog.Logger) gin.HandlerFunc {
	// In debug mode (a developer's machine) gin also keeps printing the panic and
	// its stack to stderr in readable form. Deployed, the structured record is the only copy.
	var readablePanic io.Writer
	if gin.IsDebugging() {
		readablePanic = gin.DefaultErrorWriter
	}

	return gin.CustomRecoveryWithWriter(readablePanic, func(c *gin.Context, recovered any) {
		err, isError := recovered.(error)
		if !isError {
			err = fmt.Errorf("%v", recovered)
		}

		ctx := c.Request.Context()

		// otelgin marks the span as failed from the 500 status, so only the error itself is added here
		trace.SpanFromContext(ctx).RecordError(err, trace.WithStackTrace(true))

		logger.LogAttrs(ctx, slog.LevelError, "Unhandled panic in request",
			slog.String("exception.type", fmt.Sprintf("%T", recovered)),
			slog.String("exception.message", err.Error()),
			slog.String("exception.stacktrace", string(debug.Stack())),
			slog.String("RequestMethod", c.Request.Method),
			slog.String("RequestPath", c.Request.URL.Path),
			slog.String("CorrelationId", c.GetString(CorrelationIDKey)),
		)

		c.AbortWithStatus(http.StatusInternalServerError)
	})
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
