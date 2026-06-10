// Package release owns the timestamp format and the on-disk layout of
// /var/www/<app>/{releases,shared,current}. Centralizing it here means
// the rollback / status / releases / prune commands don't disagree with
// the deploy command about where to look.
package release

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"
)

// IDFormat is the lexicographically-sortable timestamp used as a release
// directory name. UTC so deploys from different timezones still sort.
const IDFormat = "20060102150405"

// ID is a release identifier (the timestamp string).
type ID string

// NewID returns the current UTC time as a release ID.
func NewID() ID {
	return ID(time.Now().UTC().Format(IDFormat))
}

// IsValidID returns true if s is in the canonical YYYYMMDDhhmmss form.
// Used when reading remote directory listings to skip stray files.
func IsValidID(s string) bool {
	if len(s) != len(IDFormat) {
		return false
	}
	_, err := time.Parse(IDFormat, s)
	return err == nil
}

func (i ID) String() string { return string(i) }

// Layout describes the remote release-root layout.
type Layout struct {
	Root string // e.g. "/var/www/webhook-relay"
}

// NewLayout returns the layout rooted at root.
func NewLayout(root string) Layout {
	return Layout{Root: strings.TrimRight(root, "/")}
}

// CurrentSymlink is the path of the live symlink.
func (l Layout) CurrentSymlink() string {
	return path.Join(l.Root, "current")
}

// CurrentNewSymlink is the path of the temporary symlink used during the
// atomic flip. We `ln -s ... current.new` then `mv -Tf current.new current`.
func (l Layout) CurrentNewSymlink() string {
	return path.Join(l.Root, "current.new")
}

// ReleasesDir is the directory holding all release subdirs.
func (l Layout) ReleasesDir() string {
	return path.Join(l.Root, "releases")
}

// ReleaseDir is the path of release <id>.
func (l Layout) ReleaseDir(id ID) string {
	return path.Join(l.ReleasesDir(), id.String())
}

// SharedDir is the directory holding files/dirs that persist across releases.
func (l Layout) SharedDir() string {
	return path.Join(l.Root, "shared")
}

// SharedFile returns the absolute path of a shared file by its config-relative name.
func (l Layout) SharedFile(name string) string {
	return path.Join(l.SharedDir(), name)
}

// IncomingDir holds artifacts that have been uploaded but not yet extracted.
func (l Layout) IncomingDir() string {
	return path.Join(l.Root, "_incoming")
}

// IncomingArtifact is the path the freshly-uploaded artifact lands at.
func (l Layout) IncomingArtifact(id ID, ext string) string {
	return path.Join(l.IncomingDir(), id.String()+ext)
}

// SortIDsDescending sorts in-place so the most recent ID is index 0.
func SortIDsDescending(ids []ID) {
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] > ids[j]
	})
}

// FilterValidIDs keeps only entries whose name parses as a release ID.
func FilterValidIDs(names []string) []ID {
	ids := make([]ID, 0, len(names))
	for _, n := range names {
		if IsValidID(n) {
			ids = append(ids, ID(n))
		}
	}
	return ids
}

// PrunePlan picks which release IDs to delete, given a sorted-descending
// list, a keep policy, and the currently-deployed release.
//
// Semantics: keep the top `keep` most-recent releases; always also keep
// `currentID` even if it falls outside the top window (this can happen
// after a rollback). When current is in the top window, total retained
// equals keep; when current is older, total retained equals keep+1.
func PrunePlan(sortedDesc []ID, keep int, currentID ID) []ID {
	if keep < 1 {
		keep = 1
	}
	keepSet := make(map[ID]bool, keep+1)
	for i, id := range sortedDesc {
		if i >= keep {
			break
		}
		keepSet[id] = true
	}
	if currentID != "" {
		keepSet[currentID] = true
	}
	var toDelete []ID
	for _, id := range sortedDesc {
		if !keepSet[id] {
			toDelete = append(toDelete, id)
		}
	}
	return toDelete
}

// FormatHuman renders the ID as a human-readable timestamp for status
// output. Returns the raw ID if it doesn't parse.
func (i ID) FormatHuman() string {
	t, err := time.Parse(IDFormat, string(i))
	if err != nil {
		return string(i)
	}
	return fmt.Sprintf("%s UTC", t.Format("2006-01-02 15:04:05"))
}
