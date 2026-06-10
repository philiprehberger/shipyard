// Package healthcheck probes an HTTP endpoint with retries until it
// either returns the expected status + substring or runs out of attempts.
package healthcheck

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Probe describes one health-check attempt configuration.
type Probe struct {
	URL     string        // required; must be http:// or https://
	Status  int           // required; default 200 if unset
	Expect  string        // optional substring; empty = no body check
	Retries int           // total attempts; default 10
	Delay   time.Duration // between attempts; default 3s
	Timeout time.Duration // per-request; default 5s
}

// Result describes the outcome of a Run.
type Result struct {
	Attempts   int           // attempts made (1..Retries)
	LastStatus int           // last status code observed (0 = never reached server)
	LastBody   string        // truncated body of last response
	Elapsed    time.Duration // total time spent across retries
}

// AttemptObserver is called once per attempt — used by the deploy logger
// to surface per-attempt progress without coupling this package to slog.
type AttemptObserver func(attempt int, err error, status int)

// Run probes until success or out of retries. The error is non-nil if
// the probe never passed; the Result is always populated.
func Run(ctx context.Context, p Probe, observe AttemptObserver) (*Result, error) {
	if p.URL == "" {
		return nil, errors.New("URL is required")
	}
	if p.Status == 0 {
		p.Status = http.StatusOK
	}
	if p.Retries < 1 {
		p.Retries = 10
	}
	if p.Delay <= 0 {
		p.Delay = 3 * time.Second
	}
	if p.Timeout <= 0 {
		p.Timeout = 5 * time.Second
	}
	if observe == nil {
		observe = func(int, error, int) {}
	}

	client := &http.Client{
		Timeout: p.Timeout,
		// Don't follow redirects — a healthy app's healthz endpoint
		// should return 2xx directly, not 3xx then 2xx. If your app
		// genuinely needs a redirect-followed health check, point the
		// URL at the final destination.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	result := &Result{}
	start := time.Now()
	defer func() { result.Elapsed = time.Since(start) }()

	for attempt := 1; attempt <= p.Retries; attempt++ {
		result.Attempts = attempt

		if err := ctx.Err(); err != nil {
			return result, err
		}

		status, body, err := probe(ctx, client, p.URL)
		result.LastStatus = status
		result.LastBody = body

		observe(attempt, err, status)

		if err == nil && status == p.Status && (p.Expect == "" || strings.Contains(body, p.Expect)) {
			return result, nil
		}

		if attempt < p.Retries {
			select {
			case <-time.After(p.Delay):
			case <-ctx.Done():
				return result, ctx.Err()
			}
		}
	}

	return result, fmt.Errorf("health check failed after %d attempts (last status=%d)", result.Attempts, result.LastStatus)
}

func probe(ctx context.Context, client *http.Client, url string) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("User-Agent", "shipyard-healthcheck")
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	// Bound the body read to 64 KiB so a misconfigured endpoint serving
	// a huge response doesn't OOM the deploy.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return resp.StatusCode, "", err
	}
	return resp.StatusCode, string(body), nil
}
