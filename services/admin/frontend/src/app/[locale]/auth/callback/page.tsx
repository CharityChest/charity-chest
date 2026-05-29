'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Link, useRouter } from '@/i18n/navigation';
import { setToken } from '@/lib/auth';
import ErrorBanner from '@/components/ErrorBanner';

/**
 * Landing page for the Google OAuth callback (`/auth/callback`).
 * Reads `?token=` from the URL, stores it via {@link setToken}, then redirects to /dashboard.
 * Renders an error banner when `?token=` is absent — using the `?error=` value if provided,
 * or falling back to `sign_in_failed` when neither parameter is present.
 */
export default function AuthCallbackPage() {
  const t = useTranslations('authCallback');
  const router = useRouter();
  const [error, setError] = useState('');

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const token = params.get('token');
    const err = params.get('error');

    if (token) {
      setToken(token);
      router.replace('/dashboard');
    } else {
      setError(err ?? 'sign_in_failed');
    }
  }, [router]);

  if (error) {
    return (
      <main className="flex min-h-screen items-center justify-center px-4 py-12 sm:px-6 lg:px-8">
        <div className="w-full max-w-sm space-y-4">
          <ErrorBanner message={t('error')} />
          <p className="text-center">
            <Link href="/login" className="text-sm text-emerald-600 hover:underline">
              {t('tryAgain')}
            </Link>
          </p>
        </div>
      </main>
    );
  }

  return (
    <main className="flex min-h-screen items-center justify-center p-8">
      <p className="text-gray-400">{t('completing')}</p>
    </main>
  );
}