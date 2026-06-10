package ssh

import (
	"fmt"
	"strconv"
	"strings"
)

// Target is a parsed "user@host[:port]" SSH destination.
type Target struct {
	User string
	Host string
	Port int
}

// String renders the target as "user@host:port".
func (t Target) String() string {
	return fmt.Sprintf("%s@%s:%d", t.User, t.Host, t.Port)
}

// Addr returns the dial address ("host:port").
func (t Target) Addr() string {
	return fmt.Sprintf("%s:%d", t.Host, t.Port)
}

// ParseTarget accepts "user@host" or "user@host:port". The port defaults
// to 22. IPv6 addresses in brackets ("user@[::1]:22") are supported.
func ParseTarget(s string) (Target, error) {
	at := strings.Index(s, "@")
	if at <= 0 || at == len(s)-1 {
		return Target{}, fmt.Errorf("invalid ssh target %q: expected user@host[:port]", s)
	}
	user := s[:at]
	rest := s[at+1:]

	host, portStr, err := splitHostPort(rest)
	if err != nil {
		return Target{}, fmt.Errorf("invalid ssh target %q: %w", s, err)
	}

	port := 22
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil || p < 1 || p > 65535 {
			return Target{}, fmt.Errorf("invalid ssh target %q: port %q out of range", s, portStr)
		}
		port = p
	}

	return Target{User: user, Host: host, Port: port}, nil
}

// splitHostPort handles plain ("host:port"), no-port ("host"), and
// bracketed-IPv6 ("[::1]:22" or "[::1]") forms.
func splitHostPort(s string) (host, port string, err error) {
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end < 0 {
			return "", "", fmt.Errorf("missing ] in bracketed host")
		}
		host = s[1:end]
		rest := s[end+1:]
		switch {
		case rest == "":
			return host, "", nil
		case strings.HasPrefix(rest, ":"):
			return host, rest[1:], nil
		default:
			return "", "", fmt.Errorf("unexpected trailing data after ]")
		}
	}

	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 1 {
		return parts[0], "", nil
	}
	return parts[0], parts[1], nil
}
