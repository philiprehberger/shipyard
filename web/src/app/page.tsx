import Link from 'next/link';

export default function Home() {
  return (
    <>
      <h1>
        Deploys without holding<br />your breath.
      </h1>
      <p className="mt-6 text-lg" style={{ opacity: 0.85, color: 'var(--color-steel-light)' }}>
        Shipyard is an atomic-release deploy CLI: zero-downtime SSH/rsync deploys with health-gated promotion and automatic rollback. One static Go binary, one YAML config, no agent on the server.
      </p>

      <div className="mt-8 flex flex-wrap items-center gap-3">
        <Link
          href="/docs/quickstart"
          className="!text-[color:var(--color-ink)] !no-underline bg-[color:var(--color-rust-bright)] hover:bg-[color:var(--color-paper)] rounded-md px-4 py-2 text-sm font-semibold"
        >
          Quickstart →
        </Link>
        <a
          href="https://github.com/philiprehberger/shipyard"
          className="!text-[color:var(--color-paper)] !no-underline border border-[color:var(--color-line)] hover:border-[color:var(--color-rust-bright)] rounded-md px-4 py-2 text-sm font-semibold"
        >
          View on GitHub
        </a>
      </div>

      <hr />

      <h2>The wrong shape</h2>
      <p>
        Most "deploy scripts" you'll inherit look like this:
      </p>
      <pre><code>{`ssh prod 'cd app && git pull && pm2 restart app'`}</code></pre>
      <p>
        Between <code>git pull</code> and the restart, Apache is serving half-old, half-new files. Composer autoload sees a class the app doesn't yet have on disk. If the new code breaks at boot, <code>pm2 restart</code> still exits 0 — it returned the moment the process was alive, not when it was serving 200s. There's no rollback.
      </p>

      <h2>The right shape</h2>
      <p>
        Immutable release directories, atomic symlink flip, health-gated promotion, automatic rollback. This is what Capistrano has done since 2010 and what most "deploy scripts" in the wild still don't do.
      </p>

      <pre><code>{`# Pipeline shipyard runs for you:

  pre_upload hooks (local)     composer install --no-dev
                               npm run build
  SSH connect + lockfile
  SFTP upload artifact         _incoming/20260610190921.tar.gz
  extract                      releases/20260610190921/
  symlink shared/              .env, storage/, bootstrap/cache/
  post_extract hooks (remote)  php artisan migrate --force
  atomic flip                  ln -s + mv -Tf
  post_flip hooks (remote)     sudo systemctl reload apache2
  health probe                 GET /healthz → expect "healthy"
       FAIL ──→ rollback symlink, run on_rollback hooks, exit 4
       PASS ──→ continue
  auto-prune                   releases.keep = 5
  release lock`}</code></pre>

      <h2>What you write</h2>
      <p>
        One <code>shipyard.yaml</code>:
      </p>
      <pre><code>{`app: webhook-relay

host:
  ssh: ubuntu@1.2.3.4
  identity_file: ~/.ssh/id_ed25519
  release_root: /var/www/webhook-relay

artifact:
  source: ./deploy-staging/release.tar.gz
  format: tar.gz

shared:
  files: [.env]
  dirs:  [storage, bootstrap/cache]

health_check:
  url: https://api.webhook-relay.example.com/healthz
  expect: "healthy"
  retries: 10
  delay: 3s

hooks:
  pre_upload:   ["composer install --no-dev", "npm run build"]
  post_extract: ["php artisan migrate --force"]
  post_flip:    ["sudo systemctl reload apache2"]
  on_rollback:  ["sudo systemctl reload apache2"]`}</code></pre>

      <p>
        Then:
      </p>

      <pre><code>{`$ shipyard deploy

[connect]      ▸ dialing  target=ubuntu@1.2.3.4:22
[connect]      ▸ connected
[lock]         ▸ acquired
[upload]       ▸ uploading  to=_incoming/20260610190921.tar.gz
[extract]      ▸ extracting
[post-extract] ▸ php artisan migrate --force
[flip]         ▸ flipped  from=20260610185512 to=20260610190921
[post-flip]    ▸ sudo systemctl reload apache2
[health]       ▸ attempt  n=1 status=200
[health]       ▸ passed   attempts=1 elapsed=147ms
[prune]        ▸ deleted  release=20260610150318
[done]         ▸ deploy complete  release=20260610190921`}</code></pre>

      <h2>Why not Capistrano / Deployer / Kamal</h2>
      <table>
        <thead>
          <tr><th>Tool</th><th>Strength</th><th>Where it doesn't fit</th></tr>
        </thead>
        <tbody>
          <tr><td>Capistrano</td><td>Battle-tested Ruby ecosystem</td><td>Ruby-shaped — extension surface assumes Rails idioms</td></tr>
          <tr><td>Deployer</td><td>PHP-native; Composer-aware</td><td>PHP-shaped — awkward for Node/Go/Python stacks</td></tr>
          <tr><td>Kamal</td><td>Docker-first, Rails-team-shaped</td><td>If you don't run containers, mostly overhead</td></tr>
          <tr><td>GitHub Actions deploy</td><td>CI-shaped; no extra tool</td><td>Build artifacts have to be produced wherever Composer lives — usually not on the server</td></tr>
        </tbody>
      </table>

      <p>
        Shipyard's slot: language-agnostic, transport-agnostic, opinionated about safety, unopinionated about your stack. A single static binary you scp anywhere.
      </p>

      <hr />

      <div className="callout">
        <p>
          <strong>This page is deployed by Shipyard.</strong> The docs site you&rsquo;re reading lives at <code>/var/www/shipyard-web/current</code>, which is a symlink Shipyard flipped after a health-gated promotion. Every push to <code>main</code> reaches you the same way. webhook-relay&rsquo;s deploy migrates onto Shipyard 30 days after v0.1.0 once the failure modes shake out.
        </p>
      </div>

      <div className="mt-12 flex gap-4">
        <Link href="/docs/quickstart" className="!no-underline">
          → Quickstart
        </Link>
        <Link href="/docs/config-reference" className="!no-underline">
          → Config reference
        </Link>
        <Link href="/docs/cli" className="!no-underline">
          → CLI reference
        </Link>
      </div>
    </>
  );
}
