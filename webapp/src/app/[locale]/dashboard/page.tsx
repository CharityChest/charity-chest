'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Link, useRouter } from '@/i18n/navigation';
import { api, ApiError } from '@/lib/api';
import { clearToken, isAuthenticated } from '@/lib/auth';
import ErrorBanner from '@/components/ErrorBanner';
import type { User } from '@/types/api';

export default function DashboardPage() {
  const t = useTranslations();
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!isAuthenticated()) {
      router.replace('/login');
      return;
    }

    api.me().then(setUser).catch((err) => {
      // 401 means the stored token is expired or invalid — force re-login.
      if (err instanceof ApiError && err.status === 401) {
        clearToken();
        router.replace('/login');
      } else {
        setError(err instanceof ApiError ? err.message : 'Failed to load profile');
      }
    });
  }, [router]);

  function handleLogout() {
    clearToken();
    router.push('/');
  }

  if (error) {
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
      <div className="w-full max-w-md space-y-6 rounded-xl border border-gray-200 bg-white p-4 shadow-sm sm:p-8">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-bold text-emerald-700">{t('dashboard.title')}</h1>
          <button
            onClick={handleLogout}
            className="rounded-md border border-gray-300 px-3 py-2 text-sm text-gray-600 hover:bg-gray-50"
          >
            {t('common.logout')}
          </button>
        </div>

        <div className="space-y-1 text-sm">
          <ProfileRow label={t('dashboard.id')} value={String(user.id)} />
          <ProfileRow label={t('dashboard.name')} value={user.name} />
          <ProfileRow label={t('dashboard.email')} value={user.email} />
          <ProfileRow
            label={t('dashboard.memberSince')}
            value={new Date(user.created_at).toLocaleDateString(undefined, {
              year: 'numeric',
              month: 'long',
              day: 'numeric',
            })}
          />
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
