import Link from 'next/link';

export function Header() {
  return (
    <header className="border-b border-[color:var(--color-line)]">
      <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
        <Link href="/" className="!text-[color:var(--color-paper)] !no-underline">
          <span className="font-mono text-sm font-semibold tracking-tight">
            <span className="text-[color:var(--color-rust-bright)]">▸</span> shipyard
          </span>
        </Link>
        <nav className="flex items-center gap-6 text-sm">
          <Link href="/docs/quickstart" className="!text-[color:var(--color-paper)] !no-underline opacity-80 hover:opacity-100">
            quickstart
          </Link>
          <Link href="/docs/config-reference" className="!text-[color:var(--color-paper)] !no-underline opacity-80 hover:opacity-100">
            config
          </Link>
          <Link href="/docs/cli" className="!text-[color:var(--color-paper)] !no-underline opacity-80 hover:opacity-100">
            cli
          </Link>
          <a
            href="https://github.com/philiprehberger/shipyard"
            className="!text-[color:var(--color-paper)] !no-underline opacity-80 hover:opacity-100"
          >
            github
          </a>
        </nav>
      </div>
    </header>
  );
}
