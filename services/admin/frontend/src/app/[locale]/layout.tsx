import type { Metadata } from 'next';
import { NextIntlClientProvider } from 'next-intl';
import { getMessages } from 'next-intl/server';
import { notFound } from 'next/navigation';
import { routing } from '@/i18n/routing';
import Navbar from '@/components/Navbar';
import SystemGuard from '@/components/SystemGuard';
import '../globals.css';

/** Default `<head>` metadata applied to every page under the locale prefix. */
export const metadata: Metadata = {
  title: 'Charity Chest',
  description: 'Charity Chest — giving made easy',
};

/** Tells Next.js which locale segments to pre-render at build time. */
export function generateStaticParams() {
  return routing.locales.map((locale) => ({ locale }));
}

/**
 * Root layout for all locale-prefixed pages.
 * Validates the locale, loads i18n messages, and wraps children with
 * NextIntlClientProvider, the Navbar, and the SystemGuard.
 */
export default async function LocaleLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ locale: string }>;
}) {
  const { locale } = await params;

  if (!(routing.locales as readonly string[]).includes(locale)) {
    notFound();
  }

  const messages = await getMessages();

  return (
    <html lang={locale}>
      <body className="min-h-screen bg-gray-50 pt-14 text-gray-900 antialiased">
        <NextIntlClientProvider messages={messages}>
          <Navbar />
          <SystemGuard>
            {children}
          </SystemGuard>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
