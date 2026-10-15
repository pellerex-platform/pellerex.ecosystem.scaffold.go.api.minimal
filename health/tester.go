// Package health runs the app's health test. Once a minute it tests every check
// the app cannot work without, writes one log line saying what was tested and how
// long each part took, and keeps the result. The health addresses answer from that
// result (see handlers/health.go), so no number of calls to them costs the checked
// dependencies anything.
package health

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	// testInterval is how often the test runs. Tenant apps test once a minute, not
	// every 30 seconds as the platform services do: the databases Pellerex creates
	// for them share one pool, so every test of every app adds to it. The chart's
	// Kubernetes checks call once a minute too (infrastructure/Helm/templates/deployment.yaml).
	testInterval = time.Minute
	// checkTimeLimit is how long one check may take. A check that has not answered
	// by then has failed, so a dependency that hangs makes the app unhealthy.
	checkTimeLimit = 10 * time.Second
)

// Check is one thing the app cannot work without, such as its database. Test
// returns nil when the thing works and an error saying why when it does not, and
// gives up when ctx is done.
type Check struct {
	Name string
	Test func(ctx context.Context) error
}

// Tester runs the health test and keeps its last result. It is built once in
// main, which starts Run in the background and hands the Tester to the health
// handlers.
type Tester struct {
	logger    *slog.Logger
	checks    []Check
	interval  time.Duration
	timeLimit time.Duration

	mu       sync.RWMutex
	finished bool
	passed   bool
}

// checkResult is what one check of one test found.
type checkResult struct {
	name    string
	elapsed time.Duration
	err     error
}

// NewTester returns a Tester for the given checks. A new app passes none: it has
// nothing to test until its developer adds what it cannot work without.
func NewTester(logger *slog.Logger, checks ...Check) *Tester {
	return &Tester{
		logger:    logger,
		checks:    checks,
		interval:  testInterval,
		timeLimit: checkTimeLimit,
	}
}

// Run tests straight away and then once a minute, until ctx is done.
func (t *Tester) Run(ctx context.Context) {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		t.Test(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Test runs every check at the same time, writes one log line with what each
// found, and keeps whether all of them passed.
func (t *Tester) Test(ctx context.Context) {
	results := make([]checkResult, len(t.checks))
	var running sync.WaitGroup
	for index, check := range t.checks {
		running.Go(func() {
			start := time.Now()
			err := t.testCheck(ctx, check)
			results[index] = checkResult{name: check.Name, elapsed: time.Since(start), err: err}
		})
	}
	running.Wait()

	// A test stopped part way, as the app shuts down, has found nothing: it writes
	// no line, so no check is said to have failed, and the last result stands.
	if ctx.Err() != nil {
		return
	}

	passed, summary := summarise(results)
	if passed {
		t.logger.InfoContext(ctx, "Health test passed: "+summary)
	} else {
		t.logger.WarnContext(ctx, "Health test failed: "+summary)
	}

	t.mu.Lock()
	t.finished = true
	t.passed = passed
	t.mu.Unlock()
}

// testCheck runs one check and returns why it failed, or nil when it passed. A
// check that has not answered within the time limit has failed. So has one that
// panicked: the check reaches something outside the app, so its panic is its
// answer and does not stop the app.
func (t *Tester) testCheck(ctx context.Context, check Check) error {
	ctx, cancel := context.WithTimeout(ctx, t.timeLimit)
	defer cancel()

	answer := make(chan error, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				answer <- fmt.Errorf("panicked: %v", recovered)
			}
		}()
		answer <- check.Test(ctx)
	}()

	select {
	case err := <-answer:
		return err
	case <-ctx.Done():
		return fmt.Errorf("did not answer within %d ms", t.timeLimit.Milliseconds())
	}
}

// summarise says whether every check passed, and puts what each found in one
// line, in the order the checks were given: "database 12 ms, cache 3 ms".
func summarise(results []checkResult) (bool, string) {
	if len(results) == 0 {
		return true, "nothing to test"
	}

	passed := true
	parts := make([]string, len(results))
	for index, result := range results {
		milliseconds := result.elapsed.Milliseconds()
		if result.err != nil {
			passed = false
			parts[index] = fmt.Sprintf("%s failed after %d ms (%v)", result.name, milliseconds, result.err)
		} else {
			parts[index] = fmt.Sprintf("%s %d ms", result.name, milliseconds)
		}
	}

	return passed, strings.Join(parts, ", ")
}

// Started reports whether the first test has finished, whatever it found.
func (t *Tester) Started() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.finished
}

// Healthy reports whether the last test passed. Before the first test has
// finished it is false.
func (t *Tester) Healthy() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.finished && t.passed
}
