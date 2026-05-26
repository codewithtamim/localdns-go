// Package log provides structured logging for the native tunnel runtime.
package log

import (
	"context"
	"fmt"
	"log/slog"
)

// logger is configured in init by log_std.go or log_android.go.
var logger *slog.Logger

func Debug(msg string, args ...any) {
	logger.Log(context.Background(), slog.LevelDebug, msg, args...)
}

func Info(msg string, args ...any) {
	logger.Log(context.Background(), slog.LevelInfo, msg, args...)
}

func Warn(msg string, args ...any) {
	logger.Log(context.Background(), slog.LevelWarn, msg, args...)
}

func Error(msg string, args ...any) {
	logger.Log(context.Background(), slog.LevelError, msg, args...)
}

func Debugf(format string, args ...any) {
	logger.Log(context.Background(), slog.LevelDebug, fmt.Sprintf(format, args...))
}

func Infof(format string, args ...any) {
	logger.Log(context.Background(), slog.LevelInfo, fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...any) {
	logger.Log(context.Background(), slog.LevelWarn, fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...any) {
	logger.Log(context.Background(), slog.LevelError, fmt.Sprintf(format, args...))
}
