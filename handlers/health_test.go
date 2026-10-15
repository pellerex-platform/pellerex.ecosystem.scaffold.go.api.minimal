package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"RepoUniqueNormalisedIdentifier/health"

	"github.com/gin-gonic/gin"
)

// answers serves each health address from the tester's last result and returns
// what each one answered, as "<status> <body>".
func answers(t *testing.T, tester *health.Tester) map[string]string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health/startup", StartupHealthCheck(tester))
	router.GET("/health/live", LivenessHealthCheck(tester))
	router.GET("/health/ready", ReadinessHealthCheck(tester))

	answered := map[string]string{}
	for _, path := range []string{"/health/startup", "/health/live", "/health/ready"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
			t.Errorf("%s answered as %q, expected plain text", path, contentType)
		}
		answered[path] = http.StatusText(response.Code) + " " + response.Body.String()
	}
	return answered
}

func expectAnswers(t *testing.T, answered map[string]string, expected map[string]string) {
	t.Helper()
	for path, answer := range expected {
		if answered[path] != answer {
			t.Errorf("%s answered %q, expected %q", path, answered[path], answer)
		}
	}
}

func newTester(checks ...health.Check) *health.Tester {
	return health.NewTester(slog.New(slog.NewJSONHandler(io.Discard, nil)), checks...)
}

func TestBeforeTheFirstTestEveryAddressAnswersUnhealthy(t *testing.T) {
	answered := answers(t, newTester())

	expectAnswers(t, answered, map[string]string{
		"/health/startup": "Service Unavailable Unhealthy",
		"/health/live":    "Service Unavailable Unhealthy",
		"/health/ready":   "Service Unavailable Unhealthy",
	})
}

func TestAfterAPassedTestEveryAddressAnswersHealthy(t *testing.T) {
	tester := newTester()
	tester.Test(context.Background())

	expectAnswers(t, answers(t, tester), map[string]string{
		"/health/startup": "OK Healthy",
		"/health/live":    "OK Healthy",
		"/health/ready":   "OK Healthy",
	})
}

// A failed test keeps the app started, so Kubernetes does not kill it while it
// starts, and answers one word with no reason: anyone can call these addresses.
func TestAfterAFailedTestLiveAndReadyAnswerUnhealthyWithNoReason(t *testing.T) {
	tester := newTester(health.Check{Name: "database", Test: func(context.Context) error {
		return errors.New("login failed for user orders-api")
	}})
	tester.Test(context.Background())

	expectAnswers(t, answers(t, tester), map[string]string{
		"/health/startup": "OK Healthy",
		"/health/live":    "Service Unavailable Unhealthy",
		"/health/ready":   "Service Unavailable Unhealthy",
	})
}
