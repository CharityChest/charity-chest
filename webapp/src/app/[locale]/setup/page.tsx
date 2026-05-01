'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { useRouter } from '@/i18n/navigation';
import { api } from '@/lib/api';

/**
 * Shown by SystemGuard when no root user has been seeded yet.
 * Checks GET /v1/system/status once per "Check Again" click and redirects to "/" once configured.
 */
export default function SetupPage() {
  const t = useTranslations('setup');
  const router = useRouter();
  const [checking, setChecking] = useState(false);

  async function handleCheckAgain() {
    setChecking(true);
    try {
      const status = await api.systemStatus();
      if (status.configured) {
        router.replace('/');
      }
    } finally {
      setChecking(false);
    }
  }

  return (
    <main className="flex min-h-screen flex-col items-center justify-center px-4 py-12 sm:px-6 lg:px-8">
      <div className="w-full max-w-sm space-y-6 rounded-xl border border-gray-200 bg-white p-8 shadow-sm text-center">
        <h1 className="text-xl font-bold text-gray-900">{t('title')}</h1>
        <p className="text-sm text-gray-500">{t('description')}</p>
        <button
          onClick={handleCheckAgain}
          disabled={checking}
          className="w-full rounded-md bg-emerald-600 px-4 py-3 text-base font-medium text-white hover:bg-emerald-700 disabled:opacity-50 sm:py-2 sm:text-sm"
        >
          {checking ? t('checking') : t('checkAgain')}
        </button>
      </div>
    </main>
  );
}
