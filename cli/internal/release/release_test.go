package release

import (
	"reflect"
	"testing"
)

func TestIDValidation(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"20260610120000": true,
		"99991231235959": true,
		"":               false,
		"20260610":       false,
		"2026/06/10/12":  false,
		"latest":         false,
		"2026061012000a": false,
	}
	for s, want := range cases {
		if got := IsValidID(s); got != want {
			t.Errorf("IsValidID(%q) = %v; want %v", s, got, want)
		}
	}
}

func TestLayoutPaths(t *testing.T) {
	t.Parallel()
	l := NewLayout("/var/www/webhook-relay/")
	if got, want := l.CurrentSymlink(), "/var/www/webhook-relay/current"; got != want {
		t.Errorf("CurrentSymlink = %q; want %q", got, want)
	}
	if got, want := l.CurrentNewSymlink(), "/var/www/webhook-relay/current.new"; got != want {
		t.Errorf("CurrentNewSymlink = %q; want %q", got, want)
	}
	if got, want := l.ReleaseDir("20260610120000"), "/var/www/webhook-relay/releases/20260610120000"; got != want {
		t.Errorf("ReleaseDir = %q; want %q", got, want)
	}
	if got, want := l.SharedFile(".env"), "/var/www/webhook-relay/shared/.env"; got != want {
		t.Errorf("SharedFile = %q; want %q", got, want)
	}
}

func TestFilterAndSort(t *testing.T) {
	t.Parallel()
	names := []string{"20260610120000", "_incoming", "shared", "20260101000000", "junk", "20260315090000"}
	ids := FilterValidIDs(names)
	SortIDsDescending(ids)
	want := []ID{"20260610120000", "20260315090000", "20260101000000"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("got %v; want %v", ids, want)
	}
}

func TestPrunePlanRespectsKeepAndCurrent(t *testing.T) {
	t.Parallel()
	sorted := []ID{"20260610", "20260609", "20260608", "20260607", "20260606", "20260605"}

	// keep=3, current is index 0 — top 3 = {10,09,08}, delete the rest.
	got := PrunePlan(sorted, 3, "20260610")
	want := []ID{"20260607", "20260606", "20260605"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("current=20260610: got %v; want %v", got, want)
	}

	// keep=3, current=20260608 (inside top 3) — same as above.
	got = PrunePlan(sorted, 3, "20260608")
	want = []ID{"20260607", "20260606", "20260605"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("current=20260608: got %v; want %v", got, want)
	}

	// keep=3, current=20260606 (outside top 3, post-rollback). Top 3 plus
	// current = 4 retained; delete 20260607 + 20260605.
	got = PrunePlan(sorted, 3, "20260606")
	want = []ID{"20260607", "20260605"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("current=20260606 (post-rollback): got %v; want %v", got, want)
	}

	// keep=1, current=20260610 — only current retained.
	got = PrunePlan(sorted, 1, "20260610")
	want = []ID{"20260609", "20260608", "20260607", "20260606", "20260605"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keep=1: got %v; want %v", got, want)
	}
}

func TestFormatHuman(t *testing.T) {
	t.Parallel()
	id := ID("20260610120000")
	want := "2026-06-10 12:00:00 UTC"
	if got := id.FormatHuman(); got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}
