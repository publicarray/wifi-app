package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func init() {
	slog.SetDefault(newBaseLogger())
}

// newBaseLogger builds the process-wide slog logger used before the Wails ctx
// is available. Format defaults to text on a TTY (dev / interactive run) and
// JSON otherwise (journalctl / service logs). WIFI_APP_LOG_FORMAT=text|json
// overrides the auto-detection; WIFI_APP_LOG_LEVEL=debug|info|warn|error sets
// the minimum level.
func newBaseLogger() *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLogLevel(os.Getenv("WIFI_APP_LOG_LEVEL"))}

	format := strings.ToLower(os.Getenv("WIFI_APP_LOG_FORMAT"))
	if format == "" {
		if isTerminal(os.Stderr) {
			format = "text"
		} else {
			format = "json"
		}
	}

	var h slog.Handler
	switch format {
	case "text":
		h = slog.NewTextHandler(os.Stderr, opts)
	default:
		h = slog.NewJSONHandler(os.Stderr, opts)
	}
	return slog.New(h)
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// installWailsForwarding wraps the default slog logger with a handler that
// also forwards Warn+ records to the Wails runtime logger. This keeps warnings
// and errors visible in `wails dev` devtools and in the Wails-configured log
// sink after callers have been migrated off runtime.LogXxx.
func installWailsForwarding(ctx context.Context) {
	base := slog.Default().Handler()
	slog.SetDefault(slog.New(&wailsForwardingHandler{base: base, ctx: ctx}))
}

type wailsForwardingHandler struct {
	base slog.Handler
	ctx  context.Context
}

func (h *wailsForwardingHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.base.Enabled(ctx, l)
}

func (h *wailsForwardingHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelWarn && h.ctx != nil {
		// Wails logger takes a single unstructured string — flatten attrs into
		// "key=value" pairs so structured fields (interface, bssid, err, ...)
		// survive the hop.
		var sb strings.Builder
		sb.WriteString(r.Message)
		r.Attrs(func(a slog.Attr) bool {
			sb.WriteByte(' ')
			sb.WriteString(a.Key)
			sb.WriteByte('=')
			sb.WriteString(a.Value.String())
			return true
		})
		msg := sb.String()
		if r.Level >= slog.LevelError {
			runtime.LogError(h.ctx, msg)
		} else {
			runtime.LogWarning(h.ctx, msg)
		}
	}
	return h.base.Handle(ctx, r)
}

func (h *wailsForwardingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &wailsForwardingHandler{base: h.base.WithAttrs(attrs), ctx: h.ctx}
}

func (h *wailsForwardingHandler) WithGroup(name string) slog.Handler {
	return &wailsForwardingHandler{base: h.base.WithGroup(name), ctx: h.ctx}
}
