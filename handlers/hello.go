package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"RepoUniqueNormalisedIdentifier/config"
	"RepoUniqueNormalisedIdentifier/secrets"

	"github.com/gin-gonic/gin"
)

// HelloResponse represents the response structure for the hello endpoint
// This matches the structure used in .NET, Node.js, and Python scaffolds
type HelloResponse struct {
	Message       string                 `json:"message"`
	Timestamp     string                 `json:"timestamp"`
	Version       string                 `json:"version"`
	Environment   string                 `json:"environment"`
	Score         int                    `json:"score"` // Matches .NET scaffold
	Configuration map[string]interface{} `json:"configuration"`
	// Reports only whether the Key Vault DbConnectionString reached the CSI
	// mount — the secret value itself is never echoed back.
	DbConnectionStringConfigured bool `json:"dbConnectionStringConfigured"`
}

// HelloHandler handles the /v1/hello endpoint
// @Summary Hello endpoint
// @Description Returns hello message with configuration information and secret preview
// @Tags API
// @Produce json
// @Success 200 {object} HelloResponse
// @Router /v1/hello [get]
func HelloHandler(cfg *config.Config, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Build configuration information
		configInfo := map[string]interface{}{
			"debug":                cfg.Debug,
			"cors_allowed_origins": cfg.CORSAllowedOrigins,
			"log_level":            cfg.LogLevel,
		}

		// Build response matching the pattern from other minimal scaffolds
		response := HelloResponse{
			Message:                      "Hello from RepoUniqueNormalisedIdentifier!",
			Timestamp:                    time.Now().Format(time.RFC3339),
			Version:                      "v1",
			Environment:                  cfg.Environment,
			Score:                        110, // Matches the .NET scaffold
			Configuration:                configInfo,
			DbConnectionStringConfigured: secrets.HasDatabaseSecret(cfg.Secrets),
		}

		// Log the request
		logger.Info("Hello endpoint accessed",
			"endpoint", "/v1/hello",
			"version", "1.0",
			"client_ip", c.ClientIP(),
		)

		c.JSON(http.StatusOK, response)
	}
}
