package version

import (
	"strings"
	"testing"
)

func TestStringContainsAllThreeFields(t *testing.T) {
	t.Parallel()

	// String must surface all three ldflag-injected fields so users can
	// reproduce a build from `shipyard version` output alone.
	got := String()
	for _, want := range []string{Version, Commit, Date} {
		if !strings.Contains(got, want) {
			t.Errorf("String() = %q; expected to contain %q", got, want)
		}
	}
}

func TestStringDefaultsWhenNotInjected(t *testing.T) {
	t.Parallel()

	if Version == "" || Commit == "" || Date == "" {
		t.Fatal("Version, Commit, Date defaults must be non-empty so `shipyard version` does not look broken in a `go run` build")
	}
}
