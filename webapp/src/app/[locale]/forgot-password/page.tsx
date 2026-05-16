'use client';

import { FormEvent, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import { api, ApiError } from '@/lib/api';
import ErrorBanner from '@/components/ErrorBanner';

/**
 * Public page that initiates the password-recovery flow.
 *
 * Submits the email to `POST /v1/auth/password/forgot` and replaces the form
 * with a neutral success panel. The server treats unknown emails identically
 * to known ones (enumeration-safe), so the success message is shown whenever
 * the request resolves — we never reveal whether the address exists.
 *
 * Surfaces unexpected non-2xx responses through {@link ErrorBanner}.
 */
export default function ForgotPasswordPage() {
  const t = useTranslations('forgotPassword');

  const [email, setEmail] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [submitted, setSubmitted] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError('');
    if (!email.trim()) {
      setError(t('emailRequired'));
      return;
    }
    setLoading(true);
    try {
      await api.forgotPassword(email.trim());
      setSubmitted(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('send'));
    } finally {
      setLoading(false);
    }
  }

  if (submitted) {
    return (
      <main className="flex min-h-screen items-center justify-center px-4 py-12 sm:px-6 lg:px-8">
        <div className="w-full max-w-sm space-y-6">
          <div className="text-center">
            <h1 className="text-2xl font-bold text-emerald-700">{t('successTitle')}</h1>
            <p className="mt-2 text-sm text-gray-600">{t('successBody')}</p>
          </div>
          <p className="text-center text-sm">
            <Link href="/login" className="text-emerald-600 hover:underline">
              {t('backToLogin')}
            </Link>
          </p>
        </div>
      </main>
    );
  }

  return (
    <main className="flex min-h-screen items-center justify-center px-4 py-12 sm:px-6 lg:px-8">
      <div className="w-full max-w-sm space-y-6">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-emerald-700">{t('title')}</h1>
          <p className="mt-1 text-sm text-gray-500">{t('subtitle')}</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <ErrorBanner message={error} />

          <div>
            <label htmlFor="email" className="block text-sm font-medium text-gray-700">
              {t('email')}
            </label>
            <input
              id="email"
              type="email"
              required
              autoComplete="email"
              autoFocus
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-base focus:border-emerald-500 focus:outline-none sm:text-sm"
              placeholder={t('emailPlaceholder')}
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full rounded-md bg-emerald-600 py-3 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-50 sm:py-2"
          >
            {loading ? t('sending') : t('send')}
          </button>
        </form>

        <p className="text-center text-sm text-gray-500">
          <Link href="/login" className="text-emerald-600 hover:underline">
            {t('backToLogin')}
          </Link>
        </p>
      </div>
    </main>
  );
}
