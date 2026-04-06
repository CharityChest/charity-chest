'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Link, useRouter } from '@/i18n/navigation';
import { api, ApiError } from '@/lib/api';
import { clearToken, getRole, isAuthenticated } from '@/lib/auth';
import ErrorBanner from '@/components/ErrorBanner';
import type { User } from '@/types/api';

export default function AdminUsersPage() {
  const t = useTranslations();
  const router = useRouter();
  const [ready, setReady] = useState(false);

  const [userId, setUserId] = useState('');
  const [role, setRole] = useState('system');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState<User | null>(null);

  useEffect(() => {
    if (!isAuthenticated()) {
      router.replace('/login');
      return;
    }
    const r = getRole();
    if (r !== 'root') {
      router.replace('/dashboard');
      return;
    }
    setReady(true);
  }, [router]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setResult(null);
    const id = parseInt(userId, 10);
    if (!userId || isNaN(id)) return;
    setSubmitting(true);
    try {
      const updated = await api.assignSystemRole(id, role === 'none' ? '' : role);
      setResult(updated);
      setUserId('');
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        clearToken();
        router.replace('/login');
      } else {
        setError(err instanceof ApiError ? err.message : 'Request failed');
      }
    } finally {
      setSubmitting(false);
    }
  }

  if (!ready) {
    return (
      <main className="flex min-h-screen items-center justify-center px-4 py-12">
        <p className="text-gray-400">{t('common.loading')}</p>
      </main>
    );
  }

  return (
    <main className="flex min-h-screen flex-col items-center justify-start px-4 py-12 sm:px-6 lg:px-8">
      <div className="w-full max-w-md space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-bold text-emerald-700">{t('adminUsers.title')}</h1>
          <Link href="/dashboard" className="text-sm text-gray-500 hover:underline">
            ← {t('dashboard.title')}
          </Link>
        </div>

        <p className="text-sm text-gray-500">{t('adminUsers.description')}</p>

        <form
          onSubmit={handleSubmit}
          className="space-y-4 rounded-xl border border-gray-200 bg-white p-6 shadow-sm"
        >
          <ErrorBanner message={error} />

          <div className="space-y-1">
            <label className="block text-sm font-medium text-gray-700">
              {t('adminUsers.userId')}
            </label>
            <input
              type="number"
              min={1}
              value={userId}
              onChange={(e) => setUserId(e.target.value)}
              placeholder={t('adminUsers.userIdPlaceholder')}
              required
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-base text-gray-900 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 sm:text-sm"
            />
          </div>

          <div className="space-y-1">
            <label className="block text-sm font-medium text-gray-700">
              {t('adminUsers.role')}
            </label>
            <select
              value={role}
              onChange={(e) => setRole(e.target.value)}
              className="w-full rounded-md border border-gray-300 px-3 py-2 text-base text-gray-900 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 sm:text-sm"
            >
              <option value="system">{t('adminUsers.systemRole')}</option>
              <option value="none">{t('adminUsers.noRole')}</option>
            </select>
          </div>

          <button
            type="submit"
            disabled={submitting || !userId}
            className="w-full rounded-md bg-emerald-600 px-4 py-3 text-base font-medium text-white hover:bg-emerald-700 disabled:opacity-50 sm:py-2 sm:text-sm"
          >
            {submitting ? t('adminUsers.assigning') : t('adminUsers.assign')}
          </button>
        </form>

        {result && (
          <div className="rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-800">
            <p className="font-medium">{t('adminUsers.result')}</p>
            <p className="mt-1 text-gray-600">
              #{result.id} · {result.email} ·{' '}
              <span className="font-medium">{result.role ?? t('dashboard.noRole')}</span>
            </p>
          </div>
        )}
      </div>
    </main>
  );
}
