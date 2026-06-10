import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Quickstart',
  description: 'Install Shipyard, write a shipyard.yaml, and run your first deploy in under five minutes.',
};

export default function Quickstart() {
  return (
    <>
      <h1>Quickstart</h1>
      <p style={{ opacity: 0.85 }}>
        Install Shipyard, write a <code>shipyard.yaml</code>, and run your first deploy in under five minutes.
      </p>

      <h2>1. Install</h2>
      <p>The binary is a single static Go executable. No runtime to install on the server.</p>

      <h3>Via Go</h3>
      <pre><code>{`go install github.com/philiprehberger/shipyard/cli/cmd/shipyard@latest`}</code></pre>

      <h3>Via tarball (Linux / macOS)</h3>
      <pre><code>{`curl -sSL https://github.com/philiprehberger/shipyard/releases/latest/download/shipyard_linux_amd64.tar.gz \\
  | tar -xz
sudo mv shipyard /usr/local/bin/
shipyard version`}</code></pre>

      <p>
        For Apple Silicon use <code>shipyard_darwin_arm64.tar.gz</code>; for Windows download the <code>.zip</code> from the <a href="https://github.com/philiprehberger/shipyard/releases/latest">releases page</a>.
      </p>

      <h2>2. Prepare the server</h2>
      <p>
        Shipyard expects a release-root directory it owns. SSH in once and create it:
      </p>
      <pre><code>{`ssh ubuntu@my-server.example.com
sudo mkdir -p /var/www/my-app/{releases,shared,_incoming}
sudo chown -R ubuntu:ubuntu /var/www/my-app

# Put any files that must persist across deploys (e.g. .env) into shared/:
echo "APP_KEY=base64:..." > /var/www/my-app/shared/.env`}</code></pre>

      <div className="callout">
        <p>
          <strong>Composer / npm on the server is not required.</strong> Shipyard builds the artifact locally and ships it. The server only needs <code>tar</code> or <code>unzip</code>, which every Linux distro has.
        </p>
      </div>

      <h2>3. Write shipyard.yaml</h2>
      <p>
        Drop a config at the root of your project. The minimum:
      </p>
      <pre><code>{`app: my-app

host:
  ssh: ubuntu@my-server.example.com
  identity_file: ~/.ssh/id_ed25519
  release_root: /var/www/my-app

artifact:
  source: ./build/release.tar.gz
  format: tar.gz

shared:
  files: [.env]

health_check:
  url: https://my-app.example.com/healthz
  expect: "healthy"
  retries: 10
  delay: 3s

hooks:
  pre_upload:
    - npm ci
    - npm run build
    - tar -czf build/release.tar.gz -C dist .
  post_flip:
    - sudo systemctl reload apache2
  on_rollback:
    - sudo systemctl reload apache2`}</code></pre>

      <p>
        See the <Link href="/docs/config-reference">config reference</Link> for every field. The <a href="https://github.com/philiprehberger/shipyard/tree/main/examples">examples directory</a> has full configs for Laravel, Next.js, and a Laravel + supervisor queue.
      </p>

      <h2>4. Validate</h2>
      <p>
        Before you ship anything, lint the config:
      </p>
      <pre><code>{`shipyard doctor --config-only`}</code></pre>
      <p>
        If anything's wrong, doctor surfaces every issue in one pass with the exact YAML field that broke. Strict mode catches typos like <code>health-check</code> vs. <code>health_check</code> at load time.
      </p>

      <h2>5. Deploy</h2>
      <pre><code>{`shipyard deploy`}</code></pre>
      <p>
        You'll see a pretty stream of phases (<code>[connect]</code>, <code>[upload]</code>, <code>[extract]</code>, <code>[post-extract]</code>, <code>[flip]</code>, <code>[post-flip]</code>, <code>[health]</code>, <code>[prune]</code>, <code>[done]</code>) plus full structured JSON on stderr for scripting / log shipping.
      </p>
      <p>
        If the health check fails, Shipyard flips the symlink back to the previous release, runs <code>on_rollback</code>, releases the lock, and exits with code 4. Your app stays serving requests on the old release the entire time.
      </p>

      <h2>6. Rolling back manually</h2>
      <p>
        If a deploy passed the health check but you still want to revert (a bug that takes longer than the probe window to surface):
      </p>
      <pre><code>{`shipyard rollback                    # → previous release
shipyard rollback --to 20260610190921   # → a specific timestamp`}</code></pre>

      <h2>7. Inspecting state</h2>
      <pre><code>{`shipyard status      # current release, all releases, lock state
shipyard releases    # list of timestamps newest-first
shipyard prune --dry-run   # show what auto-prune would delete`}</code></pre>

      <hr />

      <p>
        <Link href="/docs/config-reference">→ Config reference</Link>
        {' · '}
        <Link href="/docs/cli">→ CLI reference</Link>
      </p>
    </>
  );
}
