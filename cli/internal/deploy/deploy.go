// Package deploy is the atomic-release orchestrator. It assembles a
// config + an SSH client + a logger + a release timestamp and walks the
// 13-step lifecycle documented in the build plan.
package deploy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/philiprehberger/shipyard/cli/internal/config"
	"github.com/philiprehberger/shipyard/cli/internal/healthcheck"
	"github.com/philiprehberger/shipyard/cli/internal/lockfile"
	"github.com/philiprehberger/shipyard/cli/internal/logger"
	"github.com/philiprehberger/shipyard/cli/internal/release"
	"github.com/philiprehberger/shipyard/cli/internal/ssh"
)

// ExitCode classifies which stage of the lifecycle failed. The CLI maps
// these to documented process exit codes 0..5.
type ExitCode int

const (
	ExitSuccess       ExitCode = 0
	ExitUsage         ExitCode = 1
	ExitTransport     ExitCode = 2
	ExitDeployStep    ExitCode = 3
	ExitHealthFailure ExitCode = 4
	ExitLockHeld      ExitCode = 5
)

// CodedError tags an error with the exit code that should be returned
// to the OS. Use errors.As to extract the code at the CLI boundary.
type CodedError struct {
	Code    ExitCode
	Wrapped error
}

func (e *CodedError) Error() string { return e.Wrapped.Error() }
func (e *CodedError) Unwrap() error { return e.Wrapped }

func coded(code ExitCode, format string, args ...any) error {
	return &CodedError{Code: code, Wrapped: fmt.Errorf(format, args...)}
}

// Options configures Run.
type Options struct {
	Config            *config.Config
	Log               *logger.Logger
	DryRun            bool
	SkipHealth        bool
	ForceLockSteal    bool
	ReleaseIDOverride string
}

// Result is what Run returns on the happy path.
type Result struct {
	ReleaseID      release.ID
	PreviousID     release.ID
	HealthAttempts int
	Rolled         bool
}

