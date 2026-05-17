'use client';

import { FormEvent, useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Link, useRouter } from '@/i18n/navigation';
import { api, ApiError } from '@/lib/api';
import ErrorBanner from '@/components/ErrorBanner';

/**
 * Page that consumes a password-reset token delivered via email.
 *
 * Reads the token from `?token=` in the URL (using `window.location.search`
 * to avoid Next 15's `useSearchParams` + `<Suspense>` requirement, matching
 * the pattern in `auth/callback/page.tsx`). Validates the password and its
 * confirmation client-side, then submits to `POST /v1/auth/password/reset`.
 *
 * On success, switches to a confirmation panel with a "sign in" link — we
 * deliberately do NOT auto-login: the user must re-authenticate (and pass
 * MFA, if enabled).
 */
export default function ResetPasswordPage() {
  const t = useTranslations('resetPassword');
  const router = useRouter();

  const [token, setToken] = useState<string | null>(null);
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [done, setDone] = useState(false);

  // Token resolution runs in an effect so the page renders identically on the
  // server (where window is undefined) and on the client. A missing token
  // surfaces as an inline error rather than blocking the entire view.
  useEffect(() => {
    const url = new URL(window.location.href);
    const t = url.searchParams.get('token');
    if (t && t.length > 0) {
      setToken(t);
      // Strip the token from the address bar so it cannot leak via browser
      // history, the Referer header on any outbound link, or screen-sharing.
      url.searchParams.delete('token');
      window.history.replaceState(null, '', url.pathname + url.search + url.hash);
    } else {
      setToken('');
    }
  }, []);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError('');
    if (!token) {
      setError(t('missingToken'));
      return;
    }
    if (password.length < 8) {
      setError(t('tooShort'));
      return;
    }
    if (password !== confirm) {
      setError(t('mismatch'));
      return;
    }
    setLoading(true);
    try {
      await api.resetPassword(token, password);
      setDone(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('submitError'));
    } finally {
      setLoading(false);
    }
  }

  // Confirmation panel — the user clicks through to /login to sign in.
  if (done) {
    return (
      <main className="flex min-h-screen items-center justify-center px-4 py-12 sm:px-6 lg:px-8">
        <div className="w-full max-w-sm space-y-6 text-center">
          <h1 className="text-2xl font-bold text-emerald-700">{t('successTitle')}</h1>
          <p className="text-sm text-gray-600">{t('successBody')}</p>
          <button
            onClick={() => router.replace('/login')}
            className="w-full rounded-md bg-emerald-600 py-3 text-sm font-medium text-white hover:bg-emerald-700 sm:py-2"
          >
            {t('backToLogin')}
          </button>
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

          {/* Show a banner-style notice when token is missing — the form still
              renders so a curious user understands what was expected. */}
          {token === '' && (
            <ErrorBanner message={t('missingToken')} />
          )}

          <div>
            <label htmlFor="password" className="block text-sm font-medium text-gray-700">
              {t('password')}{' '}
              <span className="text-xs font-normal text-gray-400">{t('passwordHint')}</span>
            </label>
            <input
              id="password"
              type="password"
              required
              minLength={8}
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-base focus:border-emerald-500 focus:outline-none sm:text-sm"
              placeholder="••••••••"
            />
          </div>

          <div>
            <label htmlFor="confirm" className="block text-sm font-medium text-gray-700">
              {t('confirm')}
            </label>
            <input
              id="confirm"
              type="password"
              required
              minLength={8}
              autoComplete="new-password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-base focus:border-emerald-500 focus:outline-none sm:text-sm"
              placeholder="••••••••"
            />
          </div>

          <button
            type="submit"
            disabled={loading || !token}
            className="w-full rounded-md bg-emerald-600 py-3 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-50 sm:py-2"
          >
            {loading ? t('submitting') : t('submit')}
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
