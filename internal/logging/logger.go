package logging

import (
	"context"
	"log/slog"
	"os"
	"time"

	"proxyrouter/internal/config"
)

// Logger provides structured logging functionality
type Logger struct {
	logger *slog.Logger
}

// New creates a new logger instance
func New(cfg *config.LoggingConfig) *Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}

	logger := slog.New(handler)
	return &Logger{logger: logger}
}

// WithRequestID adds request ID to the logger context
func (l *Logger) WithRequestID(ctx context.Context, requestID string) *slog.Logger {
	return l.logger.With("request_id", requestID)
}

// WithClientIP adds client IP to the logger context
func (l *Logger) WithClientIP(ctx context.Context, clientIP string) *slog.Logger {
	return l.logger.With("client_ip", clientIP)
}

// WithTargetHost adds target host to the logger context
func (l *Logger) WithTargetHost(ctx context.Context, targetHost string) *slog.Logger {
	return l.logger.With("target_host", targetHost)
}

// WithRoute adds route information to the logger context
func (l *Logger) WithRoute(ctx context.Context, routeGroup string, precedence int) *slog.Logger {
	return l.logger.With("route_group", routeGroup, "route_precedence", precedence)
}

// WithProxy adds proxy information to the logger context
func (l *Logger) WithProxy(ctx context.Context, proxyID int, proxyType, ip string, port int) *slog.Logger {
	return l.logger.With(
		"proxy_id", proxyID,
		"proxy_type", proxyType,
		"proxy_ip", ip,
		"proxy_port", port,
	)
}

// WithDuration adds duration to the logger context
func (l *Logger) WithDuration(ctx context.Context, duration time.Duration) *slog.Logger {
	return l.logger.With("duration_ms", duration.Milliseconds())
}

// WithError adds error to the logger context
func (l *Logger) WithError(ctx context.Context, err error) *slog.Logger {
	return l.logger.With("error", err.Error())
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

// GetLogger returns the underlying slog logger
func (l *Logger) GetLogger() *slog.Logger {
	return l.logger
}
