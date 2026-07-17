package observability

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// ShutdownFunc flushes and stops the telemetry pipeline. Always safe to call.
type ShutdownFunc func(context.Context) error

// Config is the subset of application configuration the telemetry bootstrap needs.
type Config struct {
	ServiceName              string
	ServiceVersion           string
	Environment              string
	AppInsightsConnectionStr string
	Logger                   *slog.Logger
}

// Init wires OpenTelemetry traces, metrics AND logs and exports them to Azure
// Application Insights.
//
// GO-D7 — the one genuine divergence from the .NET scaffold: there is no
// maintained first-party Go App Insights SDK (microsoft/ApplicationInsights-Go
// is archived), so Go uses OpenTelemetry with an OTLP exporter to the SAME App
// Insights resource via the SAME <azure-app-insights-connection-string-in-{env}>
// token the tokeniser substitutes into config.{env}.json. Destination, config
// token and SetupMonitoring wiring are identical to .NET; only the in-app SDK
// differs. The logs pipeline set up here is what the slog OTel bridge (see
// logging.go) exports to — the capability parity for Serilog's AI sink.
//
// Transport: by default the exporter is configured from the App Insights
// connection string. Set the standard OTEL_EXPORTER_OTLP_ENDPOINT (and
// OTEL_EXPORTER_OTLP_HEADERS) env vars to route via an OpenTelemetry Collector
// running the azuremonitorexporter instead.
//
// NOTE (GO-D7): confirm the exact ingestion transport at build time — Azure
// Monitor direct OTLP ingestion vs. an OTel Collector sidecar — and finalise the
// exporter endpoint/auth accordingly. The connection-string token is the single
// config input either way.
//
// The returned bool reports whether the pipeline was enabled, so the caller can
// decide whether to attach the Azure Monitor logs sink to its logger.
func Init(ctx context.Context, cfg Config) (ShutdownFunc, bool, error) {
	noop := func(context.Context) error { return nil }

	// Send the OTel SDK's OWN diagnostics (export failures, warnings) to stderr,
	// deliberately NOT through the app logger — so a telemetry export error can
	// never re-enter the logs bridge and amplify into a feedback loop. Application
	// logs still go to stdout (+ Azure Monitor via the bridge).
	routeOTelDiagnosticsToStderr()

	endpointConfigured := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != ""
	if strings.TrimSpace(cfg.AppInsightsConnectionStr) == "" && !endpointConfigured {
		if cfg.Logger != nil {
			cfg.Logger.Info("Telemetry disabled: no App Insights connection string / OTLP endpoint configured")
		}
		return noop, false, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return noop, false, err
	}

	traceOpts, metricOpts, logOpts := exporterOptions(cfg.AppInsightsConnectionStr, endpointConfigured)

	traceExp, err := otlptracehttp.New(ctx, traceOpts...)
	if err != nil {
		return noop, false, err
	}

	metricExp, err := otlpmetrichttp.New(ctx, metricOpts...)
	if err != nil {
		_ = traceExp.Shutdown(ctx)
		return noop, false, err
	}

	logExp, err := otlploghttp.New(ctx, logOpts...)
	if err != nil {
		_ = traceExp.Shutdown(ctx)
		_ = metricExp.Shutdown(ctx)
		return noop, false, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithResource(res),
	)
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	// Installed globally so otelslog.NewHandler (logging.go) exports to it.
	global.SetLoggerProvider(loggerProvider)

	if cfg.Logger != nil {
		cfg.Logger.Info("OpenTelemetry initialised (OTLP → Azure Monitor)", "service", cfg.ServiceName)
	}

	shutdown := func(ctx context.Context) error {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return errors.Join(
			tracerProvider.Shutdown(shutdownCtx),
			meterProvider.Shutdown(shutdownCtx),
			loggerProvider.Shutdown(shutdownCtx),
		)
	}
	return shutdown, true, nil
}

// routeOTelDiagnosticsToStderr points the OTel SDK's internal logger and error
// handler at a stderr-only JSON logger. This keeps telemetry self-diagnostics off
// stdout (where the application's structured logs live) and, crucially, out of the
// OTel logs bridge, so export failures cannot loop back through it.
func routeOTelDiagnosticsToStderr() {
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	diag := slog.New(h)
	otel.SetLogger(logr.FromSlogHandler(h))
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		diag.Error("otel diagnostics", "error", err)
	}))
}

// exporterOptions builds OTLP exporter options for traces, metrics and logs.
// When OTEL_EXPORTER_OTLP_ENDPOINT is set, the OTLP exporters read it (and
// OTEL_EXPORTER_OTLP_HEADERS) from the environment, so no explicit options are
// returned. Otherwise the options are derived from the App Insights connection
// string.
func exporterOptions(connectionString string, endpointConfigured bool) ([]otlptracehttp.Option, []otlpmetrichttp.Option, []otlploghttp.Option) {
	if endpointConfigured {
		return nil, nil, nil
	}

	fields := parseConnectionString(connectionString)
	endpoint := fields["IngestionEndpoint"]
	instrumentationKey := fields["InstrumentationKey"]

	traceOpts := []otlptracehttp.Option{}
	metricOpts := []otlpmetrichttp.Option{}
	logOpts := []otlploghttp.Option{}
	if endpoint != "" {
		traceOpts = append(traceOpts, otlptracehttp.WithEndpointURL(endpoint))
		metricOpts = append(metricOpts, otlpmetrichttp.WithEndpointURL(endpoint))
		logOpts = append(logOpts, otlploghttp.WithEndpointURL(endpoint))
	}
	if instrumentationKey != "" {
		// Auth header for direct ingestion — confirm the exact header/scheme at
		// build time per GO-D7 (differs between direct OTLP ingestion and a Collector).
		headers := map[string]string{"x-instrumentation-key": instrumentationKey}
		traceOpts = append(traceOpts, otlptracehttp.WithHeaders(headers))
		metricOpts = append(metricOpts, otlpmetrichttp.WithHeaders(headers))
		logOpts = append(logOpts, otlploghttp.WithHeaders(headers))
	}
	return traceOpts, metricOpts, logOpts
}

// parseConnectionString splits an App Insights connection string
// ("Key1=Value1;Key2=Value2;…") into a map.
func parseConnectionString(cs string) map[string]string {
	fields := make(map[string]string)
	for _, part := range strings.Split(cs, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			fields[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return fields
}
