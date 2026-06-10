export function Footer() {
  return (
    <footer className="border-t border-[color:var(--color-line)] py-8">
      <div className="mx-auto flex max-w-5xl flex-col items-start gap-2 px-6 text-xs text-[color:var(--color-steel-light)] sm:flex-row sm:items-center sm:justify-between">
        <p>
          Shipyard is an open-source project by{' '}
          <a href="https://philiprehberger.com" className="!text-[color:var(--color-steel-light)] hover:!text-[color:var(--color-paper)]">
            Philip Rehberger
          </a>
          . MIT licensed.
        </p>
        <p>
          <a href="https://github.com/philiprehberger/shipyard" className="!text-[color:var(--color-steel-light)] hover:!text-[color:var(--color-paper)]">
            github
          </a>
          {' · '}
          <a href="https://github.com/philiprehberger/shipyard/blob/main/CHANGELOG.md" className="!text-[color:var(--color-steel-light)] hover:!text-[color:var(--color-paper)]">
            changelog
          </a>
          {' · '}
          <a href="https://github.com/philiprehberger/shipyard/issues" className="!text-[color:var(--color-steel-light)] hover:!text-[color:var(--color-paper)]">
            issues
          </a>
        </p>
      </div>
    </footer>
  );
}
