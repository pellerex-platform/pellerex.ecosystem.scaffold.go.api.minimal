package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"RepoUniqueNormalisedIdentifier/secrets"

	"github.com/joho/godotenv"
)

// Config represents the application configuration
type Config struct {
	Environment string `json:"environment"`
	Port        string `json:"port"`
	Debug       bool   `json:"debug"`
	LogLevel    string `json:"log_level"`

	// CORS configuration
	CORSAllowedOrigins []string `json:"cors_allowed_origins"`
	CORSAllowedMethods []string `json:"cors_allowed_methods"`
	CORSAllowedHeaders []string `json:"cors_allowed_headers"`

	// Monitoring / observability (App Insights connection string for the
	// OpenTelemetry Azure Monitor exporter — see the observability package, GO-D7)
	Monitoring MonitoringConfig `json:"monitoring"`

	// Logging configures the on-disk log sinks. Console + Azure Monitor are always
	// wired (see the observability package); this controls the file sinks that
	// mirror the .NET Serilog File sinks.
	Logging LoggingConfig `json:"logging"`

	// Secrets (loaded separately)
	Secrets *secrets.AppSecrets `json:"-"`
}

// LoggingConfig configures the on-disk log sinks — the .NET Serilog File-sink
// equivalent. When FileEnabled, records are written under FileDirectory as
// log-<date>.log (text) and log-<date>.json (JSON), rolled daily and pruned
// after RetentionDays. Under the read-only root filesystem the directory must be
// a writable volume (an emptyDir mounted by the Helm chart).
type LoggingConfig struct {
	FileEnabled   bool   `json:"file_enabled"`
	FileDirectory string `json:"file_directory"`
	RetentionDays int    `json:"retention_days"`
}

// MonitoringConfig carries the per-env App Insights connection string. It is
// filled in at InstallRepoTemplate via the
// <azure-app-insights-connection-string-in-{env}> tokeniser token in
// config.{env}.json (matching the minimal .NET scaffold's appsettings).
type MonitoringConfig struct {
	AzureApplicationInsightsConnectionString string `json:"azure_application_insights_connection_string"`
}

// LoadConfig loads configuration as code with a clear precedence
// (mirroring the .NET appsettings model): in-code defaults < config.{env}.json <
// environment variables. Config-as-code: everything in config.{env}.json — port,
// debug, log_level, CORS, monitoring — takes effect; env vars override for
// container/CI use. A missing/invalid config file is a warning, not fatal
// (GO config-strictness decision: warn & default).
func LoadConfig() (*Config, error) {
	// Load .env file if it exists (for local development)
	envFile := ".env"
	if _, err := os.Stat(envFile); err == nil {
		if err := godotenv.Load(envFile); err != nil {
			slog.Warn("Failed to load .env file", "error", err)
		}
	}

	// The environment selects which config.{env}.json is layered in.
	environment := getEnvWithDefault("ENVIRONMENT", "development")

	// 1. In-code defaults.
	cfg := &Config{
		Environment: environment,
		// Port is hardcoded to 8890 (matching the minimal .NET scaffold). Go does
		// NOT use the port-number tokeniser token (which resolves to 9000) for its
		// port — GO-D5/G1.
		Port:               "8890",
		Debug:              environment != "production",
		LogLevel:           "info",
		CORSAllowedOrigins: []string{"*"},
		CORSAllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		CORSAllowedHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		Logging: LoggingConfig{
			FileEnabled:   true,
			FileDirectory: "logs",
			RetentionDays: 31,
		},
	}

	// 2. Overlay the environment-specific config file (warn & continue if absent).
	if err := cfg.loadConfigFile(); err != nil {
		slog.Warn("Failed to load config file, using defaults", "error", err)
	}

	// 3. Environment variables take precedence over defaults and the file.
	cfg.applyEnvOverrides()

	// Load secrets from the CSI tmpfs file mount.
	appSecrets, err := secrets.LoadSecrets()
	if err != nil {
		slog.Warn("Failed to load secrets, some features may not work", "error", err)
	}
	cfg.Secrets = appSecrets

	return cfg, nil
}

// loadConfigFile overlays config.{env}.json onto the current config. Every field
// present in the file wins over the in-code defaults (environment variables are
// applied afterwards, in applyEnvOverrides).
func (c *Config) loadConfigFile() error {
	configFile := fmt.Sprintf("config.%s.json", c.Environment)

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("config file %s not found", configFile)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var fileConfig Config
	if err := json.Unmarshal(data, &fileConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	if fileConfig.Environment != "" {
		c.Environment = fileConfig.Environment
	}
	if fileConfig.Port != "" {
		c.Port = fileConfig.Port
	}
	if fileConfig.LogLevel != "" {
		c.LogLevel = fileConfig.LogLevel
	}
	// debug is always present in the shipped config.{env}.json files, so the
	// file value is authoritative over the default.
	c.Debug = fileConfig.Debug
	if fileConfig.CORSAllowedOrigins != nil {
		c.CORSAllowedOrigins = fileConfig.CORSAllowedOrigins
	}
	if fileConfig.CORSAllowedMethods != nil {
		c.CORSAllowedMethods = fileConfig.CORSAllowedMethods
	}
	if fileConfig.CORSAllowedHeaders != nil {
		c.CORSAllowedHeaders = fileConfig.CORSAllowedHeaders
	}
	if fileConfig.Monitoring.AzureApplicationInsightsConnectionString != "" {
		c.Monitoring = fileConfig.Monitoring
	}
	// FileDirectory presence marks a populated logging block, so the file's
	// file_enabled is authoritative only then (avoids an absent block disabling files).
	if fileConfig.Logging.FileDirectory != "" {
		c.Logging.FileEnabled = fileConfig.Logging.FileEnabled
		c.Logging.FileDirectory = fileConfig.Logging.FileDirectory
		if fileConfig.Logging.RetentionDays != 0 {
			c.Logging.RetentionDays = fileConfig.Logging.RetentionDays
		}
	}

	return nil
}

// applyEnvOverrides lets environment variables override the file/defaults, so
// containers and CI can tune behaviour without editing the baked-in config.
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("ENVIRONMENT"); v != "" {
		c.Environment = v
	}
	if v := os.Getenv("PORT"); v != "" {
		c.Port = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
	if v := os.Getenv("DEBUG"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			c.Debug = b
		}
	}
	if v := os.Getenv("LOG_FILE_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			c.Logging.FileEnabled = b
		}
	}
	if v := os.Getenv("LOG_FILE_DIRECTORY"); v != "" {
		c.Logging.FileDirectory = v
	}
	if v := os.Getenv("LOG_RETENTION_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Logging.RetentionDays = n
		}
	}
}

// getEnvWithDefault returns the env var value or a fallback when unset/empty.
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
