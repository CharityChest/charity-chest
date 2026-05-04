'use client';

import { Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';

function CancelContent() {
  const t = useTranslations('billing');
  const params = useSearchParams();
  const orgId = params.get('org_id');

  return (
    <main className="flex min-h-screen items-center justify-center px-4 py-12 sm:px-6 lg:px-8">
      <div className="w-full max-w-sm space-y-6 text-center">
        <div className="flex items-center justify-center">
          <span className="flex h-14 w-14 items-center justify-center rounded-full bg-gray-100 text-3xl">
            ×
          </span>
        </div>
        <h1 className="text-xl font-bold text-gray-900">{t('cancelTitle')}</h1>
        <p className="text-sm text-gray-500">{t('cancelBody')}</p>
        {orgId && (
          <Link
            href={`/orgs/${orgId}`}
            className="inline-block rounded-md border border-gray-200 px-4 py-3 text-sm text-gray-600 hover:bg-gray-50 sm:py-2"
          >
            ← {t('backToOrg')}
          </Link>
        )}
      </div>
    </main>
  );
}

export default function BillingCancelPage() {
  return (
    <Suspense>
      <CancelContent />
    </Suspense>
  );
}
