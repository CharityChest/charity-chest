'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { useLocale } from 'next-intl';
import { Link, useRouter } from '@/i18n/navigation';
import { api, ApiError } from '@/lib/api';
import { clearToken, getRole, isAuthenticated } from '@/lib/auth';
import ErrorBanner from '@/components/ErrorBanner';
import type { Organization, OrganizationMember } from '@/types/api';

// Roles an actor may assign, given their own org-level role.
// System/root can assign any org role — handled separately.
const ASSIGNABLE: Record<string, string[]> = {
  owner: ['admin', 'operational'],
  admin: ['operational'],
  operational: [],
};

export default function OrgDetailPage({
  params,
}: {
  params: Promise<{ orgID: string }>;
}) {
  const t = useTranslations();
  const router = useRouter();
  const locale = useLocale();
  const [orgId, setOrgId] = useState<number | null>(null);

  const [org, setOrg] = useState<Organization | null>(null);
  const [members, setMembers] = useState<OrganizationMember[]>([]);
  const [loadError, setLoadError] = useState('');

  // The calling user's effective org-level role (derived from membership list after load).
  // System/root users have access to all roles.
  const [myUserId, setMyUserId] = useState<number | null>(null);
  const [myOrgRole, setMyOrgRole] = useState<string | null>(null);
  const systemRole = getRole(); // "root" | "system" | null

  // Edit-name state
  const [editing, setEditing] = useState(false);
  const [editName, setEditName] = useState('');
  const [saving, setSaving] = useState(false);
  const [editError, setEditError] = useState('');

  // Add-member state
  const [newUserId, setNewUserId] = useState('');
  const [newRole, setNewRole] = useState('operational');
  const [adding, setAdding] = useState(false);
  const [addError, setAddError] = useState('');

  // Change-role state (per member)
  const [changingRole, setChangingRole] = useState<number | null>(null);
  const [pendingRole, setPendingRole] = useState('');

  // Billing state
  const [checkingOut, setCheckingOut] = useState(false);
  const [billingError, setBillingError] = useState('');
  const [activatingEnterprise, setActivatingEnterprise] = useState(false);
  const [cancellingSubscription, setCancellingSubscription] = useState(false);

  useEffect(() => {
    params.then(({ orgID }) => setOrgId(parseInt(orgID, 10)));
  }, [params]);

  useEffect(() => {
    if (!isAuthenticated()) {
      router.replace('/login');
      return;
    }
    if (orgId === null || isNaN(orgId)) return;

    // Load org + members in parallel.
    Promise.all([api.getOrg(orgId), api.listMembers(orgId), api.me()])
      .then(([o, m, me]) => {
        setOrg(o);
        setMembers(m);
        setMyUserId(me.id);
        const myMembership = m.find((mb) => mb.user_id === me.id);
        setMyOrgRole(myMembership?.role ?? null);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) {
          clearToken();
          router.replace('/login');
        } else if (err instanceof ApiError && err.status === 403) {
          router.replace('/dashboard');
        } else {
          setLoadError(err instanceof ApiError ? err.message : 'Failed to load');
        }
      });
  }, [orgId, router]);

  // Roles this user can assign: system/root → all; otherwise use ASSIGNABLE map.
  const assignableRoles =
    systemRole === 'root' || systemRole === 'system'
      ? ['owner', 'admin', 'operational']
      : ASSIGNABLE[myOrgRole ?? ''] ?? [];

  const canManage = assignableRoles.length > 0;

  async function handleSaveName(e: React.FormEvent) {
    e.preventDefault();
    if (!orgId || !editName.trim()) return;
    setSaving(true);
    setEditError('');
    try {
      const updated = await api.updateOrg(orgId, editName.trim());
      setOrg(updated);
      setEditing(false);
    } catch (err) {
      setEditError(err instanceof ApiError ? err.message : 'Failed to save');
    } finally {
      setSaving(false);
    }
  }

  async function handleAddMember(e: React.FormEvent) {
    e.preventDefault();
    if (!orgId || !newUserId) return;
    const uid = parseInt(newUserId, 10);
    if (isNaN(uid)) return;
    setAdding(true);
    setAddError('');
    try {
      const member = await api.addMember(orgId, uid, newRole);
      setMembers((prev) => [...prev, member]);
      setNewUserId('');
    } catch (err) {
      setAddError(err instanceof ApiError ? err.message : 'Failed to add member');
    } finally {
      setAdding(false);
    }
  }

  async function handleUpdateRole(userId: number, role: string) {
    if (!orgId) return;
    try {
      const updated = await api.updateMember(orgId, userId, role);
      setMembers((prev) =>
        prev.map((m) => (m.user_id === userId ? { ...m, role: updated.role } : m)),
      );
      setChangingRole(null);
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Failed to update role');
    }
  }

  async function handleRemove(userId: number) {
    if (!orgId) return;
    if (!confirm(t('orgs.remove') + '?')) return;
    try {
      await api.removeMember(orgId, userId);
      setMembers((prev) => prev.filter((m) => m.user_id !== userId));
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Failed to remove member');
    }
  }

  // Whether this user can manage a specific member (i.e. the target's role is
  // in the assignable set — removing requires the same permission as assigning).
  function canManageMember(memberRole: string): boolean {
    return assignableRoles.includes(memberRole);
  }

  const isSystemOrRoot = systemRole === 'root' || systemRole === 'system';
  const canEditOrg = isSystemOrRoot;
  const canManageBilling = isSystemOrRoot || myOrgRole === 'owner';

  async function handleUpgradeToPro() {
    if (!orgId) return;
    setCheckingOut(true);
    setBillingError('');
    try {
      const { url } = await api.createCheckoutSession(orgId, locale);
      window.location.href = url;
    } catch (err) {
      setBillingError(err instanceof ApiError ? err.message : t('billing.checkoutFailed'));
      setCheckingOut(false);
    }
  }

  async function handleActivateEnterprise() {
    if (!orgId) return;
    setActivatingEnterprise(true);
    setBillingError('');
    try {
      const updated = await api.assignEnterprisePlan(orgId);
      setOrg(updated);
    } catch (err) {
      setBillingError(err instanceof ApiError ? err.message : t('billing.activateEnterpriseFailed'));
    } finally {
      setActivatingEnterprise(false);
    }
  }

  async function handleCancelSubscription() {
    if (cancellingSubscription) return;
    if (!orgId || !confirm(t('billing.cancelConfirm'))) return;
    setCancellingSubscription(true);
    setBillingError('');
    try {
      await api.cancelSubscription(orgId);
    } catch (err) {
      setBillingError(err instanceof ApiError ? err.message : t('billing.cancelSubscriptionFailed'));
    } finally {
      setCancellingSubscription(false);
    }
  }

  if (loadError) {
    return (
      <main className="flex min-h-screen items-center justify-center px-4 py-12">
        <div className="w-full max-w-sm space-y-4">
          <ErrorBanner message={loadError} />
          <p className="text-center">
            <Link href="/dashboard" className="text-sm text-emerald-600 hover:underline">
              ← {t('dashboard.title')}
            </Link>
          </p>
        </div>
      </main>
    );
  }

  if (!org) {
    return (
      <main className="flex min-h-screen items-center justify-center px-4 py-12">
        <p className="text-gray-400">{t('common.loading')}</p>
      </main>
    );
  }

  return (
    <main className="flex min-h-screen flex-col items-center justify-start px-4 py-12 sm:px-6 lg:px-8">
      <div className="w-full max-w-2xl space-y-6">

        {/* Header */}
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            {editing ? (
              <form onSubmit={handleSaveName} className="flex gap-2">
                <input
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  autoFocus
                  className="rounded-md border border-gray-300 px-3 py-1.5 text-base font-bold text-gray-900 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                />
                <button
                  type="submit"
                  disabled={saving}
                  className="rounded-md bg-emerald-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-50"
                >
                  {saving ? t('orgs.saving') : t('orgs.save')}
                </button>
                <button
                  type="button"
                  onClick={() => setEditing(false)}
                  className="rounded-md border border-gray-200 px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-50"
                >
                  {t('orgs.cancel')}
                </button>
              </form>
            ) : (
              <div className="flex items-center gap-3">
                <h1 className="text-xl font-bold text-emerald-700">{org.name}</h1>
                {canEditOrg && (
                  <button
                    onClick={() => { setEditName(org.name); setEditing(true); }}
                    className="rounded-md border border-gray-200 px-2.5 py-1 text-xs text-gray-500 hover:bg-gray-50"
                  >
                    {t('orgs.editOrg')}
                  </button>
                )}
              </div>
            )}
            <p className="mt-0.5 text-xs text-gray-400">#{org.id}</p>
            {editError && <p className="mt-1 text-xs text-red-600">{editError}</p>}
          </div>
          <Link href={isSystemOrRoot ? '/orgs' : '/dashboard'} className="shrink-0 text-sm text-gray-500 hover:underline">
            ←{' '}{isSystemOrRoot ? t('orgs.title') : t('dashboard.title')}
          </Link>
        </div>

        {/* Plan section */}
        <div className="rounded-xl border border-gray-200 bg-white p-4 shadow-sm sm:p-6">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p className="text-xs font-medium uppercase tracking-wide text-gray-500">{t('billing.currentPlan')}</p>
              <div className="mt-1 flex items-center gap-2">
                <PlanBadge plan={org.plan} t={t} />
                <span className="text-xs text-gray-400">
                  {org.plan === 'enterprise' ? t('billing.enterpriseLimits')
                    : org.plan === 'pro' ? t('billing.proLimits')
                    : t('billing.freeLimits')}
                </span>
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              {org.plan !== 'enterprise' && org.plan !== 'pro' && canManageBilling && (
                <button
                  onClick={handleUpgradeToPro}
                  disabled={checkingOut}
                  className="rounded-md bg-blue-600 px-3 py-3 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50 sm:py-2"
                >
                  {checkingOut ? t('common.loading') : t('billing.upgradeToPro')}
                </button>
              )}
              {org.plan !== 'enterprise' && isSystemOrRoot && (
                <button
                  onClick={handleActivateEnterprise}
                  disabled={activatingEnterprise}
                  className="rounded-md bg-purple-600 px-3 py-3 text-sm font-medium text-white hover:bg-purple-700 disabled:opacity-50 sm:py-2"
                >
                  {activatingEnterprise ? t('common.loading') : t('billing.activateEnterprise')}
                </button>
              )}
              {org.plan === 'pro' && canManageBilling && (
                <button
                  onClick={handleCancelSubscription}
                  disabled={cancellingSubscription}
                  className="rounded-md border border-red-200 px-3 py-3 text-sm text-red-600 hover:bg-red-50 disabled:opacity-50 sm:py-2"
                >
                  {cancellingSubscription ? t('common.loading') : t('billing.cancelSubscription')}
                </button>
              )}
            </div>
          </div>
          <ErrorBanner message={billingError} />
        </div>

        {/* Members section */}
        <div className="space-y-3 rounded-xl border border-gray-200 bg-white p-4 shadow-sm sm:p-6">
          <h2 className="font-semibold text-gray-800">{t('orgs.members')}</h2>

          {members.length === 0 ? (
            <p className="text-sm text-gray-400">{t('orgs.noMembers')}</p>
          ) : (
            <ul className="divide-y divide-gray-100">
              {members.map((m) => {
                const isMe = m.user_id === myUserId;
                const manageable = canManageMember(m.role) && !isMe;
                return (
                  <li key={m.id} className="flex items-center justify-between gap-2 py-3">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium text-gray-800">
                        {m.user?.name ?? `User #${m.user_id}`}
                        {isMe && (
                          <span className="ml-2 text-xs text-gray-400">(you)</span>
                        )}
                      </p>
                      <p className="text-xs text-gray-400">{m.user?.email}</p>
                    </div>

                    <div className="flex shrink-0 items-center gap-2">
                      {changingRole === m.user_id ? (
                        <>
                          <select
                            value={pendingRole}
                            onChange={(e) => setPendingRole(e.target.value)}
                            className="rounded-md border border-gray-300 px-2 py-1 text-xs text-gray-900"
                          >
                            {assignableRoles.map((r) => (
                              <option key={r} value={r}>
                                {t(`orgs.role${r.charAt(0).toUpperCase()}${r.slice(1)}`)}
                              </option>
                            ))}
                          </select>
                          <button
                            onClick={() => handleUpdateRole(m.user_id, pendingRole)}
                            className="rounded-md bg-emerald-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-emerald-700"
                          >
                            {t('orgs.save')}
                          </button>
                          <button
                            onClick={() => setChangingRole(null)}
                            className="text-xs text-gray-400 hover:text-gray-600"
                          >
                            {t('orgs.cancel')}
                          </button>
                        </>
                      ) : (
                        <>
                          <RoleBadge role={m.role} t={t} />
                          {manageable && (
                            <>
                              <button
                                onClick={() => {
                                  setPendingRole(assignableRoles[0]);
                                  setChangingRole(m.user_id);
                                }}
                                className="rounded-md border border-gray-200 px-2.5 py-1 text-xs text-gray-600 hover:bg-gray-50"
                              >
                                {t('orgs.updateRole')}
                              </button>
                              <button
                                onClick={() => handleRemove(m.user_id)}
                                className="rounded-md border border-red-200 px-2.5 py-1 text-xs text-red-600 hover:bg-red-50"
                              >
                                {t('orgs.remove')}
                              </button>
                            </>
                          )}
                        </>
                      )}
                    </div>
                  </li>
                );
              })}
            </ul>
          )}

          {/* Add member form */}
          {canManage && (
            <form onSubmit={handleAddMember} className="mt-4 space-y-3 border-t border-gray-100 pt-4">
              <h3 className="text-sm font-medium text-gray-700">{t('orgs.addMember')}</h3>
              <ErrorBanner message={addError} />
              <div className="flex gap-2">
                <input
                  type="number"
                  min={1}
                  value={newUserId}
                  onChange={(e) => setNewUserId(e.target.value)}
                  placeholder={t('orgs.userIdPlaceholder')}
                  required
                  className="min-w-0 flex-1 rounded-md border border-gray-300 px-3 py-2 text-base text-gray-900 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 sm:text-sm"
                />
                <select
                  value={newRole}
                  onChange={(e) => setNewRole(e.target.value)}
                  className="rounded-md border border-gray-300 px-3 py-2 text-base text-gray-900 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 sm:text-sm"
                >
                  {assignableRoles.map((r) => (
                    <option key={r} value={r}>
                      {t(`orgs.role${r.charAt(0).toUpperCase()}${r.slice(1)}`)}
                    </option>
                  ))}
                </select>
                <button
                  type="submit"
                  disabled={adding || !newUserId}
                  className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-50"
                >
                  {adding ? t('orgs.adding') : t('orgs.add')}
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </main>
  );
}

function PlanBadge({ plan, t }: { plan: string; t: ReturnType<typeof useTranslations> }) {
  const colours: Record<string, string> = {
    free: 'bg-gray-100 text-gray-600',
    pro: 'bg-blue-100 text-blue-700',
    enterprise: 'bg-purple-100 text-purple-700',
  };
  const labels: Record<string, string> = {
    free: t('billing.planFree'),
    pro: t('billing.planPro'),
    enterprise: t('billing.planEnterprise'),
  };
  return (
    <span className={`rounded-full px-2.5 py-0.5 text-xs font-semibold ${colours[plan] ?? 'bg-gray-100 text-gray-600'}`}>
      {labels[plan] ?? plan}
    </span>
  );
}

function RoleBadge({ role, t }: { role: string; t: ReturnType<typeof useTranslations> }) {
  const colours: Record<string, string> = {
    owner: 'bg-purple-100 text-purple-700',
    admin: 'bg-blue-100 text-blue-700',
    operational: 'bg-gray-100 text-gray-600',
  };
  const label = t(`orgs.role${role.charAt(0).toUpperCase()}${role.slice(1)}`);
  return (
    <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${colours[role] ?? 'bg-gray-100 text-gray-600'}`}>
      {label}
    </span>
  );
}
