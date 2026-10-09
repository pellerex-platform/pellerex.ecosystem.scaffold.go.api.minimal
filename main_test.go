package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"RepoUniqueNormalisedIdentifier/config"
	"RepoUniqueNormalisedIdentifier/observability"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// recordedLogs keeps every record the OpenTelemetry logs pipeline exports, so a
// test can read what would have been sent to Azure Monitor.
type recordedLogs struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (r *recordedLogs) Export(_ context.Context, records []sdklog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range records {
		r.records = append(r.records, record.Clone())
	}
	return nil
}

func (r *recordedLogs) Shutdown(context.Context) error   { return nil }
func (r *recordedLogs) ForceFlush(context.Context) error { return nil }

// find returns the first exported record with the given message.
func (r *recordedLogs) find(t *testing.T, message string) sdklog.Record {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range r.records {
		if record.Body().AsString() == message {
			return record
		}
	}
	t.Fatalf("no record with the message %q was exported", message)
	return sdklog.Record{}
}

// attribute returns the value of one attribute of an exported record.
func attribute(t *testing.T, record sdklog.Record, key string) otellog.Value {
	t.Helper()
	var value otellog.Value
	found := false
	record.WalkAttributes(func(pair otellog.KeyValue) bool {
		if pair.Key == key {
			value = pair.Value
			found = true
		}
		return !found
	})
	if !found {
		t.Fatalf("the record %q has no attribute %q", record.Body().AsString(), key)
	}
	return value
}

// captureGinOutput keeps what gin itself prints to stderr and drops its route
// listing, so a test can tell whether gin printed a panic in readable form.
func captureGinOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	printed := &bytes.Buffer{}
	previousErrorWriter, previousWriter := gin.DefaultErrorWriter, gin.DefaultWriter
	gin.DefaultErrorWriter, gin.DefaultWriter = printed, io.Discard
	t.Cleanup(func() { gin.DefaultErrorWriter, gin.DefaultWriter = previousErrorWriter, previousWriter })
	return printed
}

// newTestRouter builds the service's real router and logger, in the given gin
// mode, on top of an in-memory trace exporter and an in-memory logs exporter.
func newTestRouter(t *testing.T, mode string) (*gin.Engine, *tracetest.InMemoryExporter, *recordedLogs) {
	t.Helper()
	gin.SetMode(mode)
	t.Cleanup(func() { gin.SetMode(gin.TestMode) })

	spans := tracetest.NewInMemoryExporter()
	previousTracerProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans)))
	t.Cleanup(func() { otel.SetTracerProvider(previousTracerProvider) })

	logs := &recordedLogs{}
	previousLoggerProvider := global.GetLoggerProvider()
	global.SetLoggerProvider(sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(logs))))
	t.Cleanup(func() { global.SetLoggerProvider(previousLoggerProvider) })

	logger, closeLogger := observability.NewLogger(observability.LogConfig{
		Level:          "debug",
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		Environment:    "development",
		OTelEnabled:    true,
	})
	t.Cleanup(closeLogger)

	cfg := &config.Config{Environment: "development", Port: "8080", LogLevel: "debug"}
	return newRouter(cfg, logger), spans, logs
}

// requestSpan returns the one span a single request produces.
func requestSpan(t *testing.T, spans *tracetest.InMemoryExporter) tracetest.SpanStub {
	t.Helper()
	recorded := spans.GetSpans()
	if len(recorded) != 1 {
		t.Fatalf("expected one span for the request, got %d", len(recorded))
	}
	if !recorded[0].SpanContext.TraceID().IsValid() {
		t.Fatal("the request's span has no trace id")
	}
	return recorded[0]
}