// Run executes the deploy lifecycle and returns when it has either
// successfully promoted a new release or rolled back to the previous one.
//
// Phases logged: connect, lock, upload, extract, shared-links,
// post-extract, flip, post-flip, health, prune, rollback (when needed).
func Run(ctx context.Context, opts Options) (*Result, error) {
	if opts.Config == nil {
		return nil, coded(ExitUsage, "config is required")
	}
	if opts.Log == nil {
		opts.Log = logger.NewLogger(logger.Options{})
	}

	cfg := opts.Config
	log := opts.Log
	layout := release.NewLayout(cfg.Host.ReleaseRoot)

	releaseID := release.ID(opts.ReleaseIDOverride)
	if releaseID == "" {
		releaseID = release.NewID()
	} else if !release.IsValidID(string(releaseID)) {
		return nil, coded(ExitUsage, "--release-id %q is not a valid YYYYMMDDhhmmss timestamp", releaseID)
	}

	// ─── Phase: pre_upload hooks (local) ───
	preLog := log.WithPhase("pre-upload")
	for _, h := range cfg.Hooks.PreUpload {
		if opts.DryRun {
			preLog.Info("would run local hook", logAttr("cmd", h))
			continue
		}
		if err := runLocalHook(ctx, preLog, h); err != nil {
			return nil, coded(ExitDeployStep, "pre_upload hook: %w", err)
		}
	}

	// Verify the artifact exists locally before we touch the network.
	artifactPath, err := resolveArtifactPath(cfg)
	if err != nil {
		return nil, coded(ExitUsage, "artifact: %w", err)
	}
	if _, err := os.Stat(artifactPath); err != nil {
		return nil, coded(ExitUsage, "artifact %s missing after pre_upload hooks: %w", artifactPath, err)
	}

	// ─── Phase: connect ───
	connLog := log.WithPhase("connect")
	target, err := ssh.ParseTarget(cfg.Host.SSH)
	if err != nil {
		return nil, coded(ExitUsage, "host.ssh: %w", err)
	}
	connLog.Info("dialing", logAttr("target", target.String()))

	sshC, err := ssh.Connect(ctx, target, ssh.ConnectOpts{
		IdentityFile: cfg.Host.IdentityFile,
		Timeout:      15 * time.Second,
	})
	if err != nil {
		return nil, coded(ExitTransport, "ssh: %w", err)
	}
	defer sshC.Close()
	connLog.Info("connected")

	if opts.DryRun {
		log.Info("dry-run — stopping after connect")
		return &Result{ReleaseID: releaseID}, nil
	}

	// ─── Phase: lock ───
	var handle *lockfile.Handle
	if cfg.Lock.IsEnabled() {
		lockLog := log.WithPhase("lock")
		if err := sshC.MkdirAll(ctx, path.Dir(cfg.Lock.Path)); err != nil {
			return nil, coded(ExitTransport, "preparing lock dir: %w", err)
		}
		h, prev, stole, err := lockfile.Acquire(ctx, sshC.SFTP(), cfg.Lock.Path, cfg.Lock.TTL.Duration(), opts.ForceLockSteal)
		if err != nil {
			if errors.Is(err, lockfile.ErrAlreadyHeld) {
				if prev != nil {
					return nil, coded(ExitLockHeld, "lock held by %s@%s (pid %d) since %s — pass --force to steal",
						prev.HeldBy, prev.Hostname, prev.PID, prev.AcquiredAt.Format(time.RFC3339))
				}
				return nil, coded(ExitLockHeld, "lock already held")
			}
			return nil, coded(ExitTransport, "acquire lock: %w", err)
		}
		handle = h
		if stole {
			lockLog.Warn("stole stale lock", logAttr("held_by_prev", lockHolder(prev)))
		} else {
			lockLog.Info("acquired", logAttr("path", cfg.Lock.Path))
		}
		defer func() {
			if relErr := handle.Release(context.Background()); relErr != nil {
				lockLog.Error("releasing lock", logAttr("err", relErr.Error()))
			} else {
				lockLog.Info("released")
			}
		}()
	}

	// ─── Phase: prep dirs ───
	if err := sshC.MkdirAll(ctx, layout.ReleasesDir()); err != nil {
		return nil, coded(ExitTransport, "mkdir releases dir: %w", err)
	}
	if err := sshC.MkdirAll(ctx, layout.SharedDir()); err != nil {
		return nil, coded(ExitTransport, "mkdir shared dir: %w", err)
	}
	if err := sshC.MkdirAll(ctx, layout.IncomingDir()); err != nil {
		return nil, coded(ExitTransport, "mkdir _incoming dir: %w", err)
	}

	// Determine the previous release (might be empty on first deploy).
	previousID, err := currentReleaseID(ctx, sshC, layout)
	if err != nil {
		return nil, coded(ExitTransport, "reading current symlink: %w", err)
	}

	result := &Result{ReleaseID: releaseID, PreviousID: previousID}

	// ─── Phase: upload ───
	upLog := log.WithPhase("upload")
	ext := filepath.Ext(artifactPath)
	if cfg.Artifact.Format == "tar.gz" {
		ext = ".tar.gz"
	}
	remoteArtifact := layout.IncomingArtifact(releaseID, ext)
	upLog.Info("uploading", logAttr("from", artifactPath), logAttr("to", remoteArtifact))
	if err := sshC.Upload(ctx, artifactPath, remoteArtifact); err != nil {
		return nil, coded(ExitTransport, "upload artifact: %w", err)
	}

	// ─── Phase: extract ───
	exLog := log.WithPhase("extract")
	releaseDir := layout.ReleaseDir(releaseID)
	exLog.Info("extracting", logAttr("into", releaseDir))
	if err := sshC.MkdirAll(ctx, releaseDir); err != nil {
		return nil, coded(ExitTransport, "mkdir release dir: %w", err)
	}
	if err := extractRemote(ctx, sshC, cfg.Artifact.Format, remoteArtifact, releaseDir); err != nil {
		_ = sshC.RemoveAll(ctx, releaseDir)
		_ = sshC.RemoveAll(ctx, remoteArtifact)
		return nil, coded(ExitDeployStep, "extract: %w", err)
	}
	_ = sshC.RemoveAll(ctx, remoteArtifact)

	// ─── Phase: shared-links ───
	shLog := log.WithPhase("shared-links")
	for _, f := range cfg.Shared.Files {
		src := layout.SharedFile(f)
		dst := path.Join(releaseDir, f)
		ok, err := sshC.Exists(ctx, src)
		if err != nil {
			_ = sshC.RemoveAll(ctx, releaseDir)
			return nil, coded(ExitTransport, "stat shared %s: %w", src, err)
		}
		if !ok {
			_ = sshC.RemoveAll(ctx, releaseDir)
			return nil, coded(ExitDeployStep, "shared.files[%s] missing at %s — populate shared/ before first deploy", f, src)
		}
		_ = sshC.MkdirAll(ctx, path.Dir(dst))
		if _, err := sshC.Run(ctx, fmt.Sprintf("rm -f %s && ln -s %s %s", shellQuote(dst), shellQuote(src), shellQuote(dst))); err != nil {
			_ = sshC.RemoveAll(ctx, releaseDir)
			return nil, coded(ExitTransport, "link shared %s: %w", f, err)
		}
		shLog.Info("linked file", logAttr("name", f))
	}
	for _, d := range cfg.Shared.Dirs {
		src := layout.SharedFile(d)
		dst := path.Join(releaseDir, d)
		_, _ = sshC.Run(ctx, fmt.Sprintf("mkdir -p %s", shellQuote(src)))
		_ = sshC.MkdirAll(ctx, path.Dir(dst))
		if _, err := sshC.Run(ctx, fmt.Sprintf("rm -rf %s && ln -s %s %s", shellQuote(dst), shellQuote(src), shellQuote(dst))); err != nil {
			_ = sshC.RemoveAll(ctx, releaseDir)
			return nil, coded(ExitTransport, "link shared dir %s: %w", d, err)
		}
		shLog.Info("linked dir", logAttr("name", d))
	}

	// ─── Phase: post_extract hooks ───
	for _, h := range cfg.Hooks.PostExtract {
		if err := runRemoteHook(ctx, log.WithPhase("post-extract"), sshC, releaseDir, h); err != nil {
			_ = sshC.RemoveAll(ctx, releaseDir)
			return nil, coded(ExitDeployStep, "post_extract: %w", err)
		}
	}

	// ─── Phase: atomic flip ───
	flipLog := log.WithPhase("flip")
	flipCmd := fmt.Sprintf(
		"ln -s %s %s && mv -Tf %s %s",
		shellQuote(releaseDir),
		shellQuote(layout.CurrentNewSymlink()),
		shellQuote(layout.CurrentNewSymlink()),
		shellQuote(layout.CurrentSymlink()),
	)
	if _, err := sshC.Run(ctx, flipCmd); err != nil {
		_ = sshC.RemoveAll(ctx, releaseDir)
		return nil, coded(ExitDeployStep, "atomic flip: %w", err)
	}
	flipLog.Info("flipped",
		logAttr("from", string(previousID)),
		logAttr("to", string(releaseID)),
	)

	// ─── Phase: post_flip hooks ───
	for _, h := range cfg.Hooks.PostFlip {
		if err := runRemoteHook(ctx, log.WithPhase("post-flip"), sshC, layout.CurrentSymlink(), h); err != nil {
			log.WithPhase("post-flip").Error("hook failed", logAttr("cmd", h), logAttr("err", err.Error()))
			rollback(ctx, log, sshC, layout, previousID, cfg.Hooks.OnRollback)
			result.Rolled = true
			return result, coded(ExitDeployStep, "post_flip hook failed: %w", err)
		}
	}

	// ─── Phase: health check ───
	if !opts.SkipHealth && cfg.HealthCheck.URL != "" {
		healthLog := log.WithPhase("health")
		healthLog.Info("probing", logAttr("url", cfg.HealthCheck.URL))
		probe := healthcheck.Probe{
			URL:     cfg.HealthCheck.URL,
			Status:  cfg.HealthCheck.Status,
			Expect:  cfg.HealthCheck.Expect,
			Retries: cfg.HealthCheck.Retries,
			Delay:   cfg.HealthCheck.Delay.Duration(),
			Timeout: cfg.HealthCheck.Timeout.Duration(),
		}
		hcResult, err := healthcheck.Run(ctx, probe, func(attempt int, perr error, status int) {
			healthLog.Info("attempt", slog.Int("n", attempt), slog.Int("status", status), slog.String("err", errString(perr)))
		})
		result.HealthAttempts = hcResult.Attempts
		if err != nil {
			healthLog.Error("failed", logAttr("err", err.Error()))
			rollback(ctx, log, sshC, layout, previousID, cfg.Hooks.OnRollback)
			result.Rolled = true
			return result, coded(ExitHealthFailure, "health check failed: %w", err)
		}
		healthLog.Info("passed", logAttr("attempts", hcResult.Attempts), logAttr("elapsed", hcResult.Elapsed))
	}

	// ─── Phase: prune ───
	pruneLog := log.WithPhase("prune")
	all, err := listReleases(ctx, sshC, layout)
	if err != nil {
		pruneLog.Warn("listing failed; skipping prune", logAttr("err", err.Error()))
	} else {
		release.SortIDsDescending(all)
		toDelete := release.PrunePlan(all, cfg.Releases.Keep, releaseID)
		for _, id := range toDelete {
			if err := sshC.RemoveAll(ctx, layout.ReleaseDir(id)); err != nil {
				pruneLog.Warn("could not delete", logAttr("release", string(id)), logAttr("err", err.Error()))
			} else {
				pruneLog.Info("deleted", logAttr("release", string(id)))
			}
		}
	}

	log.WithPhase("done").Info("deploy complete", logAttr("release", string(releaseID)))
	return result, nil
}

