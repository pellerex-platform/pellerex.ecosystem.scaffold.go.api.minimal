package main

import (
	"os"
	"strings"
	"testing"
)

// probeSettings reads one Kubernetes check of the chart's app container: every
// "key: value" line under it, with the nesting dropped ("path", "periodSeconds").
// The chart is a Helm template, not plain YAML, so it is read as text.
func probeSettings(t *testing.T, chart string, probe string) map[string]string {
	t.Helper()
	settings := map[string]string{}
	lines := strings.Split(chart, "\n")
	for index, line := range lines {
		if strings.TrimSpace(line) != probe+":" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		for _, setting := range lines[index+1:] {
			if len(setting)-len(strings.TrimLeft(setting, " ")) <= indent {
				break
			}
			key, value, found := strings.Cut(strings.TrimSpace(setting), ":")
			if found && strings.TrimSpace(value) != "" {
				settings[key] = strings.TrimSpace(value)
			}
		}
		return settings
	}
	t.Fatalf("the chart has no %s", probe)
	return nil
}

// The chart's Kubernetes checks act on the health test, which runs once a minute:
// out of traffic after two failed tests, restarted after five, about 5 minutes,
// and an answer that takes over 5 seconds counts as failed.
func TestChartChecksActOnTheHealthTestAtTheTwoSpeeds(t *testing.T) {
	content, err := os.ReadFile("infrastructure/Helm/templates/deployment.yaml")
	if err != nil {
		t.Fatalf("cannot read the chart: %v", err)
	}
	chart := string(content)

	expected := map[string]map[string]string{
		"startupProbe":   {"path": "/health/startup", "port": "http", "periodSeconds": "5", "failureThreshold": "24"},
		"readinessProbe": {"path": "/health/ready", "port": "http", "periodSeconds": "60", "timeoutSeconds": "5", "failureThreshold": "2"},
		"livenessProbe":  {"path": "/health/live", "port": "http", "periodSeconds": "60", "timeoutSeconds": "5", "failureThreshold": "5"},
	}

	for probe, settings := range expected {
		found := probeSettings(t, chart, probe)
		if len(found) != len(settings) {
			t.Errorf("%s has the settings %v, expected %v", probe, found, settings)
		}
		for key, value := range settings {
			if found[key] != value {
				t.Errorf("%s has %s %q, expected %q", probe, key, found[key], value)
			}
		}
	}
}
