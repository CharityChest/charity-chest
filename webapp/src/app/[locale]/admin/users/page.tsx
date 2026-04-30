'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Link, useRouter } from '@/i18n/navigation';
import { api, ApiError } from '@/lib/api';
import { clearToken, getRole, isAuthenticated } from '@/lib/auth';
import ErrorBanner from '@/components/ErrorBanner';
import type { PaginatedResult, User, UserWithOrgs } from '@/types/api';

export default function AdminUsersPage() {
  const t = useTranslations();
  const router = useRouter();
  const [ready, setReady] = useState(false);

  // --- Search state ---
  const [searchEmail, setSearchEmail] = useState('');
  const [searchPage, setSearchPage] = useState(1);
  const searchSize = 20;
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState('');
  const [searchResult, setSearchResult] = useState<PaginatedResult<UserWithOrgs> | null>(null);

  // --- Role assignment state ---
  const [userId, setUserId] = useState('');
  const [role, setRole] = useState('system');
  const [submitting, setSubmitting] = useState(false);
  const [assignError, setAssignError] = useState('');
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

  async function handleSearch(e: React.FormEvent, page = searchPage) {
    e.preventDefault();
    setSearchError('');
    setSearching(true);
    try {
      const res = await api.searchUsers(searchEmail, page, searchSize);
      setSearchResult(res);
      setSearchPage(page);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        clearToken();
        router.replace('/login');
      } else {
        setSearchError(err instanceof ApiError ? err.message : 'Request failed');
      }
    } finally {
      setSearching(false);
    }
  }

  function handlePageChange(e: React.MouseEvent, page: number) {
    e.preventDefault();
    handleSearch(e as unknown as React.FormEvent, page);
  }

  async function handleAssign(e: React.FormEvent) {
    e.preventDefault();
    setAssignError('');
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
        setAssignError(err instanceof ApiError ? err.message : 'Request failed');
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

  const meta = searchResult?.metadata;
  const totalPages = meta?.total_pages ?? 1;

  return (
    <main className="flex min-h-screen flex-col items-start px-4 py-12 sm:px-6 lg:px-8">
      <div className="w-full max-w-4xl space-y-8">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-bold text-emerald-700">{t('adminUsers.title')}</h1>
          <Link href="/dashboard" className="text-sm text-gray-500 hover:underline">
            ← {t('dashboard.title')}
          </Link>
        </div>

        {/* --- Search section --- */}
        <section className="space-y-4">
          <h2 className="text-base font-semibold text-gray-700">{t('adminUsers.searchSection')}</h2>

          <form onSubmit={handleSearch} className="flex gap-2">
            <input
              type="text"
              value={searchEmail}
              onChange={(e) => setSearchEmail(e.target.value)}
              placeholder={t('adminUsers.searchEmailPlaceholder')}
              className="flex-1 rounded-md border border-gray-300 px-3 py-2 text-base text-gray-900 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 sm:text-sm"
            />
            <button
              type="submit"
              disabled={searching}
              className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-50"
            >
              {searching ? t('adminUsers.searching') : t('adminUsers.search')}
            </button>
          </form>

          <ErrorBanner message={searchError} />

          {searchResult && (
            <div className="space-y-2">
              <div className="overflow-x-auto rounded-xl border border-gray-200 bg-white shadow-sm">
                <table className="w-full text-sm">
                  <thead className="border-b border-gray-100 bg-gray-50 text-left text-xs font-medium uppercase tracking-wide text-gray-500">
                    <tr>
                      <th className="px-4 py-3">{t('adminUsers.colId')}</th>
                      <th className="px-4 py-3">{t('adminUsers.colEmail')}</th>
                      <th className="px-4 py-3">{t('adminUsers.colRole')}</th>
                      <th className="px-4 py-3">{t('adminUsers.colOrganizations')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {searchResult.data.length === 0 ? (
                      <tr>
                        <td colSpan={4} className="px-4 py-6 text-center text-gray-400">
                          {t('adminUsers.noResults')}
                        </td>
                      </tr>
                    ) : (
                      searchResult.data.map((u) => (
                        <tr key={u.id} className="hover:bg-gray-50">
                          <td className="px-4 py-3 text-gray-500">{u.id}</td>
                          <td className="px-4 py-3 font-medium text-gray-900">{u.email}</td>
                          <td className="px-4 py-3 text-gray-600">{u.role ?? '—'}</td>
                          <td className="px-4 py-3 text-gray-600">
                            {u.organizations.length === 0
                              ? '—'
                              : u.organizations.map((o) => `${o.name} (${o.role})`).join(', ')}
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>

              {/* Pagination */}
              <div className="flex items-center justify-between text-sm text-gray-500">
                <button
                  onClick={(e) => handlePageChange(e, searchPage - 1)}
                  disabled={searching || searchPage <= 1}
                  className="rounded px-3 py-1 hover:bg-gray-100 disabled:opacity-40"
                >
                  ← {t('adminUsers.prevPage')}
                </button>
                <span>
                  {t('adminUsers.pageInfo', { page: searchPage, totalPages })}
                </span>
                <button
                  onClick={(e) => handlePageChange(e, searchPage + 1)}
                  disabled={searching || searchPage >= totalPages}
                  className="rounded px-3 py-1 hover:bg-gray-100 disabled:opacity-40"
                >
                  {t('adminUsers.nextPage')} →
                </button>
              </div>
            </div>
          )}
        </section>

        {/* --- Role assignment section --- */}
        <section className="space-y-4">
          <h2 className="text-base font-semibold text-gray-700">{t('adminUsers.assignRoleSection')}</h2>
          <p className="text-sm text-gray-500">{t('adminUsers.description')}</p>

          <form
            onSubmit={handleAssign}
            className="space-y-4 rounded-xl border border-gray-200 bg-white p-6 shadow-sm"
          >
            <ErrorBanner message={assignError} />

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
        </section>
      </div>
    </main>
  );
}