// rollback flips the symlink back to previousID and runs on_rollback hooks.
// Errors are logged but not surfaced — once we're rolling back, the deploy
// is already failed and there's nothing useful for the caller to do with
// a "rollback also failed" error other than page someone.
func rollback(ctx context.Context, log *logger.Logger, sshC *ssh.Client, layout release.Layout, previousID release.ID, onRollback []string) {
	rbLog := log.WithPhase("rollback")
	if previousID == "" {
		rbLog.Error("no previous release to roll back to — this was the first deploy")
		return
	}
	previousDir := layout.ReleaseDir(previousID)
	cmd := fmt.Sprintf(
		"ln -s %s %s && mv -Tf %s %s",
		shellQuote(previousDir),
		shellQuote(layout.CurrentNewSymlink()),
		shellQuote(layout.CurrentNewSymlink()),
		shellQuote(layout.CurrentSymlink()),
	)
	if _, err := sshC.Run(ctx, cmd); err != nil {
		rbLog.Error("failed to flip symlink back", logAttr("err", err.Error()))
		return
	}
	rbLog.Info("rolled back", logAttr("to", string(previousID)))
	for _, h := range onRollback {
		if err := runRemoteHook(ctx, rbLog, sshC, layout.CurrentSymlink(), h); err != nil {
			rbLog.Error("on_rollback hook failed", logAttr("cmd", h), logAttr("err", err.Error()))
		}
	}
}

