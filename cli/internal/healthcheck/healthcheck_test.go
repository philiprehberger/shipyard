package healthcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPassesOnFirstAttempt(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"healthy": true}`))
	}))
	defer srv.Close()

	res, err := Run(context.Background(), Probe{
		URL:     srv.URL,
		Expect:  "healthy",
		Retries: 3,
		Delay:   10 * time.Millisecond,
		Timeout: 1 * time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Attempts != 1 {
		t.Errorf("Attempts = %d; want 1", res.Attempts)
	}
	if res.LastStatus != 200 {
		t.Errorf("LastStatus = %d; want 200", res.LastStatus)
	}
}

func TestRetriesUntilHealthy(t *testing.T) {
	t.Parallel()

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 3 {
			http.Error(w, "warming up", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	res, err := Run(context.Background(), Probe{
		URL:     srv.URL,
		Retries: 5,
		Delay:   5 * time.Millisecond,
		Timeout: 1 * time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Attempts != 3 {
		t.Errorf("Attempts = %d; want 3", res.Attempts)
	}
}

func TestFailsAfterRetriesExhausted(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer srv.Close()

	res, err := Run(context.Background(), Probe{
		URL:     srv.URL,
		Retries: 3,
		Delay:   1 * time.Millisecond,
		Timeout: 1 * time.Second,
	}, nil)
	if err == nil {
		t.Fatal("expected failure")
	}
	if res.Attempts != 3 {
		t.Errorf("Attempts = %d; want 3", res.Attempts)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention last status; got %v", err)
	}
}

func TestFailsWhenExpectSubstringMissing(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("running but not the right shape"))
	}))
	defer srv.Close()

	_, err := Run(context.Background(), Probe{
		URL:     srv.URL,
		Expect:  "healthy",
		Retries: 2,
		Delay:   1 * time.Millisecond,
		Timeout: 1 * time.Second,
	}, nil)
	if err == nil {
		t.Fatal("expected failure due to expect-substring missing")
	}
}

func TestObserverCalledPerAttempt(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	defer srv.Close()

	var observed int32
	_, _ = Run(context.Background(), Probe{
		URL:     srv.URL,
		Retries: 4,
		Delay:   1 * time.Millisecond,
		Timeout: 1 * time.Second,
	}, func(attempt int, err error, status int) {
		atomic.AddInt32(&observed, 1)
	})
	if got := atomic.LoadInt32(&observed); got != 4 {
		t.Errorf("observer called %d times; want 4", got)
	}
}

func TestCancelContextStopsRetries(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := Run(ctx, Probe{
		URL:     srv.URL,
		Retries: 100,
		Delay:   50 * time.Millisecond,
		Timeout: 1 * time.Second,
	}, nil)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}
