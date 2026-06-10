package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/philiprehberger/shipyard/cli/internal/config"
	"github.com/philiprehberger/shipyard/cli/internal/lockfile"
	"github.com/philiprehberger/shipyard/cli/internal/logger"
	"github.com/philiprehberger/shipyard/cli/internal/release"
	"github.com/philiprehberger/shipyard/cli/internal/ssh"
)

// Status is the output of GetStatus.
type Status struct {
	App         string
	Host        string
	ReleaseRoot string
	CurrentID   release.ID
	Releases    []ReleaseInfo
	LockHeld    bool
	LockInfo    *lockfile.Lock
}

// ReleaseInfo is a single row in Status.Releases.
type ReleaseInfo struct {
	ID         release.ID
	IsCurrent  bool
	ModifiedAt time.Time
}

// GetStatus connects to the remote and reads layout + lock state.
func GetStatus(ctx context.Context, cfg *config.Config, log *logger.Logger) (*Status, error) {
	if log == nil {
		log = logger.NewLogger(logger.Options{})
	}
	sshC, closeFn, err := openSSH(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer closeFn()

	layout := release.NewLayout(cfg.Host.ReleaseRoot)

	currentID, err := currentReleaseID(ctx, sshC, layout)
	if err != nil {
		return nil, fmt.Errorf("reading current symlink: %w", err)
	}

	ids, err := listReleases(ctx, sshC, layout)
	if err != nil {
		return nil, fmt.Errorf("listing releases: %w", err)
	}
	release.SortIDsDescending(ids)

	var infos []ReleaseInfo
	for _, id := range ids {
		info, err := sshC.SFTP().Stat(layout.ReleaseDir(id))
		if err != nil {
			continue
		}
		infos = append(infos, ReleaseInfo{
			ID:         id,
			IsCurrent:  id == currentID,
			ModifiedAt: info.ModTime(),
		})
	}

	s := &Status{
		App:         cfg.App,
		Host:        cfg.Host.SSH,
		ReleaseRoot: cfg.Host.ReleaseRoot,
		CurrentID:   currentID,
		Releases:    infos,
	}

	if cfg.Lock.IsEnabled() {
		ok, err := sshC.Exists(ctx, cfg.Lock.Path)
		if err == nil && ok {
			s.LockHeld = true
			s.LockInfo = readLockBestEffort(sshC, cfg.Lock.Path)
		}
	}
	return s, nil
}

// ListReleases returns every release on the remote, newest first, plus
// the current ID.
func ListReleases(ctx context.Context, cfg *config.Config) ([]release.ID, release.ID, error) {
	sshC, closeFn, err := openSSH(ctx, cfg)
	if err != nil {
		return nil, "", err
	}
	defer closeFn()

	layout := release.NewLayout(cfg.Host.ReleaseRoot)
	ids, err := listReleases(ctx, sshC, layout)
	if err != nil {
		return nil, "", err
	}
	release.SortIDsDescending(ids)

	currentID, err := currentReleaseID(ctx, sshC, layout)
	if err != nil {
		return nil, "", err
	}
	return ids, currentID, nil
}

// PruneResult is what Prune returns.
type PruneResult struct {
	Deleted []release.ID
	Kept    []release.ID
}

// PruneOptions configures Prune.
type PruneOptions struct {
	Config *config.Config
	Log    *logger.Logger
	Keep   int // 0 = honor config
	DryRun bool
}

// Prune deletes old releases.
func Prune(ctx context.Context, opts PruneOptions) (*PruneResult, error) {
	if opts.Log == nil {
		opts.Log = logger.NewLogger(logger.Options{})
	}
	sshC, closeFn, err := openSSH(ctx, opts.Config)
	if err != nil {
		return nil, err
	}
	defer closeFn()

	layout := release.NewLayout(opts.Config.Host.ReleaseRoot)
	ids, err := listReleases(ctx, sshC, layout)
	if err != nil {
		return nil, err
	}
	release.SortIDsDescending(ids)

	currentID, err := currentReleaseID(ctx, sshC, layout)
	if err != nil {
		return nil, err
	}

	keep := opts.Keep
	if keep == 0 {
		keep = opts.Config.Releases.Keep
	}
	if keep < 1 {
		keep = 1
	}

	toDelete := release.PrunePlan(ids, keep, currentID)
	deleteSet := make(map[release.ID]bool, len(toDelete))
	for _, d := range toDelete {
		deleteSet[d] = true
	}

	result := &PruneResult{}
	for _, id := range ids {
		if deleteSet[id] {
			result.Deleted = append(result.Deleted, id)
		} else {
			result.Kept = append(result.Kept, id)
		}
	}

	if opts.DryRun {
		opts.Log.WithPhase("prune").Info("dry-run", logAttr("would_delete", len(result.Deleted)))
		return result, nil
	}

	for _, id := range result.Deleted {
		if err := sshC.RemoveAll(ctx, layout.ReleaseDir(id)); err != nil {
			opts.Log.WithPhase("prune").Warn("delete failed", logAttr("release", string(id)), logAttr("err", err.Error()))
		} else {
			opts.Log.WithPhase("prune").Info("deleted", logAttr("release", string(id)))
		}
	}
	return result, nil
}

// RollbackOptions configures Rollback.
type RollbackOptions struct {
	Config *config.Config
	Log    *logger.Logger
	ToID   release.ID // empty = previous release
}

// RollbackResult is what Rollback returns on the happy path.
type RollbackResult struct {
	From release.ID
	To   release.ID
}

// Rollback flips the current symlink to the previous (or specified) release.
func Rollback(ctx context.Context, opts RollbackOptions) (*RollbackResult, error) {
	if opts.Log == nil {
		opts.Log = logger.NewLogger(logger.Options{})
	}
	sshC, closeFn, err := openSSH(ctx, opts.Config)
	if err != nil {
		return nil, err
	}
	defer closeFn()

	layout := release.NewLayout(opts.Config.Host.ReleaseRoot)

	currentID, err := currentReleaseID(ctx, sshC, layout)
	if err != nil {
		return nil, err
	}
	if currentID == "" {
		return nil, errors.New("no current release — nothing to roll back from")
	}

	target := opts.ToID
	if target == "" {
		ids, err := listReleases(ctx, sshC, layout)
		if err != nil {
			return nil, err
		}
		release.SortIDsDescending(ids)
		for _, id := range ids {
			if id != currentID {
				target = id
				break
			}
		}
		if target == "" {
			return nil, errors.New("no other release found to roll back to")
		}
	} else {
		ok, err := sshC.Exists(ctx, layout.ReleaseDir(target))
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("release %s does not exist on remote", target)
		}
	}

	rbLog := opts.Log.WithPhase("rollback")
	cmd := fmt.Sprintf(
		"ln -s %s %s && mv -Tf %s %s",
		shellQuote(layout.ReleaseDir(target)),
		shellQuote(layout.CurrentNewSymlink()),
		shellQuote(layout.CurrentNewSymlink()),
		shellQuote(layout.CurrentSymlink()),
	)
	if _, err := sshC.Run(ctx, cmd); err != nil {
		return nil, fmt.Errorf("flipping symlink back: %w", err)
	}
	rbLog.Info("rolled back", logAttr("from", string(currentID)), logAttr("to", string(target)))

	for _, h := range opts.Config.Hooks.OnRollback {
		if err := runRemoteHook(ctx, rbLog, sshC, layout.CurrentSymlink(), h); err != nil {
			rbLog.Error("on_rollback hook failed", logAttr("cmd", h), logAttr("err", err.Error()))
		}
	}

	return &RollbackResult{From: currentID, To: target}, nil
}

// openSSH is the shared connection-open helper for the inspect commands.
func openSSH(ctx context.Context, cfg *config.Config) (*ssh.Client, func(), error) {
	target, err := ssh.ParseTarget(cfg.Host.SSH)
	if err != nil {
		return nil, nil, fmt.Errorf("host.ssh: %w", err)
	}
	sshC, err := ssh.Connect(ctx, target, ssh.ConnectOpts{
		IdentityFile: cfg.Host.IdentityFile,
		Timeout:      15 * time.Second,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("ssh: %w", err)
	}
	return sshC, func() { _ = sshC.Close() }, nil
}

// readLockBestEffort reads + decodes the JSON lock file for display in
// status output. Returns nil silently if anything goes wrong — status is
// diagnostic, not load-bearing.
func readLockBestEffort(sshC *ssh.Client, lockPath string) *lockfile.Lock {
	f, err := sshC.SFTP().Open(lockPath)
	if err != nil {
		return nil
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 4096))
	if err != nil {
		return nil
	}
	var l lockfile.Lock
	if err := json.Unmarshal(data, &l); err != nil {
		return nil
	}
	return &l
}
