package observability

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// dateLayout matches the .NET Serilog daily file naming (log-20250823.log).
const dateLayout = "20060102"

// dailyFileWriter is an io.Writer that appends to
// <dir>/<prefix><YYYYMMDD><suffix>, rolling to a new file at each date change and
// pruning files older than retentionDays. It is the daily-rolling File sink from
// the .NET Serilog config, implemented for slog. Safe for concurrent use.
type dailyFileWriter struct {
	dir           string
	prefix        string
	suffix        string
	retentionDays int

	mu   sync.Mutex
	day  string
	file *os.File
}

func newDailyFileWriter(dir, prefix, suffix string, retentionDays int) (*dailyFileWriter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	w := &dailyFileWriter{dir: dir, prefix: prefix, suffix: suffix, retentionDays: retentionDays}
	if err := w.rotate(today()); err != nil {
		return nil, err
	}
	return w, nil
}

func today() string { return time.Now().Format(dateLayout) }

func (w *dailyFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if d := today(); d != w.day {
		if err := w.rotate(d); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

// rotate closes the current file and opens the one for day d, then prunes.
func (w *dailyFileWriter) rotate(d string) error {
	if w.file != nil {
		_ = w.file.Close()
	}
	name := filepath.Join(w.dir, w.prefix+d+w.suffix)
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	w.file = f
	w.day = d
	w.prune()
	return nil
}

// prune removes matching log files older than retentionDays (best-effort).
func (w *dailyFileWriter) prune() {
	if w.retentionDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -w.retentionDays)
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, w.prefix) || !strings.HasSuffix(name, w.suffix) {
			continue
		}
		datePart := strings.TrimSuffix(strings.TrimPrefix(name, w.prefix), w.suffix)
		t, err := time.Parse(dateLayout, datePart)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			_ = os.Remove(filepath.Join(w.dir, name))
		}
	}
}

// Close closes the underlying file.
func (w *dailyFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}
