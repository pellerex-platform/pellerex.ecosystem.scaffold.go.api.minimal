package health

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// logLine is one line the tester wrote.
type logLine struct {
	Level   string `json:"level"`
	Message string `json:"msg"`
}

// newTestTester returns a tester for the checks, writing its lines to a buffer
// the test reads with lines.
func newTestTester(checks ...Check) (*Tester, *bytes.Buffer) {
	written := &bytes.Buffer{}
	return NewTester(slog.New(slog.NewJSONHandler(written, nil)), checks...), written
}

// lines returns every line written so far.
func lines(t *testing.T, written *bytes.Buffer) []logLine {
	t.Helper()
	var found []logLine
	for _, text := range strings.Split(strings.TrimSpace(written.String()), "\n") {
		if text == "" {
			continue
		}
		var line logLine
		if err := json.Unmarshal([]byte(text), &line); err != nil {
			t.Fatalf("a written line is not one JSON object: %v\n%s", err, text)
		}
		found = append(found, line)
	}
	return found
}

// onlyLine returns the one line a single test wrote.
func onlyLine(t *testing.T, written *bytes.Buffer) logLine {
	t.Helper()
	found := lines(t, written)
	if len(found) != 1 {
		t.Fatalf("expected one line per test, got %d: %+v", len(found), found)
	}
	return found[0]
}

func passing(name string) Check {
	return Check{Name: name, Test: func(context.Context) error { return nil }}
}

func failing(name string, reason string) Check {
	return Check{Name: name, Test: func(context.Context) error { return errors.New(reason) }}
}

// hanging returns a check that never answers, however long it is given, until
// the test ends.
func hanging(t *testing.T, name string) Check {
	released := make(chan struct{})
	t.Cleanup(func() { close(released) })
	return Check{Name: name, Test: func(context.Context) error {
		<-released
		return nil
	}}
}

func TestBeforeTheFirstTestTheAppIsNeitherStartedNorHealthy(t *testing.T) {
	tester, written := newTestTester()

	if tester.Started() || tester.Healthy() {
		t.Errorf("expected neither started nor healthy before the first test, got started %v healthy %v", tester.Started(), tester.Healthy())
	}
	if written.Len() != 0 {
		t.Errorf("expected nothing written before the first test, got %s", written.String())
	}
}

func TestANewAppHasNothingToTestAndIsHealthy(t *testing.T) {
	tester, written := newTestTester()

	tester.Test(context.Background())

	if !tester.Started() || !tester.Healthy() {
		t.Errorf("expected started and healthy, got started %v healthy %v", tester.Started(), tester.Healthy())
	}
	line := onlyLine(t, written)
	if line.Level != "INFO" || line.Message != "Health test passed: nothing to test" {
		t.Errorf("expected the passed line at INFO, got %+v", line)
	}
}

func TestPassingChecksAreWrittenWithHowLongEachTook(t *testing.T) {
	tester, written := newTestTester(passing("database"), passing("cache"))

	tester.Test(context.Background())

	if !tester.Healthy() {
		t.Error("expected healthy when every check passed")
	}
	line := onlyLine(t, written)
	expected := regexp.MustCompile(`^Health test passed: database \d+ ms, cache \d+ ms$`)
	if line.Level != "INFO" || !expected.MatchString(line.Message) {
		t.Errorf("expected the passed line with each part's time at INFO, got %+v", line)
	}
}

func TestOneFailedCheckMakesTheAppUnhealthyAndSaysWhy(t *testing.T) {
	tester, written := newTestTester(failing("database", "connection refused"), passing("cache"))

	tester.Test(context.Background())

	if !tester.Started() {
		t.Error("expected started once the first test has finished, whatever it found")
	}
	if tester.Healthy() {
		t.Error("expected unhealthy when a check failed")
	}
	line := onlyLine(t, written)
	expected := regexp.MustCompile(`^Health test failed: database failed after \d+ ms \(connection refused\), cache \d+ ms$`)
	if line.Level != "WARN" || !expected.MatchString(line.Message) {
		t.Errorf("expected the failed line with what failed and why at WARN, got %+v", line)
	}
}

