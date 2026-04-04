'use client';

import { FormEvent, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Link, useRouter } from '@/i18n/navigation';
import { api, ApiError } from '@/lib/api';
import { setToken } from '@/lib/auth';
import ErrorBanner from '@/components/ErrorBanner';

export default function RegisterPage() {
  const t = useTranslations();
  const router = useRouter();
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const { token } = await api.register(email, password, name);
      setToken(token);
      router.push('/dashboard');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('register.submit'));
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center p-8">
      <div className="w-full max-w-sm space-y-6">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-emerald-700">{t('common.appName')}</h1>
          <p className="mt-1 text-sm text-gray-500">{t('register.subtitle')}</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <ErrorBanner message={error} />

          <div>
            <label htmlFor="name" className="block text-sm font-medium text-gray-700">
              {t('register.name')}
            </label>
            <input
              id="name"
              type="text"
              required
              autoComplete="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none"
              placeholder={t('register.namePlaceholder')}
            />
          </div>

          <div>
            <label htmlFor="email" className="block text-sm font-medium text-gray-700">
              {t('register.email')}
            </label>
            <input
              id="email"
              type="email"
              required
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none"
              placeholder="you@example.com"
            />
          </div>

          <div>
            <label htmlFor="password" className="block text-sm font-medium text-gray-700">
              {t('register.password')}{' '}
              <span className="font-normal text-gray-400">{t('register.passwordHint')}</span>
            </label>
            <input
              id="password"
              type="password"
              required
              minLength={8}
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none"
              placeholder="••••••••"
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full rounded-md bg-emerald-600 py-2 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-50"
          >
            {loading ? t('register.submitting') : t('register.submit')}
          </button>
        </form>

        <p className="text-center text-sm text-gray-500">
          {t('register.alreadyHave')}{' '}
          <Link href="/login" className="text-emerald-600 hover:underline">
            {t('register.signIn')}
          </Link>
        </p>
      </div>
    </main>
  );
}
