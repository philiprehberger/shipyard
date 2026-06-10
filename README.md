# Shipyard

Atomic-release deploy CLI. Zero-downtime SSH/rsync deploys with health-gated promotion and automatic rollback. One static Go binary, one YAML config, no agent on the server.

```bash
shipyard deploy shipyard.yaml
```

That command:

1. Runs your local `pre_upload` hooks (build, package).
2. SSHes into the target host, acquires a lockfile.
3. Uploads the artifact into `releases/<timestamp>/`.
4. Symlinks shared files (`.env`, `storage/`) into the release.
5. Runs `post_extract` hooks on the remote (migrations, cache:clear).
6. **Atomically** flips the `current` symlink — `ln -s` + `mv -Tf`, never `ln -sfn`.
7. Runs `post_flip` hooks (reload Apache, kick supervisor).
8. Probes the health-check URL. If it fails after configured retries, **rolls back the symlink and runs `on_rollback`**.
9. Auto-prunes releases older than `releases.keep`.
10. Releases the lockfile.

Exit codes are documented and stable. The docs site enumerates every failure mode.

## Why

Most "deploy scripts" are `ssh prod 'cd app && git pull && pm2 restart'`. That serves half-old / half-new files mid-pull, has no rollback, and reports success the instant the process is alive — not when it's serving 200s. Shipyard does what Capistrano did in 2010, as a single static binary that doesn't require Ruby (or Docker, or Python) on either side.

## Install

```bash
# Go
go install github.com/philiprehberger/shipyard/cli/cmd/shipyard@latest

# Linux/macOS tarball (no Go required)
curl -sSL https://github.com/philiprehberger/shipyard/releases/latest/download/shipyard_linux_amd64.tar.gz | tar -xz
sudo mv shipyard /usr/local/bin/

# Verify
shipyard version
```

## Quickstart

```bash
shipyard init                          # interactive — writes shipyard.yaml
shipyard doctor                        # validates config, SSH access, remote writability
shipyard deploy                        # ships
```

## Config

See `examples/` for canonical shapes (Laravel API, Next.js standalone, PM2 reload). Full schema reference at https://shipyard.philiprehberger.com/docs/config-reference.

```yaml
app: my-app

host:
  ssh: ubuntu@1.2.3.4
  identity_file: ~/.ssh/id_ed25519
  release_root: /var/www/my-app

artifact:
  source: ./build/release.tar.gz
  format: tar.gz

releases:
  keep: 5

shared:
  files: [.env]
  dirs:  [storage, bootstrap/cache]

health_check:
  url: https://my-app.example.com/healthz
  expect: "healthy"
  retries: 10
  delay: 3s
  timeout: 5s

hooks:
  pre_upload:   ["composer install --no-dev", "npm run build"]
  post_extract: ["php artisan migrate --force"]
  post_flip:    ["sudo systemctl reload apache2"]
  on_rollback:  ["sudo systemctl reload apache2"]
```

## Commands

| Command | What |
|---|---|
| `shipyard deploy [config]` | Run a deploy. |
| `shipyard rollback [config]` | Swap symlink to previous release. |
| `shipyard status [config]` | Show current + last N releases + lock state. |
| `shipyard releases [config]` | List all releases on the remote. |
| `shipyard prune [config]` | Delete old releases. |
| `shipyard init` | Interactive config generator. |
| `shipyard doctor [config]` | Validate config + SSH + remote writability. |
| `shipyard version` | Print version. |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Usage or config error |
| 2 | SSH / transport error |
| 3 | Deploy step error (rolled back) |
| 4 | Health-check failure (rolled back) |
| 5 | Lock held by another process |

## License

[MIT](LICENSE).

## Links

- [Docs](https://shipyard.philiprehberger.com)
- [Releases](https://github.com/philiprehberger/shipyard/releases)
- [Issues](https://github.com/philiprehberger/shipyard/issues)
