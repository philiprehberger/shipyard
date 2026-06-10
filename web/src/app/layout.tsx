import type { Metadata } from 'next';
import './globals.css';
import { Header } from '@/components/Header';
import { Footer } from '@/components/Footer';

export const metadata: Metadata = {
  title: {
    default: 'Shipyard — atomic-release deploy CLI',
    template: '%s — Shipyard',
  },
  description:
    'Zero-downtime SSH deploys with health-gated promotion and automatic rollback. One static Go binary, one YAML config, no agent on the server.',
  metadataBase: new URL('https://shipyard.philiprehberger.com'),
  openGraph: {
    title: 'Shipyard — atomic-release deploy CLI',
    description:
      'Zero-downtime SSH deploys with health-gated promotion and automatic rollback.',
    url: 'https://shipyard.philiprehberger.com',
    siteName: 'Shipyard',
    type: 'website',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Shipyard — atomic-release deploy CLI',
    description:
      'Zero-downtime SSH deploys with health-gated promotion and automatic rollback.',
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <Header />
        <main className="mx-auto max-w-3xl px-6 py-16 sm:py-24">{children}</main>
        <Footer />
      </body>
    </html>
  );
}
