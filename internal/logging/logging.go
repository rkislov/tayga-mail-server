package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
	w := io.MultiWriter(os.Stdout, f)
	var handler slog.Handler
	if strings.EqualFold(format, "text") {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}
	closeFn := func() error { return f.Close() }
	return slog.New(handler), closeFn, nil
}