// extractRemote unpacks the artifact into releaseDir on the remote.
func extractRemote(ctx context.Context, sshC *ssh.Client, format, artifact, releaseDir string) error {
	var cmd string
	switch format {
	case "zip":
		cmd = fmt.Sprintf("unzip -q -o %s -d %s", shellQuote(artifact), shellQuote(releaseDir))
	case "tar.gz":
		cmd = fmt.Sprintf("tar -xzf %s -C %s", shellQuote(artifact), shellQuote(releaseDir))
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
	res, err := sshC.Run(ctx, cmd)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("extract exited %d: %s", res.ExitCode, string(res.Stderr))
	}
	return nil
}

// currentReleaseID resolves the current symlink and returns the release ID
// it points at (i.e. the basename of releases/<ID>). Returns "" if no
// current symlink exists yet (first deploy).
func currentReleaseID(ctx context.Context, sshC *ssh.Client, layout release.Layout) (release.ID, error) {
	ok, err := sshC.Exists(ctx, layout.CurrentSymlink())
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	res, err := sshC.Run(ctx, fmt.Sprintf("readlink -f %s", shellQuote(layout.CurrentSymlink())))
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		return "", nil
	}
	out := trimTrailingNewline(string(res.Stdout))
	name := path.Base(out)
	if !release.IsValidID(name) {
		return "", nil
	}
	return release.ID(name), nil
}

// listReleases returns every release directory's name.
func listReleases(ctx context.Context, sshC *ssh.Client, layout release.Layout) ([]release.ID, error) {
	res, err := sshC.Run(ctx, fmt.Sprintf("ls -1 %s 2>/dev/null", shellQuote(layout.ReleasesDir())))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range splitLines(string(res.Stdout)) {
		if line != "" {
			names = append(names, line)
		}
	}
	return release.FilterValidIDs(names), nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// resolveArtifactPath converts the relative-to-config path into an
// absolute filesystem path the local OS will accept.
func resolveArtifactPath(cfg *config.Config) (string, error) {
	src := cfg.Artifact.Source
	if filepath.IsAbs(src) {
		return src, nil
	}
	base := cfg.SourcePath()
	if base == "" {
		abs, err := filepath.Abs(src)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	return filepath.Join(filepath.Dir(base), src), nil
}

func lockHolder(l *lockfile.Lock) string {
	if l == nil {
		return "unknown"
	}
	return fmt.Sprintf("%s@%s pid=%d at=%s", l.HeldBy, l.Hostname, l.PID, l.AcquiredAt.Format(time.RFC3339))
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
