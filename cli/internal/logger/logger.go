// Package logger is the shipyard CLI's structured logger.
//
// Two outputs, two formats:
//   - stderr (always): JSON, one event per line, for machine consumption
//     and for the docs-site "live log replay" page.
//   - stdout (when TTY, or when --verbose): pretty-printed, line-prefixed
//     with the phase the deploy is in.
package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// Logger wraps two slog handlers: a JSON sink (stderr) and a pretty
// sink (stdout). Phase prefixes are managed externally via WithPhase
// so the wrapping code can choose phase boundaries without each call
// site repeating itself.
type Logger struct {
	mu      sync.Mutex
	json    *slog.Logger
	pretty  io.Writer
	color   bool
	phase   string
	verbose bool
}

// Options configures NewLogger.
type Options struct {
	JSONOut   io.Writer // stderr by default
	PrettyOut io.Writer // stdout by default
	Color     bool      // honored only if PrettyOut is a TTY
	Verbose   bool      // emits pretty output even when no TTY
}

// NewLogger builds a Logger. Pass empty Options for sensible defaults.
func NewLogger(opts Options) *Logger {
	if opts.JSONOut == nil {
		opts.JSONOut = os.Stderr
	}
	if opts.PrettyOut == nil {
		opts.PrettyOut = os.Stdout
	}
	jsonHandler := slog.NewJSONHandler(opts.JSONOut, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	return &Logger{
		json:    slog.New(jsonHandler),
		pretty:  opts.PrettyOut,
		color:   opts.Color && isTTY(opts.PrettyOut),
		verbose: opts.Verbose,
	}
}

// WithPhase returns a child logger that tags subsequent records with phase.
// Phases line up with the deploy lifecycle: "upload", "extract",
// "post-extract", "flip", "post-flip", "health", "prune", "rollback".
func (l *Logger) WithPhase(phase string) *Logger {
	clone := *l
	clone.phase = phase
	return &clone
}

// Info logs an informational event.
func (l *Logger) Info(msg string, attrs ...slog.Attr) {
	l.emit(slog.LevelInfo, msg, attrs)
}

// Warn logs a non-fatal warning.
func (l *Logger) Warn(msg string, attrs ...slog.Attr) {
	l.emit(slog.LevelWarn, msg, attrs)
}

// Error logs a fatal event but does not exit.
func (l *Logger) Error(msg string, attrs ...slog.Attr) {
	l.emit(slog.LevelError, msg, attrs)
}

// Debug logs a verbose-only event. Pretty output is suppressed unless
// verbose was enabled on construction.
func (l *Logger) Debug(msg string, attrs ...slog.Attr) {
	l.emit(slog.LevelDebug, msg, attrs)
}

func (l *Logger) emit(level slog.Level, msg string, attrs []slog.Attr) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// JSON sink — always.
	if l.phase != "" {
		attrs = append([]slog.Attr{slog.String("phase", l.phase)}, attrs...)
	}
	l.json.LogAttrs(nil, level, msg, attrs...)

	// Pretty sink — only when verbose, or when level >= Info.
	if level < slog.LevelInfo && !l.verbose {
		return
	}

	prefix := l.phasePrefix(level)
	suffix := formatAttrs(attrs)
	if suffix != "" {
		_, _ = fmt.Fprintf(l.pretty, "%s %s %s\n", prefix, msg, suffix)
	} else {
		_, _ = fmt.Fprintf(l.pretty, "%s %s\n", prefix, msg)
	}
}

func (l *Logger) phasePrefix(level slog.Level) string {
	icon := ""
	switch level {
	case slog.LevelError:
		icon = "✗"
	case slog.LevelWarn:
		icon = "!"
	default:
		icon = "▸"
	}

	phase := l.phase
	if phase == "" {
		phase = "shipyard"
	}
	prefix := fmt.Sprintf("[%s] %s", phase, icon)
	if !l.color {
		return prefix
	}
	switch level {
	case slog.LevelError:
		return "\x1b[31m" + prefix + "\x1b[0m"
	case slog.LevelWarn:
		return "\x1b[33m" + prefix + "\x1b[0m"
	default:
		return "\x1b[36m" + prefix + "\x1b[0m"
	}
}

func formatAttrs(attrs []slog.Attr) string {
	if len(attrs) == 0 {
		return ""
	}
	var b strings.Builder
	for i, a := range attrs {
		if a.Key == "phase" {
			continue
		}
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(a.Key)
		b.WriteByte('=')
		b.WriteString(formatValue(a.Value))
	}
	return b.String()
}

func formatValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		s := v.String()
		if strings.ContainsAny(s, " \t") {
			b, _ := json.Marshal(s)
			return string(b)
		}
		return s
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time().Format(time.RFC3339)
	default:
		return v.String()
	}
}

// isTTY returns true if w looks like a terminal. We don't import golang.org/x/term
// just for this — checking the os.File against os.Stdout / os.Stderr's Stat()
// would work but the simpler check (file mode) is enough for now.
func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
