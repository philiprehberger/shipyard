import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'CLI reference',
  description: 'Every subcommand and flag in shipyard. Exit codes, dry-run, force-lock-steal, all of it.',
};

export default function CLIReference() {
  return (
    <>
      <h1>CLI reference</h1>
      <p style={{ opacity: 0.85 }}>
        Every subcommand and flag. Exit codes are stable and documented at the bottom.
      </p>

      <h2>shipyard deploy</h2>
      <pre><code>{`shipyard deploy [config]
  --dry-run                show what would happen, stop after SSH connect
  --skip-health            flip without running the health check (don't)
  --release-id <ts>        override the auto-generated YYYYMMDDhhmmss timestamp
  --force                  ignore an active remote lock and steal it`}</code></pre>
      <p>
        Runs the full 13-step lifecycle described on the <Link href="/">home page</Link>. The pipeline:
      </p>
      <ol>
        <li>Parse config (strict mode — unknown fields rejected).</li>
        <li>Run <code>pre_upload</code> hooks locally.</li>
        <li>SSH connect, host-key verify against <code>~/.ssh/known_hosts</code>.</li>
        <li>Acquire remote lockfile (steals stale ones past TTL).</li>
        <li>SFTP-upload the artifact into <code>_incoming/&lt;timestamp&gt;.&lt;ext&gt;</code>.</li>
        <li>Extract into <code>releases/&lt;timestamp&gt;/</code>.</li>
        <li>Symlink <code>shared.files</code> and <code>shared.dirs</code> into the release dir.</li>
        <li>Run <code>post_extract</code> hooks remotely (cwd: the new release dir).</li>
        <li>Atomic symlink flip: <code>ln -s …/releases/&lt;ts&gt; current.new && mv -Tf current.new current</code>.</li>
        <li>Run <code>post_flip</code> hooks remotely. Failure here triggers rollback.</li>
        <li>Run health check. Failure triggers rollback.</li>
        <li>Auto-prune (honors <code>releases.keep</code>, never deletes the current release).</li>
        <li>Release lockfile.</li>
      </ol>

      <h2>shipyard rollback</h2>
      <pre><code>{`shipyard rollback [config]
  --to <timestamp>         roll back to a specific release (default: previous)`}</code></pre>
      <p>
        Atomically swap the <code>current</code> symlink to the previous (or specified) release, then run <code>on_rollback</code> hooks. Deploys after a rollback continue from the rolled-back state — <code>shipyard deploy</code> picks the next timestamp as expected.
      </p>

      <h2>shipyard status</h2>
      <pre><code>{`shipyard status [config]
  --format pretty | json   output format (default: pretty)`}</code></pre>
      <p>
        Shows app, host, release root, current release (highlighted with <code>*</code>), every release on the remote with mtime, and lock state. Pretty mode is for humans; JSON is for scripts.
      </p>

      <h2>shipyard releases</h2>
      <pre><code>{`shipyard releases [config]`}</code></pre>
      <p>
        Newest-first list of every <code>releases/&lt;timestamp&gt;</code> directory with the current one marked.
      </p>

      <h2>shipyard prune</h2>
      <pre><code>{`shipyard prune [config]
  --keep <n>               override config; keep this many releases
  --dry-run                list what would be deleted, do not delete`}</code></pre>
      <p>
        Manual prune. The auto-prune step at the end of every successful deploy uses the same code path. The current release is never deleted, even if it&rsquo;s outside the keep window (post-rollback case).
      </p>

      <h2>shipyard doctor</h2>
      <pre><code>{`shipyard doctor [config]
  --config-only            stop after YAML parse + validation; skip SSH checks`}</code></pre>
      <p>
        Validates the config and (for the non-<code>--config-only</code> path, landing in v0.2) attempts an SSH connection plus a writability check on <code>release_root</code> and <code>shared/</code>. Run this in CI to catch typos before they reach a deploy.
      </p>

      <h2>shipyard init</h2>
      <pre><code>{`shipyard init`}</code></pre>
      <p>
        Interactive config generator. Lands in v0.2. Until then, copy <a href="https://github.com/philiprehberger/shipyard/tree/main/examples">an example</a> and edit.
      </p>

      <h2>shipyard version</h2>
      <pre><code>{`shipyard version`}</code></pre>
      <p>
        Prints the semver, short commit, and build date. Reproducible from <code>git checkout</code> + <code>go build -ldflags "…"</code> — the exact command is in the version source file.
      </p>

      <h2>Global flags</h2>
      <pre><code>{`--no-color    disable colored output (also honors NO_COLOR)
--verbose     emit JSON to stderr in addition to pretty stdout
--version     same as 'shipyard version'
--help        help for any command`}</code></pre>

      <h2>Exit codes</h2>
      <table>
        <thead><tr><th>Code</th><th>Meaning</th></tr></thead>
        <tbody>
          <tr><td><code>0</code></td><td>Success.</td></tr>
          <tr><td><code>1</code></td><td>Usage or config error (bad flag, schema validation failure, artifact missing).</td></tr>
          <tr><td><code>2</code></td><td>SSH or transport error (couldn&rsquo;t connect, host-key mismatch, SFTP I/O failure).</td></tr>
          <tr><td><code>3</code></td><td>Deploy step error before health check (extract failed, post_extract failed, atomic flip failed). Release dir cleaned up.</td></tr>
          <tr><td><code>4</code></td><td>Health-check failure after flip. Symlink rolled back to previous release. <code>on_rollback</code> hooks ran.</td></tr>
          <tr><td><code>5</code></td><td>Lock held by another in-flight deploy. <code>--force</code> overrides.</td></tr>
        </tbody>
      </table>

      <hr />
      <p>
        <Link href="/docs/quickstart">← Quickstart</Link>
        {' · '}
        <Link href="/docs/config-reference">← Config reference</Link>
      </p>
    </>
  );
}
