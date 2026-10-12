package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"RepoUniqueNormalisedIdentifier/config"

	"github.com/gin-gonic/gin"
)

// The request line and the panic record are what the portal's Logs tab finds a
// request by, so they name the correlation id themselves and do not lean on the
// logger they are given to add it.
func TestRequestLineAndPanicRecordNameTheCorrelationIDThemselves(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	t.Cleanup(func() { gin.SetMode(gin.TestMode) })

	written := &bytes.Buffer{}
	plainLogger := slog.New(slog.NewJSONHandler(written, nil))

	router := gin.New()
	router.Use(LoggingMiddleware(plainLogger, &config.Config{Environment: "development", Port: "8080"}))
	router.Use(RecoveryMiddleware(plainLogger))
	router.GET("/boom", func(*gin.Context) { panic("boom") })

	request := httptest.NewRequest(http.MethodGet, "/boom", nil)
	request.Header.Set("X-Correlation-Id", "order-42")
	router.ServeHTTP(httptest.NewRecorder(), request)

	carried := map[string]any{}
	for _, text := range strings.Split(strings.TrimSpace(written.String()), "\n") {
		var line map[string]any
		if err := json.Unmarshal([]byte(text), &line); err != nil {
			t.Fatalf("a logged line is not one JSON object: %v\n%s", err, text)
		}
		carried[line["msg"].(string)] = line["CorrelationId"]
	}

	for _, message := range []string{"Unhandled panic in request", "HTTP request"} {
		if carried[message] != "order-42" {
			t.Errorf("expected the record %q to carry the correlation id, got %v", message, carried[message])
		}
	}
}
