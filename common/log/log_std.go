//go:build !android || !cgo

package log

import (
	"log/slog"
	"os"
)

func init() {
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))
}
