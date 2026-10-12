package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// The name the correlation id is logged under is written out here on purpose:
// the portal's Logs tab reads it by this exact name, so a rename must fail a test.
const loggedAs = "CorrelationId"

// writtenLine logs one line through the correlation handler and returns the
// text the JSON handler behind it wrote.
func writtenLine(log func(logger *slog.Logger)) string {
	written := &bytes.Buffer{}
	log(slog.New(&correlationHandler{next: slog.NewJSONHandler(written, nil)}))
	return written.String()
}

// loggedLine logs one line through the correlation handler and returns it as
// the JSON handler behind it wrote it.
func loggedLine(t *testing.T, log func(logger *slog.Logger)) map[string]any {
	t.Helper()
	written := writtenLine(log)

	var line map[string]any
	if err := json.Unmarshal([]byte(written), &line); err != nil {
		t.Fatalf("the logged line is not one JSON object: %v\n%s", err, written)
	}
	return line
}

func TestRecordLoggedWithARequestsContextCarriesItsCorrelationID(t *testing.T) {
	ctx := WithCorrelationID(context.Background(), "order-42")

	line := loggedLine(t, func(logger *slog.Logger) { logger.InfoContext(ctx, "saved", "rows", 3) })

	if line[loggedAs] != "order-42" {
		t.Errorf("expected the line to carry the correlation id, got %v", line)
	}
	if line["rows"] != float64(3) {
		t.Errorf("expected the line to keep its own attributes, got %v", line)
	}
}

func TestRecordLoggedWithNoRequestContextCarriesNoCorrelationID(t *testing.T) {
	line := loggedLine(t, func(logger *slog.Logger) { logger.InfoContext(context.Background(), "started") })

	if _, carried := line[loggedAs]; carried {
		t.Errorf("expected no correlation id outside a request, got %v", line)
	}
}

func TestLoggerWithItsOwnAttributesStillCarriesTheCorrelationID(t *testing.T) {
	ctx := WithCorrelationID(context.Background(), "order-42")

	line := loggedLine(t, func(logger *slog.Logger) { logger.With("component", "billing").InfoContext(ctx, "saved") })

	if line[loggedAs] != "order-42" || line["component"] != "billing" {
		t.Errorf("expected the correlation id beside the logger's own attribute, got %v", line)
	}
}

func TestLoggerWithAGroupCarriesTheCorrelationIDUnderThatGroup(t *testing.T) {
	ctx := WithCorrelationID(context.Background(), "order-42")

	line := loggedLine(t, func(logger *slog.Logger) { logger.WithGroup("billing").InfoContext(ctx, "saved", "rows", 3) })

	group, isGroup := line["billing"].(map[string]any)
	if !isGroup || group[loggedAs] != "order-42" || group["rows"] != float64(3) {
		t.Errorf("expected the correlation id under the group, beside the record's own attribute, got %v", line)
	}
}

func TestRecordThatNamesTheCorrelationIDItselfKeepsItsOwnOnce(t *testing.T) {
	ctx := WithCorrelationID(context.Background(), "order-42")

	written := writtenLine(func(logger *slog.Logger) {
		logger.InfoContext(ctx, "HTTP request", "StatusCode", 200, loggedAs, "named-by-the-record", "Port", "8080")
	})

	if strings.Count(written, `"`+loggedAs+`"`) != 1 || !strings.Contains(written, `"named-by-the-record"`) {
		t.Errorf("expected the record's own correlation id, once, got %s", written)
	}
}
