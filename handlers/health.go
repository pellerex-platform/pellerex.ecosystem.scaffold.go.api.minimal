package handlers

import (
	"net/http"

	"RepoUniqueNormalisedIdentifier/health"

	"github.com/gin-gonic/gin"
)

// The one-word answers of the health addresses. Anyone can call them, so they say
// only healthy or not: what failed and why goes to the health test's log line.
// The words are ASP.NET Core's own, which the .NET scaffold answers with, so an app
// answers the same in every language.
const (
	healthyAnswer   = "Healthy"
	unhealthyAnswer = "Unhealthy"
)

// StartupHealthCheck handles startup health checks
// @Summary Startup health check
// @Description Healthy once the first health test has finished, whatever it found
// @Tags Health
// @Produce plain
// @Success 200 {string} string "Healthy"
// @Failure 503 {string} string "Unhealthy"
// @Router /health/startup [get]
func StartupHealthCheck(tester *health.Tester) gin.HandlerFunc {
	return func(c *gin.Context) {
		answer(c, tester.Started())
	}
}

// LivenessHealthCheck handles liveness health checks
// @Summary Liveness health check
// @Description Healthy when the last health test passed
// @Tags Health
// @Produce plain
// @Success 200 {string} string "Healthy"
// @Failure 503 {string} string "Unhealthy"
// @Router /health/live [get]
func LivenessHealthCheck(tester *health.Tester) gin.HandlerFunc {
	return func(c *gin.Context) {
		answer(c, tester.Healthy())
	}
}

// ReadinessHealthCheck handles readiness health checks
// @Summary Readiness health check
// @Description Healthy when the last health test passed
// @Tags Health
// @Produce plain
// @Success 200 {string} string "Healthy"
// @Failure 503 {string} string "Unhealthy"
// @Router /health/ready [get]
func ReadinessHealthCheck(tester *health.Tester) gin.HandlerFunc {
	return func(c *gin.Context) {
		answer(c, tester.Healthy())
	}
}

// answer writes the one-word answer: 200 Healthy, or 503 Unhealthy.
func answer(c *gin.Context, healthy bool) {
	if healthy {
		c.String(http.StatusOK, healthyAnswer)
		return
	}
	c.String(http.StatusServiceUnavailable, unhealthyAnswer)
}