func TestRequestThatPanicsIsLoggedUnderItsRequest(t *testing.T) {
	printedByGin := captureGinOutput(t)
	router, spans, logs := newTestRouter(t, gin.ReleaseMode)
	router.GET("/boom", func(*gin.Context) { panic("boom") })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected a 500 response, got %d", response.Code)
	}

	span := requestSpan(t, spans)
	traceID := span.SpanContext.TraceID()

	panicRecord := logs.find(t, "Unhandled panic in request")
	if panicRecord.Severity() != otellog.SeverityError {
		t.Errorf("expected the panic at error severity, got %v", panicRecord.Severity())
	}
	if panicRecord.TraceID() != traceID {
		t.Errorf("the panic record carries trace id %s, the request has %s", panicRecord.TraceID(), traceID)
	}
	if message := attribute(t, panicRecord, "exception.message").AsString(); message != "boom" {
		t.Errorf("expected exception.message %q, got %q", "boom", message)
	}
	if kind := attribute(t, panicRecord, "exception.type").AsString(); kind != "string" {
		t.Errorf("expected exception.type %q, got %q", "string", kind)
	}
	if stack := attribute(t, panicRecord, "exception.stacktrace").AsString(); !strings.Contains(stack, "main_test.go") {
		t.Errorf("the stack trace does not name the file that panicked:\n%s", stack)
	}
	if path := attribute(t, panicRecord, "RequestPath").AsString(); path != "/boom" {
		t.Errorf("expected RequestPath %q, got %q", "/boom", path)
	}

	// The request line is still written for a request that crashed, with its real status
	requestRecord := logs.find(t, "HTTP request")
	if status := attribute(t, requestRecord, "StatusCode").AsInt64(); status != http.StatusInternalServerError {
		t.Errorf("expected the request line to say 500, got %d", status)
	}
	if requestRecord.TraceID() != traceID {
		t.Errorf("the request line carries trace id %s, the request has %s", requestRecord.TraceID(), traceID)
	}

	if span.Status.Code != codes.Error {
		t.Errorf("expected the request's span to be marked as an error, got %v", span.Status.Code)
	}
	recordedOnSpan := false
	for _, event := range span.Events {
		if event.Name == "exception" {
			recordedOnSpan = true
		}
	}
	if !recordedOnSpan {
		t.Error("the panic was not recorded on the request's span")
	}

	// Deployed (release mode), the structured record is the only copy of the panic
	if printedByGin.Len() != 0 {
		t.Errorf("gin printed the panic by itself in release mode:\n%s", printedByGin.String())
	}
}

func TestPanicIsAlsoPrintedReadablyInDebugMode(t *testing.T) {
	printedByGin := captureGinOutput(t)
	router, _, logs := newTestRouter(t, gin.DebugMode)
	router.GET("/boom", func(*gin.Context) { panic("boom") })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected a 500 response, got %d", response.Code)
	}
	if !strings.Contains(printedByGin.String(), "panic recovered") {
		t.Errorf("gin did not print the panic in readable form in debug mode:\n%s", printedByGin.String())
	}
	logs.find(t, "Unhandled panic in request")
}

func TestHandlerLogLinesCarryTheirRequest(t *testing.T) {
	cases := []struct {
		path    string
		message string
	}{
		{path: "/v1/hello", message: "Hello endpoint accessed"},
		{path: "/health/live", message: "Liveness health check accessed"},
		{path: "/health/startup", message: "Startup health check accessed"},
		{path: "/health/ready", message: "Readiness health check accessed"},
	}

	for _, testCase := range cases {
		t.Run(testCase.path, func(t *testing.T) {
			router, spans, logs := newTestRouter(t, gin.ReleaseMode)

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, testCase.path, nil))

			traceID := requestSpan(t, spans).SpanContext.TraceID()

			handlerRecord := logs.find(t, testCase.message)
			if handlerRecord.TraceID() != traceID {
				t.Errorf("the handler's record carries trace id %s, the request has %s", handlerRecord.TraceID(), traceID)
			}

			requestRecord := logs.find(t, "HTTP request")
			if requestRecord.TraceID() != traceID {
				t.Errorf("the request line carries trace id %s, the request has %s", requestRecord.TraceID(), traceID)
			}
			if status := attribute(t, requestRecord, "StatusCode").AsInt64(); status != int64(response.Code) {
				t.Errorf("the request line says %d, the response was %d", status, response.Code)
			}
		})
	}
}
