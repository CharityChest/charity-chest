'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Link, useRouter } from '@/i18n/navigation';
import { api, ApiError } from '@/lib/api';
import { clearToken, getRole, isAuthenticated } from '@/lib/auth';
import ErrorBanner from '@/components/ErrorBanner';
import type { Organization } from '@/types/api';

export default function OrgsPage() {
  const t = useTranslations();
  const router = useRouter();
  const [ready, setReady] = useState(false);

  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [loadError, setLoadError] = useState('');

  const [newName, setNewName] = useState('');
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');
  const [deleteError, setDeleteError] = useState('');

  useEffect(() => {
    if (!isAuthenticated()) {
      router.replace('/login');
      return;
    }
    const r = getRole();
    if (r !== 'root' && r !== 'system') {
      router.replace('/dashboard');
      return;
    }
    setReady(true);
    api.listOrgs().then(setOrgs).catch((err) => {
      if (err instanceof ApiError && err.status === 401) {
        clearToken();
        router.replace('/login');
      } else {
        setLoadError(err instanceof ApiError ? err.message : t('orgs.loadFailed'));
      }
    });
    // Mount-only: `t` is intentionally excluded so a re-render (e.g. after
    // creating an org) doesn't re-fetch and clobber local list state.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [router]);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!newName.trim()) return;
    setCreating(true);
    setCreateError('');
    try {
      const org = await api.createOrg(newName.trim());
      setOrgs((prev) => [...prev, org]);
      setNewName('');
    } catch (err) {
      setCreateError(err instanceof ApiError ? err.message : t('orgs.createFailed'));
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete(orgUuid: string) {
    if (!confirm(t('orgs.deleteOrg') + '?')) return;
    setDeleteError('');
    try {
      await api.deleteOrg(orgUuid);
      setOrgs((prev) => prev.filter((o) => o.uuid !== orgUuid));
    } catch (err) {
      setDeleteError(err instanceof ApiError ? err.message : t('orgs.deleteFailed'));
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
      <div className="w-full max-w-2xl space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-bold text-emerald-700">{t('orgs.title')}</h1>
          <Link href="/dashboard" className="text-sm text-gray-500 hover:underline">
            ← {t('dashboard.title')}
          </Link>
        </div>

        {/* Create form */}
        <form
          onSubmit={handleCreate}
          className="flex gap-2 rounded-xl border border-gray-200 bg-white p-4 shadow-sm"
        >
          <input
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder={t('orgs.orgNamePlaceholder')}
            className="min-w-0 flex-1 rounded-md border border-gray-300 px-3 py-2 text-base text-gray-900 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 sm:text-sm"
          />
          <button
            type="submit"
            disabled={creating || !newName.trim()}
            className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-50"
          >
            {creating ? t('orgs.creating') : t('orgs.create')}
          </button>
        </form>
        <ErrorBanner message={createError} />
        <ErrorBanner message={loadError} />
        <ErrorBanner message={deleteError} />

        {/* Org list */}
        {orgs.length === 0 ? (
          <p className="text-center text-sm text-gray-400">{t('orgs.noOrgs')}</p>
        ) : (
          <ul className="space-y-2">
            {orgs.map((org) => (
              <li
                key={org.uuid}
                className="flex items-center justify-between rounded-xl border border-gray-200 bg-white px-4 py-3 shadow-sm"
              >
                <div className="min-w-0">
                  <Link
                    href={`/orgs/${org.uuid}`}
                    className="font-medium text-emerald-700 hover:underline"
                  >
                    {org.name}
                  </Link>
                  <p className="break-all text-xs text-gray-400">#{org.uuid}</p>
                </div>
                <div className="flex shrink-0 gap-2">
                  <Link
                    href={`/orgs/${org.uuid}`}
                    className="rounded-md border border-gray-200 px-3 py-1.5 text-xs text-gray-600 hover:bg-gray-50"
                  >
                    {t('orgs.members')}
                  </Link>
                  <button
                    onClick={() => handleDelete(org.uuid)}
                    className="rounded-md border border-red-200 px-3 py-1.5 text-xs text-red-600 hover:bg-red-50"
                  >
                    {t('orgs.deleteOrg')}
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </main>
  );
}
