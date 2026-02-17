// Package logger provides centralized logging for igent using slog.
package logger

import (
	"io"
	"log/slog"
	"os"
)

// Level represents a log level.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Format represents the log output format.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Config holds logger configuration.
type Config struct {
	Level  Level  `mapstructure:"level"`
	Format Format `mapstructure:"format"`
}

// DefaultConfig returns default logger configuration.
func DefaultConfig() Config {
	return Config{
		Level:  LevelInfo,
		Format: FormatText,
	}
}

var (
	defaultLogger *slog.Logger
)

// Init initializes the default logger with the given configuration.
func Init(cfg Config, output io.Writer) {
	if output == nil {
		output = os.Stderr
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: parseLevel(cfg.Level),
	}

	switch cfg.Format {
	case FormatJSON:
		handler = slog.NewJSONHandler(output, opts)
	default:
		handler = slog.NewTextHandler(output, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

// parseLevel converts string level to slog.Level.
func parseLevel(level Level) slog.Level {
	switch level {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// L returns the default logger.
func L() *slog.Logger {
	if defaultLogger == nil {
		Init(DefaultConfig(), nil)
	}
	return defaultLogger
}

// Debug logs at debug level.
func Debug(msg string, args ...any) {
	L().Debug(msg, args...)
}

// Info logs at info level.
func Info(msg string, args ...any) {
	L().Info(msg, args...)
}

// Warn logs at warn level.
func Warn(msg string, args ...any) {
	L().Warn(msg, args...)
}

// Error logs at error level.
func Error(msg string, args ...any) {
	L().Error(msg, args...)
}

// With returns a logger with additional context.
func With(args ...any) *slog.Logger {
	return L().With(args...)
}
