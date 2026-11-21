package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// New creates a structured slog.Logger that always writes to file and stdout.
// closeFlushes should be called on shutdown (may be a no-op closer).
func New(level, format, file string) (*slog.Logger, func() error, error) {
	file = strings.TrimSpace(file)
	if file == "" {
		return nil, nil, fmt.Errorf("log file path is required")
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil && filepath.Dir(file) != "." {
		return nil, nil, fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}

	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}
	fileWriter := &rotatingFile{file: f, path: file}
	stopRotation := watchRotation(fileWriter)
	w := io.MultiWriter(os.Stdout, fileWriter)
	var handler slog.Handler
	if strings.EqualFold(format, "text") {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}
	closeFn := func() error {
		stopRotation()
		return fileWriter.Close()
	}
	return slog.New(handler), closeFn, nil
}

// Serialize writes and descriptor replacement so rename-based rotation does not
// truncate records. If reopening fails, keep writing to the original descriptor.
type rotatingFile struct {
	mu   sync.Mutex
	file *os.File
	path string
}

func (f *rotatingFile) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.file == nil {
		return 0, os.ErrClosed
	}
	return f.file.Write(p)
}

func (f *rotatingFile) reopen() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.file == nil {
		return os.ErrClosed
	}
	next, err := os.OpenFile(f.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	old := f.file
	f.file = next
	return old.Close()
}

func (f *rotatingFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.file == nil {
		return nil
	}
	err := f.file.Close()
	f.file = nil
	return err
}
