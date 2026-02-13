package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lmittmann/tint"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config holds logger configuration.
type Config struct {
	LogDir     string
	LogFile    string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	DevMode    bool
	Level      slog.Level
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		LogDir:     "./logs",
		LogFile:    "app.log",
		MaxSizeMB:  50,
		MaxBackups: 5,
		MaxAgeDays: 30,
		DevMode:    true,
		Level:      slog.LevelInfo,
	}
}

// MultiHandler fans out log records to multiple slog handlers.
type MultiHandler struct {
	handlers []slog.Handler
}

func (m *MultiHandler) Enabled(_ context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(context.Background(), level) {
			return true
		}
	}
	return false
}

func (m *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: handlers}
}

func (m *MultiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &MultiHandler{handlers: handlers}
}

// New creates a dual-output logger: JSON rolling file + pretty/text console.
// Returns the logger and a Closer for the file writer.
func New(cfg Config) (*slog.Logger, io.Closer) {
	os.MkdirAll(cfg.LogDir, 0o755)

	logPath := filepath.Join(cfg.LogDir, cfg.LogFile)

	// Rolling JSON file writer
	lj := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		LocalTime:  true,
	}

	jsonOpts := &slog.HandlerOptions{Level: cfg.Level}
	fileHandler := slog.NewJSONHandler(lj, jsonOpts)

	// Console handler
	var consoleHandler slog.Handler
	if cfg.DevMode {
		consoleHandler = tint.NewHandler(os.Stderr, &tint.Options{
			Level:      cfg.Level,
			TimeFormat: "15:04:05",
		})
	} else {
		consoleHandler = slog.NewTextHandler(os.Stderr, jsonOpts)
	}

	multi := &MultiHandler{handlers: []slog.Handler{fileHandler, consoleHandler}}
	return slog.New(multi), lj
}
