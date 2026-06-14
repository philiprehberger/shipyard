'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useEffect, useState } from 'react';

const NAV_LINKS: Array<
  | { href: string; label: string; external?: false }
  | { href: string; label: string; external: true }
> = [
  { href: '/docs/quickstart', label: 'quickstart' },
  { href: '/docs/config-reference', label: 'config' },
  { href: '/docs/cli', label: 'cli' },
  { href: 'https://github.com/philiprehberger/shipyard', label: 'github', external: true },
];

export function Header() {
  const [open, setOpen] = useState(false);
  const pathname = usePathname();
  const close = () => setOpen(false);

  useEffect(() => {
    setOpen(false);
  }, [pathname]);

  useEffect(() => {
    if (!open) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.body.style.overflow = prev;
    };
  }, [open]);

  return (
    <header className="border-b border-[color:var(--color-line)]">
      <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
        <Link href="/" className="!text-[color:var(--color-paper)] !no-underline">
          <span className="font-mono text-sm font-semibold tracking-tight">
            <span className="text-[color:var(--color-rust-bright)]">▸</span> shipyard
          </span>
        </Link>

        <nav className="hidden md:flex items-center gap-6 text-sm">
          {NAV_LINKS.map((link) =>
            link.external ? (
              <a
                key={link.href}
                href={link.href}
                className="!text-[color:var(--color-paper)] !no-underline opacity-80 hover:opacity-100"
              >
                {link.label}
              </a>
            ) : (
              <Link
                key={link.href}
                href={link.href}
                className="!text-[color:var(--color-paper)] !no-underline opacity-80 hover:opacity-100"
              >
                {link.label}
              </Link>
            ),
          )}
        </nav>

        <button
          type="button"
          aria-label="Open menu"
          aria-expanded={open}
          aria-controls="site-mobile-nav"
          onClick={() => setOpen(true)}
          className="md:hidden inline-flex items-center justify-center rounded-md border border-[color:var(--color-line)] p-2 !text-[color:var(--color-paper)] cursor-pointer"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="22"
            height="22"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <line x1="3" y1="6" x2="21" y2="6" />
            <line x1="3" y1="12" x2="21" y2="12" />
            <line x1="3" y1="18" x2="21" y2="18" />
          </svg>
        </button>
      </div>

      {open && (
        <>
          <div
            className="md:hidden fixed inset-0 z-40 bg-black/60"
            onClick={close}
            aria-hidden="true"
          />
          <div
            id="site-mobile-nav"
            className="md:hidden fixed top-0 right-0 z-50 h-full w-72 max-w-[80vw] bg-[color:var(--color-ink)] border-l border-[color:var(--color-line)] shadow-xl"
            role="dialog"
            aria-modal="true"
            aria-label="Site navigation"
          >
            <div className="flex items-center justify-between px-5 py-4 border-b border-[color:var(--color-line)]">
              <span className="font-mono text-sm font-semibold text-[color:var(--color-paper)]">
                <span className="text-[color:var(--color-rust-bright)]">▸</span> menu
              </span>
              <button
                type="button"
                aria-label="Close menu"
                onClick={close}
                className="inline-flex items-center justify-center rounded-md border border-[color:var(--color-line)] p-2 !text-[color:var(--color-paper)] cursor-pointer"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden="true"
                >
                  <line x1="18" y1="6" x2="6" y2="18" />
                  <line x1="6" y1="6" x2="18" y2="18" />
                </svg>
              </button>
            </div>
            <nav className="flex flex-col px-2 py-3 text-base">
              {NAV_LINKS.map((link) =>
                link.external ? (
                  <a
                    key={link.href}
                    href={link.href}
                    onClick={close}
                    className="px-4 py-3 rounded-md hover:bg-[color:var(--color-line)] !text-[color:var(--color-paper)] !no-underline opacity-90 hover:opacity-100"
                  >
                    {link.label}
                  </a>
                ) : (
                  <Link
                    key={link.href}
                    href={link.href}
                    onClick={close}
                    className="px-4 py-3 rounded-md hover:bg-[color:var(--color-line)] !text-[color:var(--color-paper)] !no-underline opacity-90 hover:opacity-100"
                  >
                    {link.label}
                  </Link>
                ),
              )}
            </nav>
          </div>
        </>
      )}
    </header>
  );
}
