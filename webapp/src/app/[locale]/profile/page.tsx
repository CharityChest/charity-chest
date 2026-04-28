'use client';

import { FormEvent, useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { QRCodeSVG } from 'qrcode.react';
import { Link, useRouter } from '@/i18n/navigation';
import { api, ApiError } from '@/lib/api';
import { clearToken, isAuthenticated } from '@/lib/auth';
import ErrorBanner from '@/components/ErrorBanner';
import type { User, MFASetupResponse } from '@/types/api';

type MFAStep = 'idle' | 'setup' | 'disabling';

export default function ProfilePage() {
  const t = useTranslations();
  const router = useRouter();

  const [user, setUser] = useState<User | null>(null);
  const [error, setError] = useState('');
  const [mfaStep, setMFAStep] = useState<MFAStep>('idle');
  const [setupData, setSetupData] = useState<MFASetupResponse | null>(null);
  const [code, setCode] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!isAuthenticated()) {
      router.replace('/login');
      return;
    }
    api.me().then(setUser).catch((err) => {
      if (err instanceof ApiError && err.status === 401) {
        clearToken();
        router.replace('/login');
      } else {
        setError(err instanceof ApiError ? err.message : 'Failed to load profile');
      }
    });
  }, [router]);

  async function handleStartSetup() {
    setError('');
    setLoading(true);
    try {
      const data = await api.mfaSetup();
      setSetupData(data);
      setMFAStep('setup');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Setup failed');
    } finally {
      setLoading(false);
    }
  }

  async function handleEnableMFA(e: FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await api.mfaEnable(code);
      const updated = await api.me();
      setUser(updated);
      setMFAStep('idle');
      setSetupData(null);
      setCode('');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Enable failed');
    } finally {
      setLoading(false);
    }
  }

  async function handleDisableMFA(e: FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await api.mfaDisable(code);
      const updated = await api.me();
      setUser(updated);
      setMFAStep('idle');
      setCode('');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Disable failed');
    } finally {
      setLoading(false);
    }
  }

  function handleCancelSetup() {
    setMFAStep('idle');
    setSetupData(null);
    setCode('');
    setError('');
  }

  if (error && !user) {
    return (
      <main className="flex min-h-screen items-center justify-center px-4 py-12 sm:px-6 lg:px-8">
        <div className="w-full max-w-sm space-y-4">
          <ErrorBanner message={error} />
          <p className="text-center">
            <Link href="/" className="text-sm text-emerald-600 hover:underline">
              {t('common.goHome')}
            </Link>
          </p>
        </div>
      </main>
    );
  }

  if (!user) {
    return (
      <main className="flex min-h-screen items-center justify-center px-4 py-12 sm:px-6 lg:px-8">
        <p className="text-gray-400">{t('common.loading')}</p>
      </main>
    );
  }

  return (
    <main className="flex min-h-screen flex-col items-center justify-center px-4 py-12 sm:px-6 lg:px-8">
      <div className="w-full max-w-md space-y-4">

        {/* User info */}
        <div className="space-y-4 rounded-xl border border-gray-200 bg-white p-4 shadow-sm sm:p-8">
          <h1 className="text-xl font-bold text-emerald-700">{t('profile.title')}</h1>
          <div className="space-y-1 text-sm">
            <ProfileRow label={t('dashboard.name')} value={user.name} />
            <ProfileRow label={t('dashboard.email')} value={user.email} />
          </div>
        </div>

        {/* MFA section */}
        <div className="rounded-xl border border-gray-200 bg-white p-4 shadow-sm sm:p-8">
          <h2 className="mb-4 text-base font-semibold text-gray-800">{t('profile.mfaSection')}</h2>

          <ErrorBanner message={error} />

          {/* Setup flow */}
          {mfaStep === 'setup' && setupData && (
            <div className="space-y-4">
              <p className="text-sm text-gray-600">{t('profile.scanQR')}</p>
              <div className="flex justify-center rounded-lg border border-gray-200 bg-white p-4">
                <QRCodeSVG value={setupData.uri} size={180} />
              </div>
              <div>
                <p className="mb-1 text-xs text-gray-500">{t('profile.orEnterKey')}</p>
                <code className="block break-all rounded bg-gray-100 px-3 py-2 text-xs font-mono text-gray-800">
                  {setupData.secret}
                </code>
              </div>
              <form onSubmit={handleEnableMFA} className="space-y-3">
                <div>
                  <label htmlFor="enable-code" className="block text-sm font-medium text-gray-700">
                    {t('profile.verifyCode')}
                  </label>
                  <input
                    id="enable-code"
                    type="text"
                    inputMode="numeric"
                    maxLength={6}
                    required
                    autoComplete="one-time-code"
                    autoFocus
                    value={code}
                    onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
                    className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-center text-xl tracking-widest focus:border-emerald-500 focus:outline-none"
                    placeholder={t('profile.codePlaceholder')}
                  />
                </div>
                <div className="flex gap-2">
                  <button
                    type="submit"
                    disabled={loading || code.length !== 6}
                    className="flex-1 rounded-md bg-emerald-600 py-2 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-50"
                  >
                    {loading ? '…' : t('profile.verifyEnable')}
                  </button>
                  <button
                    type="button"
                    onClick={handleCancelSetup}
                    className="rounded-md border border-gray-300 px-4 py-2 text-sm text-gray-600 hover:bg-gray-50"
                  >
                    {t('profile.cancel')}
                  </button>
                </div>
              </form>
            </div>
          )}

          {/* Disable flow */}
          {mfaStep === 'disabling' && (
            <form onSubmit={handleDisableMFA} className="space-y-3">
              <div>
                <label htmlFor="disable-code" className="block text-sm font-medium text-gray-700">
                  {t('profile.verifyCode')}
                </label>
                <input
                  id="disable-code"
                  type="text"
                  inputMode="numeric"
                  maxLength={6}
                  required
                  autoComplete="one-time-code"
                  autoFocus
                  value={code}
                  onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-center text-xl tracking-widest focus:border-emerald-500 focus:outline-none"
                  placeholder={t('profile.codePlaceholder')}
                />
              </div>
              <div className="flex gap-2">
                <button
                  type="submit"
                  disabled={loading || code.length !== 6}
                  className="flex-1 rounded-md bg-red-600 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50"
                >
                  {loading ? '…' : t('profile.verifyDisable')}
                </button>
                <button
                  type="button"
                  onClick={() => { setMFAStep('idle'); setCode(''); setError(''); }}
                  className="rounded-md border border-gray-300 px-4 py-2 text-sm text-gray-600 hover:bg-gray-50"
                >
                  {t('profile.cancel')}
                </button>
              </div>
            </form>
          )}

          {/* Idle state */}
          {mfaStep === 'idle' && (
            <div className="flex items-center justify-between">
              {user.mfa_enabled ? (
                <>
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-100 px-3 py-1 text-sm font-medium text-emerald-700">
                    <span aria-hidden="true">✓</span>
                    {t('profile.mfaEnabled')}
                  </span>
                  <button
                    onClick={() => { setMFAStep('disabling'); setError(''); }}
                    className="rounded-md border border-red-300 px-3 py-2 text-sm text-red-600 hover:bg-red-50"
                  >
                    {t('profile.disableMFA')}
                  </button>
                </>
              ) : (
                <>
                  <span className="text-sm text-gray-500">{t('profile.mfaDisabled')}</span>
                  <button
                    onClick={handleStartSetup}
                    disabled={loading}
                    className="rounded-md bg-emerald-600 px-3 py-2 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-50"
                  >
                    {loading ? '…' : t('profile.enableMFA')}
                  </button>
                </>
              )}
            </div>
          )}
        </div>

        <div className="text-center">
          <Link href="/dashboard" className="text-sm text-emerald-600 hover:underline">
            {t('profile.backToDashboard')}
          </Link>
        </div>

      </div>
    </main>
  );
}

function ProfileRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between border-b border-gray-100 py-2">
      <span className="text-gray-500">{label}</span>
      <span className="min-w-0 truncate pl-4 font-medium">{value}</span>
    </div>
  );
}
