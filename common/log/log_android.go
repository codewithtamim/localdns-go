//go:build android && cgo

package log

// #cgo LDFLAGS: -llog
// #include <stdlib.h>
// #include <android/log.h>
import "C"

import (
	"context"
	"log/slog"
	"strings"
	"unsafe"
)

const maxAndroidLogLine = 3800

// Log tag for adb logcat: adb logcat -s LocalDNS
var localdnsGoTag *C.char

func init() {
	localdnsGoTag = C.CString("LocalDNS")
	logger = slog.New(&androidHandler{
		tag: localdnsGoTag,
		min: slog.LevelDebug,
	})
}

// androidHandler formats records like slog's text handler, then writes to Android logcat via
// __android_log_write (same pattern as WireGuard's libwg-go).
type androidHandler struct {
	tag    *C.char
	min    slog.Level
	attrs  []slog.Attr
	groups []string
}

func (h *androidHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.min
}

func (h *androidHandler) Handle(ctx context.Context, r slog.Record) error {
	var b strings.Builder
	var sub slog.Handler = slog.NewTextHandler(&b, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	if len(h.attrs) > 0 {
		sub = sub.WithAttrs(h.attrs)
	}
	for _, g := range h.groups {
		sub = sub.WithGroup(g)
	}
	if err := sub.Handle(ctx, r); err != nil {
		return err
	}
	line := strings.TrimSpace(b.String())
	if len(line) > maxAndroidLogLine {
		line = line[:maxAndroidLogLine] + "...(truncated)"
	}
	cs := C.CString(line)
	defer C.free(unsafe.Pointer(cs))
	C.__android_log_write(androidPrio(r.Level), h.tag, cs)
	return nil
}

func androidPrio(l slog.Level) C.int {
	switch {
	case l >= slog.LevelError:
		return C.ANDROID_LOG_ERROR
	case l >= slog.LevelWarn:
		return C.ANDROID_LOG_WARN
	case l >= slog.LevelInfo:
		return C.ANDROID_LOG_INFO
	default:
		return C.ANDROID_LOG_DEBUG
	}
}

func (h *androidHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := *h
	h2.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &h2
}

func (h *androidHandler) WithGroup(name string) slog.Handler {
	h2 := *h
	h2.groups = append(append([]string{}, h.groups...), name)
	return &h2
}
