package observability

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelslog"
)

// LogConfig is the subset of application configuration NewLogger needs. It is the
// Go analogue of the .NET scaffold's Serilog section in appsettings.json — the
// same knobs (level, enrichment, sinks), expressed as config-as-code (GO-D7).
type LogConfig struct {
	// Level is the minimum level string from config (debug|info|warn|error).
	Level string
	// ServiceName / ServiceVersion / Environment enrich every record, mirroring
	// the .NET LogContextEnrichment (ApplicationName/Version/Environment).
	ServiceName    string
	ServiceVersion string
	Environment    string
	// OTelEnabled adds an Azure Monitor sink (records exported via the OTel logs
	// bridge) alongside stdout — the capability parity for Serilog's
	// ApplicationInsights sink. Set true only once Init has stood up the pipeline.
	OTelEnabled bool

	// FileEnabled adds daily-rolling text + JSON file sinks under FileDirectory
	// (the .NET Serilog File sinks). RetentionDays bounds how long they are kept.
	FileEnabled   bool
	FileDirectory string
	RetentionDays int
}

// NewLogger builds a slog.Logger fanned out across the Serilog-equivalent sinks:
// structured JSON to stdout (console, always); daily-rolling text + JSON files
// under FileDirectory (when FileEnabled); and Azure Monitor via the OpenTelemetry
// logs bridge (when OTelEnabled). Every record is enriched with
// service.name/version + deployment.environment (the standing enrichers), and the
// minimum level is config-driven. The returned func flushes/closes the file
// sinks and must be called on shutdown (no-op when no files are open).
func NewLogger(cfg LogConfig) (*slog.Logger, func()) {
	level := parseLevel(cfg.Level)

	stdout := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	handlers := []slog.Handler{stdout}
	var closers []io.Closer

	if cfg.FileEnabled && cfg.FileDirectory != "" {
		// Text file (.NET Logs/log-<date>.log) — human-readable.
		if tw, err := newDailyFileWriter(cfg.FileDirectory, "log-", ".log", cfg.RetentionDays); err == nil {
			handlers = append(handlers, slog.NewTextHandler(tw, &slog.HandlerOptions{Level: level}))
			closers = append(closers, tw)
		} else {
			slog.New(stdout).Warn("file logging (text) disabled: cannot open log file", "error", err, "dir", cfg.FileDirectory)
		}
		// JSON file (.NET Logs/log-<date>.json) — machine-queryable.
		if jw, err := newDailyFileWriter(cfg.FileDirectory, "log-", ".json", cfg.RetentionDays); err == nil {
			handlers = append(handlers, slog.NewJSONHandler(jw, &slog.HandlerOptions{Level: level}))
			closers = append(closers, jw)
		} else {
			slog.New(stdout).Warn("file logging (json) disabled: cannot open log file", "error", err, "dir", cfg.FileDirectory)
		}
	}

	if cfg.OTelEnabled {
		// Reads the global LoggerProvider that Init installed → Azure Monitor.
		handlers = append(handlers, otelslog.NewHandler(cfg.ServiceName))
	}

	logger := slog.New(newFanoutHandler(handlers...)).With(
		slog.String("service.name", cfg.ServiceName),
		slog.String("service.version", cfg.ServiceVersion),
		slog.String("deployment.environment", cfg.Environment),
	)

	cleanup := func() {
		for _, c := range closers {
			_ = c.Close()
		}
	}
	return logger, cleanup
}

// parseLevel maps the config log_level string to an slog.Level (default info),
// the analogue of Serilog's MinimumLevel.
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// fanoutHandler dispatches each record to every wrapped handler — the slog
// equivalent of Serilog's multiple WriteTo sinks (stdout + Azure Monitor).
type fanoutHandler struct {
	handlers []slog.Handler
}

func newFanoutHandler(handlers ...slog.Handler) *fanoutHandler {
	return &fanoutHandler{handlers: handlers}
}

func (f *fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (f *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range f.handlers {
		if h.Enabled(ctx, r.Level) {
			// Clone: a Record must not be shared unmodified across handlers.
			if err := h.Handle(ctx, r.Clone()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (f *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &fanoutHandler{handlers: next}
}

func (f *fanoutHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithGroup(name)
	}
	return &fanoutHandler{handlers: next}
}
