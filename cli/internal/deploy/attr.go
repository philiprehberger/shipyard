package deploy

import (
	"log/slog"
	"time"
)

// logAttr is a tiny helper so call sites can write
//
//	log.Info("uploaded", logAttr("bytes", n))
//
// without importing slog directly.
func logAttr(k string, v any) slog.Attr {
	switch val := v.(type) {
	case string:
		return slog.String(k, val)
	case int:
		return slog.Int(k, val)
	case int64:
		return slog.Int64(k, val)
	case time.Duration:
		return slog.Duration(k, val)
	default:
		return slog.Any(k, v)
	}
}
