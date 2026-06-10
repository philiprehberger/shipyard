import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Config reference',
  description: 'Every field in shipyard.yaml: required, defaults, validation, semantics.',
};

export default function ConfigReference() {
  return (
    <>
      <h1>Config reference</h1>
      <p style={{ opacity: 0.85 }}>
        Every field in <code>shipyard.yaml</code>. Required fields are flagged; everything else has a sensible default.
      </p>

      <div className="callout">
        <p>
          <strong>Strict mode.</strong> Unknown fields are rejected at load time. Misspell <code>health_check</code> as <code>health-check</code> and the deploy refuses to start with the exact YAML line that caused it.
        </p>
      </div>

      <h2>app <span style={{ opacity: 0.6, fontSize: '0.9em' }}>(required)</span></h2>
      <pre><code>{`app: webhook-relay`}</code></pre>
      <p>
        Short slug used in log lines and as the default lock-path component. Lowercase alphanumerics with <code>-</code> or <code>_</code> only. Up to 63 chars.
      </p>

      <h2>host <span style={{ opacity: 0.6, fontSize: '0.9em' }}>(required)</span></h2>
      <pre><code>{`host:
  ssh: ubuntu@1.2.3.4
  identity_file: ~/.ssh/id_ed25519
  release_root: /var/www/webhook-relay`}</code></pre>

      <table>
        <thead>
          <tr><th>Field</th><th>Required</th><th>Notes</th></tr>
        </thead>
        <tbody>
          <tr><td><code>ssh</code></td><td>yes</td><td><code>user@host[:port]</code>. Port defaults to 22. IPv6 supported via <code>user@[::1]:port</code>.</td></tr>
          <tr><td><code>identity_file</code></td><td>yes</td><td>Path to private key. <code>~/</code> is expanded. Encrypted keys are not supported in v0.1 — decrypt with <code>ssh-keygen -p</code> first.</td></tr>
          <tr><td><code>release_root</code></td><td>yes</td><td>Absolute path on remote. Must be writable by the SSH user.</td></tr>
        </tbody>
      </table>

      <h2>artifact <span style={{ opacity: 0.6, fontSize: '0.9em' }}>(required)</span></h2>
      <pre><code>{`artifact:
  source: ./build/release.tar.gz
  format: tar.gz`}</code></pre>

      <table>
        <thead>
          <tr><th>Field</th><th>Required</th><th>Notes</th></tr>
        </thead>
        <tbody>
          <tr><td><code>source</code></td><td>yes</td><td>Local path to the build artifact. Relative paths resolve against the directory of the config file, not the shell&rsquo;s cwd.</td></tr>
          <tr><td><code>format</code></td><td>auto</td><td><code>zip</code> or <code>tar.gz</code>. Inferred from the source extension if omitted.</td></tr>
        </tbody>
      </table>

      <h2>releases</h2>
      <pre><code>{`releases:
  keep: 5`}</code></pre>
      <p>
        Auto-prune retains the most recent <code>keep</code> releases plus the current symlink target (if different). Default 5. Cannot be less than 1.
      </p>

      <h2>shared</h2>
      <pre><code>{`shared:
  files:
    - .env
  dirs:
    - storage
    - bootstrap/cache`}</code></pre>
      <p>
        At deploy time, every entry under <code>shared.files</code> and <code>shared.dirs</code> is symlinked from the release dir back to <code>shared/</code>. Use this for anything that must persist between releases (<code>.env</code>, Laravel <code>storage/</code>, uploaded media).
      </p>
      <p>
        Paths are relative to the release directory. Absolute paths are rejected. If a <code>shared.files</code> target doesn&rsquo;t exist on the remote when the deploy runs, the deploy fails fast with an actionable message (no silently-created empty <code>.env</code>).
      </p>

      <h2>health_check</h2>
      <pre><code>{`health_check:
  url: https://my-app.example.com/healthz
  expect: "healthy"
  status: 200
  retries: 10
  delay: 3s
  timeout: 5s`}</code></pre>

      <table>
        <thead>
          <tr><th>Field</th><th>Default</th><th>Notes</th></tr>
        </thead>
        <tbody>
          <tr><td><code>url</code></td><td>required if block present</td><td>Must start with <code>http://</code> or <code>https://</code>.</td></tr>
          <tr><td><code>expect</code></td><td>empty</td><td>Substring that must appear in the body. Empty = status check only.</td></tr>
          <tr><td><code>status</code></td><td>200</td><td>HTTP status that counts as healthy.</td></tr>
          <tr><td><code>retries</code></td><td>10</td><td>Number of attempts. Total wall time ≈ retries × (delay + timeout).</td></tr>
          <tr><td><code>delay</code></td><td>3s</td><td>Wait between attempts. <code>3s</code>, <code>500ms</code>, <code>1m</code> all parse.</td></tr>
          <tr><td><code>timeout</code></td><td>5s</td><td>Per-request timeout.</td></tr>
        </tbody>
      </table>
      <p>
        If the whole <code>health_check</code> block is omitted, the deploy flips and exits without probing. <code>--skip-health</code> on the deploy command does the same per-invocation — use it sparingly.
      </p>
      <p>
        Redirects are <strong>not followed</strong>. Point the URL at the final destination directly. The deploy log surfaces every attempt&rsquo;s status code so a flapping probe is immediately diagnosable.
      </p>

      <h2>hooks</h2>
      <pre><code>{`hooks:
  pre_upload:
    - composer install --no-dev --optimize-autoloader
    - npm run build
  post_extract:
    - php artisan migrate --force
    - php artisan config:cache
  post_flip:
    - sudo systemctl reload apache2
  on_rollback:
    - sudo systemctl reload apache2`}</code></pre>

      <table>
        <thead>
          <tr><th>Phase</th><th>Where</th><th>Cwd</th><th>What it&rsquo;s for</th></tr>
        </thead>
        <tbody>
          <tr><td><code>pre_upload</code></td><td>local</td><td>your shell&rsquo;s cwd</td><td>Build the artifact.</td></tr>
          <tr><td><code>post_extract</code></td><td>remote</td><td>the new release dir</td><td>Migrations, cache warming. Runs before the flip — if it fails, the release dir is deleted and the current symlink never moves.</td></tr>
          <tr><td><code>post_flip</code></td><td>remote</td><td><code>current/</code></td><td>Reload the web server / restart workers. Runs after the flip — if it fails, Shipyard rolls back automatically.</td></tr>
          <tr><td><code>on_rollback</code></td><td>remote</td><td><code>current/</code></td><td>Cleanup or notification after a rollback (health-fail or post-flip-fail).</td></tr>
        </tbody>
      </table>

      <p>
        Each entry is passed to <code>sh -c</code> on the relevant side. Quotes are linted for balance at load time — an unterminated string is rejected before the deploy starts.
      </p>

      <h2>lock</h2>
      <pre><code>{`lock:
  enabled: true
  path: /var/www/webhook-relay/shared/.shipyard.lock
  ttl: 10m`}</code></pre>

      <table>
        <thead>
          <tr><th>Field</th><th>Default</th><th>Notes</th></tr>
        </thead>
        <tbody>
          <tr><td><code>enabled</code></td><td>true</td><td>Set <code>false</code> only if you&rsquo;re certain about concurrency.</td></tr>
          <tr><td><code>path</code></td><td><code>&lt;release_root&gt;/shared/.shipyard.lock</code></td><td>Absolute path. Must be writable by the SSH user.</td></tr>
          <tr><td><code>ttl</code></td><td>10m</td><td>Stale-lock threshold. Minimum 30s. A lock older than ttl is stolen automatically (logged loudly) so a crashed deploy can&rsquo;t block forever.</td></tr>
        </tbody>
      </table>
      <p>
        Implementation: SFTP <code>O_CREATE|O_EXCL</code> on a small JSON file containing local user, hostname, PID, and acquired-at timestamp. <code>shipyard status</code> displays the holder.
      </p>

      <hr />
      <p>
        <Link href="/docs/quickstart">← Quickstart</Link>
        {' · '}
        <Link href="/docs/cli">→ CLI reference</Link>
      </p>
    </>
  );
}
