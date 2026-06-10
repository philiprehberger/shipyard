# Changelog

All notable changes to Shipyard documented here. Format follows [keep-a-changelog](https://keepachangelog.com/en/1.1.0/); versioning follows [SemVer](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.0.1] — 2026-06-10

First publishable build. Proves the release pipeline works end-to-end; the CLI does the entire atomic-release lifecycle but should still be treated as a preview while real-world configs settle.

### Added

- `shipyard deploy` — full 13-step lifecycle: pre_upload local hooks, SSH connect, remote SFTP lockfile, SFTP upload to `_incoming/`, remote extract to `releases/<ts>/`, shared file/dir symlinks, post_extract remote hooks, atomic symlink flip (`ln -s` + `mv -Tf`), post_flip remote hooks, health check with retries, auto-rollback on health-check failure or post_flip hook failure, auto-prune respecting `releases.keep`, lock release.
- `shipyard rollback [config] [--to <timestamp>]` — atomic symlink swap to the previous (or specified) release, runs `on_rollback` hooks.
- `shipyard status [config] [--format json|pretty]` — current release, all releases with mtimes, lock state.
- `shipyard releases [config]` — lex-sorted-desc release list with current marker.
- `shipyard prune [config] [--keep N] [--dry-run]` — delete old releases, never the current one.
- `shipyard doctor [config] [--config-only]` — config-only validation works; SSH + remote-writability checks land in v0.2.
- `shipyard version` — prints version/commit/date.
- Documented exit codes 0..5.
- YAML strict-mode parsing — unknown fields are rejected (catches `health-check` vs. `health_check` typos at load time).
- Custom `Duration` type — config accepts `3s` / `10m` / `1h`.
- Example configs for Laravel API (mod_php), Next.js standalone (PM2 reload), Laravel with supervisor-managed queue worker.
- Cross-platform builds via GoReleaser: linux/darwin (amd64+arm64), windows (amd64).

### Known limitations

- Encrypted private keys are not supported (decrypt with `ssh-keygen -p` first).
- `shipyard init` (interactive config generator) is not yet implemented.
- `shipyard doctor` SSH/writability checks are not yet implemented.
- Telemetry: none, ever — no phone-home.
