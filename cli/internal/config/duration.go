package config

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a YAML-friendly wrapper around time.Duration so users can write
// "3s", "10m", "1h" in shipyard.yaml. yaml.v3 doesn't unmarshal time.Duration
// natively (it'd need a numeric value of nanoseconds), and that would be a
// terrible UX.
type Duration time.Duration

// UnmarshalYAML accepts either a string ("30s") or an integer (treated as
// seconds — anything else would be a footgun).
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		switch value.Tag {
		case "!!str", "":
			parsed, err := time.ParseDuration(value.Value)
			if err != nil {
				return fmt.Errorf("invalid duration %q at line %d: %w", value.Value, value.Line, err)
			}
			*d = Duration(parsed)
			return nil
		case "!!int":
			var seconds int
			if err := value.Decode(&seconds); err != nil {
				return fmt.Errorf("invalid duration at line %d: %w", value.Line, err)
			}
			*d = Duration(time.Duration(seconds) * time.Second)
			return nil
		}
	}
	return fmt.Errorf("invalid duration at line %d: expected string like %q or integer seconds", value.Line, "30s")
}

// MarshalYAML emits the duration as a Go-style string so round-trips stay
// human-readable.
func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// String returns the Go-style string form (e.g. "30s", "1h10m").
func (d Duration) String() string {
	return time.Duration(d).String()
}
