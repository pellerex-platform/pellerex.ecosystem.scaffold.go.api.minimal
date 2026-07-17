package secrets

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// DefaultSecretsMountPath is where the Azure Key Vault Provider for Secrets Store
// CSI Driver mounts the vault's secrets — one tmpfs file per secret (key-per-file).
// This is the Go equivalent of .NET's config.AddKeyPerFile(mountDir).
//
// The mount lives on an in-memory tmpfs: secret values never touch etcd, the
// container image, the repo, or the process environment (env vars leak via
// /proc/<pid>/environ, child processes and crash dumps — which is why the file
// mount is used instead of secretObjects/env injection — GO-D6/G11).
//
// Override with SECRETS_MOUNT_PATH (e.g. a local directory for development).
const DefaultSecretsMountPath = "/mnt/secrets-store"

// AppSecrets represents the application secrets structure.
type AppSecrets struct {
	APISecretKey       string `json:"APISecretKey,omitempty"`
	Environment        string `json:"Environment,omitempty"`
	DbConnectionString string `json:"DbConnectionString,omitempty"`
}

// LoadSecrets reads the application secrets key-per-file from the CSI tmpfs
// mount. Each file in the mount directory is one secret: the file name is the
// key, the (trimmed) file contents are the value.
//
// A missing mount is non-fatal: local/dev runs that have no CSI mount continue
// with empty secrets rather than failing to start.
func LoadSecrets() (*AppSecrets, error) {
	mountPath := mountPath()

	values, err := readKeyPerFile(mountPath)
	if err != nil {
		slog.Warn("CSI secrets mount not available; continuing without mounted secrets",
			"error", err, "path", mountPath)
		return &AppSecrets{}, nil
	}

	secrets := &AppSecrets{
		APISecretKey:       values["APISecretKey"],
		Environment:        values["Environment"],
		DbConnectionString: values["DbConnectionString"],
	}

	slog.Debug("Loaded secrets from CSI tmpfs mount",
		"source", "csi-file-mount", "path", mountPath, "keys", len(values))

	return secrets, nil
}

// mountPath returns the configured CSI secrets mount path.
func mountPath() string {
	if path := os.Getenv("SECRETS_MOUNT_PATH"); path != "" {
		return path
	}
	return DefaultSecretsMountPath
}

// readKeyPerFile reads every regular file in dir as a secret (key = file name,
// value = trimmed file contents). Directories and the CSI driver's bookkeeping
// dot-entries (e.g. "..data") are skipped.
func readKeyPerFile(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	values := make(map[string]string)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			slog.Warn("Failed to read mounted secret", "error", err, "key", name)
			continue
		}

		values[name] = strings.TrimRight(string(data), "\r\n")
	}

	return values, nil
}

// GetSecret safely retrieves a specific secret value.
func GetSecret(secrets *AppSecrets, key string) string {
	if secrets == nil {
		return ""
	}

	switch key {
	case "APISecretKey":
		return secrets.APISecretKey
	case "Environment":
		return secrets.Environment
	case "DbConnectionString":
		return secrets.DbConnectionString
	default:
		return ""
	}
}

// HasDatabaseSecret checks if a database connection string is configured.
func HasDatabaseSecret(secrets *AppSecrets) bool {
	return secrets != nil && secrets.DbConnectionString != ""
}

// GetDatabaseConnectionPreview returns a preview of the database connection string.
func GetDatabaseConnectionPreview(secrets *AppSecrets) string {
	if !HasDatabaseSecret(secrets) {
		return "No database connection configured"
	}

	dbConn := secrets.DbConnectionString
	if len(dbConn) > 30 {
		return dbConn[:30] + "..."
	}
	return dbConn
}