// Every check is tested at the same time, so checks that hang take 10 seconds
// together, not 10 seconds each. Each check here passes only once the other one
// has started too, so checks tested one after another fail.
func TestEveryCheckIsTestedAtTheSameTime(t *testing.T) {
	var started sync.WaitGroup
	started.Add(2)
	bothStarted := make(chan struct{})
	go func() {
		started.Wait()
		close(bothStarted)
	}()
	waitForTheOther := func(ctx context.Context) error {
		started.Done()
		select {
		case <-bothStarted:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	tester, written := newTestTester(Check{Name: "database", Test: waitForTheOther}, Check{Name: "cache", Test: waitForTheOther})
	tester.timeLimit = time.Second

	tester.Test(context.Background())

	if line := onlyLine(t, written); !tester.Healthy() {
		t.Errorf("expected both checks to be tested at the same time and pass, got %+v", line)
	}
}

func TestACheckThatDoesNotAnswerHasFailed(t *testing.T) {
	tester, written := newTestTester(hanging(t, "database"))
	tester.timeLimit = 20 * time.Millisecond

	tester.Test(context.Background())

	if tester.Healthy() {
		t.Error("expected unhealthy when a check did not answer")
	}
	expected := regexp.MustCompile(`^Health test failed: database failed after \d+ ms \(did not answer within 20 ms\)$`)
	if line := onlyLine(t, written); !expected.MatchString(line.Message) {
		t.Errorf("expected the check that did not answer to be named, got %+v", line)
	}
}

// A check reaches something outside the app, so a check that panics has failed,
// with the panic as its reason: it does not stop the app.
func TestACheckThatPanicsHasFailed(t *testing.T) {
	check := Check{Name: "database", Test: func(context.Context) error {
		panic("the database was never opened")
	}}
	tester, written := newTestTester(check, passing("cache"))

	tester.Test(context.Background())

	if tester.Healthy() {
		t.Error("expected unhealthy when a check panicked")
	}
	expected := regexp.MustCompile(`^Health test failed: database failed after \d+ ms \(panicked: the database was never opened\), cache \d+ ms$`)
	if line := onlyLine(t, written); !expected.MatchString(line.Message) {
		t.Errorf("expected the check that panicked to be named with its panic, got %+v", line)
	}
}

// A test the app stops part way, as it shuts down, has found nothing: it writes
// no line, so no check is said to have failed, and the last result stands.
func TestATestStoppedPartWayWritesNothing(t *testing.T) {
	checking := make(chan struct{})
	check := Check{Name: "database", Test: func(ctx context.Context) error {
		close(checking)
		<-ctx.Done()
		return ctx.Err()
	}}
	tester, written := newTestTester(check)

	ctx, stop := context.WithCancel(context.Background())
	go func() {
		<-checking
		stop()
	}()
	tester.Test(ctx)

	if written.Len() != 0 {
		t.Errorf("expected nothing written for a test that was stopped, got %s", written.String())
	}
	if tester.Started() {
		t.Error("expected a stopped first test to leave the app not yet started")
	}
}

func TestACheckIsGivenUpOnWhenItsTimeIsUp(t *testing.T) {
	gaveUp := make(chan error, 1)
	check := Check{Name: "database", Test: func(ctx context.Context) error {
		<-ctx.Done()
		gaveUp <- ctx.Err()
		return ctx.Err()
	}}
	tester, _ := newTestTester(check)
	tester.timeLimit = 20 * time.Millisecond

	tester.Test(context.Background())

	select {
	case err := <-gaveUp:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected the check's context to end at the time limit, got %v", err)
		}
	case <-time.After(time.Second):
		t.Error("the check's context did not end at the time limit")
	}
}

func TestTheAppIsHealthyAgainOnceItsChecksPass(t *testing.T) {
	var databaseUp atomic.Bool
	check := Check{Name: "database", Test: func(context.Context) error {
		if databaseUp.Load() {
			return nil
		}
		return errors.New("connection refused")
	}}
	tester, _ := newTestTester(check)

	tester.Test(context.Background())
	if tester.Healthy() {
		t.Fatal("expected unhealthy while the database is down")
	}

	databaseUp.Store(true)
	tester.Test(context.Background())
	if !tester.Healthy() {
		t.Error("expected healthy again once the database is back")
	}
}

// The first test runs as soon as Run starts, not an interval later: the startup
// address waits for it, so a new pod would otherwise wait a whole minute for traffic.
func TestRunTestsStraightAwayNotAnIntervalLater(t *testing.T) {
	tester, _ := newTestTester(passing("database"))
	tester.interval = time.Hour

	ctx, stop := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		tester.Run(ctx)
		close(stopped)
	}()
	defer func() {
		stop()
		<-stopped
	}()

	deadline := time.Now().Add(time.Second)
	for !tester.Started() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !tester.Started() {
		t.Error("expected the first test straight away, not an interval later")
	}
}

func TestRunTestsOnEveryTickUntilStopped(t *testing.T) {
	var tests atomic.Int32
	check := Check{Name: "database", Test: func(context.Context) error {
		tests.Add(1)
		return nil
	}}
	tester, _ := newTestTester(check)
	tester.interval = 10 * time.Millisecond

	ctx, stop := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		tester.Run(ctx)
		close(stopped)
	}()

	deadline := time.Now().Add(time.Second)
	for tests.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if tests.Load() < 3 {
		t.Fatalf("expected a test on every tick, got %d in a second", tests.Load())
	}

	stop()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Run did not return once its context was done")
	}
}

// The test runs once a minute and a check has 10 seconds: the chart's Kubernetes
// checks count failed tests by these, so a change must fail a test.
func TestTheTestRunsOnceAMinuteAndACheckHasTenSeconds(t *testing.T) {
	tester, _ := newTestTester()

	if tester.interval != time.Minute {
		t.Errorf("expected a test once a minute, got every %s", tester.interval)
	}
	if tester.timeLimit != 10*time.Second {
		t.Errorf("expected a check to have 10 seconds, got %s", tester.timeLimit)
	}
}
