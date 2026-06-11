// Package lockfile is a remote-host mutex implemented on top of SFTP.
//
// SFTP doesn't expose POSIX flock, so we use SFTP's open-with-CREATE-and-EXCL
// to atomically materialize a small JSON file. The file's mtime gates TTL
// staleness — if a previous deploy crashed and left the lock orphaned past
// the configured TTL, we steal it (logging loudly).
package lockfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"time"

	"github.com/pkg/sftp"
)

// Lock describes the file written to the remote lock path. It's not
// security state — it's diagnostic so a human can tell who else is in
// the middle of deploying.
type Lock struct {
	HeldBy     string    `json:"held_by"`
	Hostname   string    `json:"hostname"`
	PID        int       `json:"pid"`
	AcquiredAt time.Time `json:"acquired_at"`
}

// Errors returned by Acquire.
var (
	ErrAlreadyHeld = errors.New("lock is already held by another deploy")
)

// Handle is what Acquire returns. Call Release when done — the deferred
// Release in a deploy is critical.
type Handle struct {
	sftp *sftp.Client
	path string
}

// Acquire creates the lock file atomically. If the file already exists and
// its mtime is within ttl, returns ErrAlreadyHeld. If older than ttl, the
// caller is given the chance to steal via force.
//
// On steal, the existing lock content is returned so the caller can log
// who originally held it.
func Acquire(ctx context.Context, sc *sftp.Client, path string, ttl time.Duration, force bool) (*Handle, *Lock, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, false, err
	}

	// Try to create exclusively first. SFTP's OpenFile maps to fopen with
	// O_CREATE|O_EXCL when both flags are set.
	f, err := sc.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY)
	if err == nil {
		if err := writeLockFile(f); err != nil {
			f.Close()
			_ = sc.Remove(path)
			return nil, nil, false, err
		}
		f.Close()
		return &Handle{sftp: sc, path: path}, nil, false, nil
	}

	// Couldn't create — file might already exist. Check stat to see who
	// holds it and whether it's stale.
	info, statErr := sc.Stat(path)
	if statErr != nil {
		// Open failed and Stat failed too — surface the original error.
		return nil, nil, false, fmt.Errorf("acquire lock at %s: %w", path, err)
	}
	existing := readLockFileBestEffort(sc, path)
	stale := time.Since(info.ModTime()) >= ttl

	if !stale && !force {
		return nil, existing, false, ErrAlreadyHeld
	}

	// Steal the lock. Overwrite atomically by truncating and rewriting.
	f, err = sc.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		return nil, existing, false, fmt.Errorf("stealing stale lock at %s: %w", path, err)
	}
	if err := writeLockFile(f); err != nil {
		f.Close()
		return nil, existing, false, err
	}
	f.Close()
	return &Handle{sftp: sc, path: path}, existing, true, nil
}

// Release deletes the lock file. Idempotent — safe to call twice.
func (h *Handle) Release(ctx context.Context) error {
	if h == nil || h.sftp == nil {
		return nil
	}
	// Release ignores ctx.Err — holding a lock through a crash is the
	// worst possible state, so we attempt the remove unconditionally.
	if err := h.sftp.Remove(h.path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("releasing lock at %s: %w", h.path, err)
	}
	return nil
}

func writeLockFile(w io.Writer) error {
	lock := Lock{
		HeldBy:     localUser(),
		Hostname:   localHostname(),
		PID:        os.Getpid(),
		AcquiredAt: time.Now().UTC(),
	}
	return json.NewEncoder(w).Encode(lock)
}

func readLockFileBestEffort(sc *sftp.Client, path string) *Lock {
	f, err := sc.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var l Lock
	if err := json.NewDecoder(f).Decode(&l); err != nil {
		return nil
	}
	return &l
}

func localUser() string {
	u, err := user.Current()
	if err != nil || u == nil {
		return "unknown"
	}
	return u.Username
}

func localHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}
