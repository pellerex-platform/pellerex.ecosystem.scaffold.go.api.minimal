package handlers

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"RepoUniqueNormalisedIdentifier/config"
	"RepoUniqueNormalisedIdentifier/secrets"

	"github.com/gin-gonic/gin"
)

// HealthResponse represents the structure of health check responses
type HealthResponse struct {
	Status    string                 `json:"status"`
	Timestamp string                 `json:"timestamp"`
	Service   string                 `json:"service"`
	Version   string                 `json:"version"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// StartupHealthCheck handles startup health checks
// @Summary Startup health check
// @Description Checks if the application has started successfully
// @Tags Health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health/startup [get]
func StartupHealthCheck(cfg *config.Config, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Startup check - verify all critical components are initialized
		details := map[string]interface{}{
			"startTime": time.Now().Add(-time.Since(time.Now())).Format(time.RFC3339),
			"uptime":    time.Since(time.Now()).Seconds(),
			"pid":       os.Getpid(),
		}

		response := HealthResponse{
			Status:    "started",
			Timestamp: time.Now().Format(time.RFC3339),
			Service:   "RepoUniqueNormalisedIdentifier",
			Version:   "1.0.0",
			Details:   details,
		}

		logger.Debug("Startup health check accessed")
		c.JSON(http.StatusOK, response)
	}
}

// LivenessHealthCheck handles liveness health checks
// @Summary Liveness health check
// @Description Checks if the application is alive and responsive
// @Tags Health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health/live [get]
func LivenessHealthCheck(cfg *config.Config, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Liveness check - basic responsiveness (no external dependencies)
		response := HealthResponse{
			Status:    "alive",
			Timestamp: time.Now().Format(time.RFC3339),
			Service:   "RepoUniqueNormalisedIdentifier",
			Version:   "1.0.0",
		}

		logger.Debug("Liveness health check accessed")
		c.JSON(http.StatusOK, response)
	}
}

// ReadinessHealthCheck handles readiness health checks
// @Summary Readiness health check
// @Description Checks if the application is ready to serve traffic
// @Tags Health
// @Produce json
// @Success 200 {object} HealthResponse
// @Success 503 {object} HealthResponse
// @Router /health/ready [get]
func ReadinessHealthCheck(cfg *config.Config, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Readiness check - verify dependencies and configuration
		isReady := true
		details := make(map[string]interface{})

		// Check configuration
		if cfg == nil {
			isReady = false
			details["configuration"] = "failed"
		} else {
			details["configuration"] = "ok"
		}

		// Check secrets availability (non-blocking). LoadSecrets never returns nil,
		// so report on the DbConnectionString content, not the struct pointer —
		// "loaded" must mean the Key Vault secret actually arrived on the mount.
		if secrets.HasDatabaseSecret(cfg.Secrets) {
			details["secret_mount"] = "loaded"
		} else {
			details["secret_mount"] = "absent"
			// Note: We don't mark as not ready for secrets in minimal scaffold
		}

		// Check environment configuration
		details["environment"] = cfg.Environment
		details["port"] = cfg.Port

		status := "ready"
		httpStatus := http.StatusOK

		if !isReady {
			status = "not_ready"
			httpStatus = http.StatusServiceUnavailable
		}

		response := HealthResponse{
			Status:    status,
			Timestamp: time.Now().Format(time.RFC3339),
			Service:   "RepoUniqueNormalisedIdentifier",
			Version:   "1.0.0",
			Details:   details,
		}

		logger.Debug("Readiness health check accessed", "ready", isReady)
		c.JSON(httpStatus, response)
	}
}
