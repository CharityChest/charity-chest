'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Link, useRouter } from '@/i18n/navigation';
import { api, ApiError } from '@/lib/api';
import { clearToken, getRole, isAuthenticated } from '@/lib/auth';
import ErrorBanner from '@/components/ErrorBanner';
import type { User } from '@/types/api';

export default function DashboardPage() {
  const t = useTranslations();
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [error, setError] = useState('');
  const role = getRole(); // read once — stable for the page lifetime

  // Org quick-access state (for users with no system role)
  const [orgIdInput, setOrgIdInput] = useState('');

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

  function handleLogout() {
    clearToken();
    router.push('/');
  }

  function handleGoToOrg(e: React.FormEvent) {
    e.preventDefault();
    const id = parseInt(orgIdInput, 10);
    if (!isNaN(id) && id > 0) router.push(`/orgs/${id}`);
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
      <div className="w-full max-w-md space-y-4">

        {/* Profile card */}
        <div className="space-y-6 rounded-xl border border-gray-200 bg-white p-4 shadow-sm sm:p-8">
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
              label={t('dashboard.role')}
              value={user.role ? roleName(user.role) : t('dashboard.noRole')}
            />
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

        {/* Role-based quick actions */}
        {role === 'root' && (
          <div className="grid grid-cols-2 gap-3">
            <QuickLink href="/orgs" label={t('dashboard.manageOrgs')} icon="🏢" />
            <QuickLink href="/admin/users" label={t('dashboard.manageUsers')} icon="👤" />
          </div>
        )}

        {role === 'system' && (
          <div className="grid grid-cols-1 gap-3">
            <QuickLink href="/orgs" label={t('dashboard.manageOrgs')} icon="🏢" />
          </div>
        )}

        {/* Org-level users and roleless users: quick org access form */}
        {role !== 'root' && role !== 'system' && (
          <div className="rounded-xl border border-gray-200 bg-white p-4 shadow-sm">
            <h2 className="mb-2 text-sm font-medium text-gray-700">{t('dashboard.orgAccess')}</h2>
            <p className="mb-3 text-xs text-gray-400">{t('dashboard.orgAccessHint')}</p>
            <form onSubmit={handleGoToOrg} className="flex gap-2">
              <input
                type="number"
                min={1}
                value={orgIdInput}
                onChange={(e) => setOrgIdInput(e.target.value)}
                placeholder={t('dashboard.orgIdPlaceholder')}
                className="min-w-0 flex-1 rounded-md border border-gray-300 px-3 py-2 text-base text-gray-900 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 sm:text-sm"
              />
              <button
                type="submit"
                disabled={!orgIdInput}
                className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-50"
              >
                {t('dashboard.goToOrg')}
              </button>
            </form>
          </div>
        )}

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

function QuickLink({ href, label, icon }: { href: string; label: string; icon: string }) {
  return (
    <Link
      href={href}
      className="flex items-center gap-2 rounded-xl border border-gray-200 bg-white px-4 py-3 text-sm font-medium text-gray-700 shadow-sm hover:border-emerald-300 hover:bg-emerald-50 hover:text-emerald-700"
    >
      <span aria-hidden="true">{icon}</span>
      {label}
    </Link>
  );
}

function roleName(role: string): string {
  const map: Record<string, string> = {
    root: 'Root',
    system: 'System',
  };
  return map[role] ?? role;
}
