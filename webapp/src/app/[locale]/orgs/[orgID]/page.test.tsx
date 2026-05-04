import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import OrgDetailPage from './page';

const mockReplace = vi.fn();
// Stable object reference — prevents router identity change on every render,
// which would cause the load useEffect to re-run and reset state.
const mockRouter = { replace: mockReplace };

vi.mock('@/i18n/navigation', () => ({
  useRouter: () => mockRouter,
  Link: ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock('next-intl', () => ({
  useTranslations: () => (key: string) => key,
  useLocale: () => 'en',
}));

vi.mock('@/lib/auth', () => ({
  isAuthenticated: vi.fn(),
  getRole: vi.fn(),
  clearToken: vi.fn(),
}));

vi.mock('@/lib/api', () => {
  class ApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.name = 'ApiError';
      this.status = status;
    }
  }
  return {
    ApiError,
    api: {
      getOrg: vi.fn(),
      listMembers: vi.fn(),
      me: vi.fn(),
      addMember: vi.fn(),
      updateMember: vi.fn(),
      removeMember: vi.fn(),
      updateOrg: vi.fn(),
      createCheckoutSession: vi.fn(),
      cancelSubscription: vi.fn(),
      assignEnterprisePlan: vi.fn(),
    },
  };
});

import { isAuthenticated, getRole } from '@/lib/auth';
import { api, ApiError } from '@/lib/api';

const ORG = { id: 1, name: 'Test Org', plan: 'free' as const, created_at: '', updated_at: '' };
const ME = { id: 10, email: 'me@test.com', name: 'Me', created_at: '', updated_at: '' };

function makeParams(orgID = '1') {
  return Promise.resolve({ orgID });
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('OrgDetailPage — access control', () => {
  it('redirects to /login when not authenticated', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(false);
    vi.mocked(getRole).mockReturnValue(null);

    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith('/login');
    });
  });

  it('redirects to /dashboard on 403 response', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue(null);
    vi.mocked(api.getOrg).mockRejectedValue(new ApiError(403, 'forbidden'));
    vi.mocked(api.listMembers).mockResolvedValue([]);
    vi.mocked(api.me).mockResolvedValue(ME);

    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith('/dashboard');
    });
  });
});

describe('OrgDetailPage — org display', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.getOrg).mockResolvedValue(ORG);
    vi.mocked(api.me).mockResolvedValue(ME);
  });

  it('renders the org name once loaded', async () => {
    vi.mocked(api.listMembers).mockResolvedValue([]);

    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => {
      expect(screen.getByText('Test Org')).toBeTruthy();
    });
  });

  it('renders member names in the list', async () => {
    const members = [
      {
        id: 1, org_id: 1, user_id: 10, role: 'owner', created_at: '', updated_at: '',
        user: { id: 10, email: 'me@test.com', name: 'Me', created_at: '', updated_at: '' },
      },
      {
        id: 2, org_id: 1, user_id: 20, role: 'admin', created_at: '', updated_at: '',
        user: { id: 20, email: 'other@test.com', name: 'Other', created_at: '', updated_at: '' },
      },
    ];
    vi.mocked(api.listMembers).mockResolvedValue(members);

    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => {
      expect(screen.getByText('Me')).toBeTruthy();
      expect(screen.getByText('Other')).toBeTruthy();
    });
  });

  it('shows noMembers message when member list is empty', async () => {
    vi.mocked(api.listMembers).mockResolvedValue([]);

    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => {
      expect(screen.getByText('orgs.noMembers')).toBeTruthy();
    });
  });
});

describe('OrgDetailPage — role-based actions', () => {
  it('shows edit button for root users', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.getOrg).mockResolvedValue(ORG);
    vi.mocked(api.listMembers).mockResolvedValue([]);
    vi.mocked(api.me).mockResolvedValue(ME);

    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => {
      expect(screen.getByText('orgs.editOrg')).toBeTruthy();
    });
  });

  it('does not show edit button for org-level members', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue(null); // no system role
    vi.mocked(api.getOrg).mockResolvedValue(ORG);
    vi.mocked(api.me).mockResolvedValue(ME);
    vi.mocked(api.listMembers).mockResolvedValue([
      {
        id: 1, org_id: 1, user_id: 10, role: 'owner', created_at: '', updated_at: '',
        user: { id: 10, email: 'me@test.com', name: 'Me', created_at: '', updated_at: '' },
      },
    ]);

    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => {
      expect(screen.queryByText('orgs.editOrg')).toBeNull();
    });
  });
});

describe('OrgDetailPage — edit org name', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.getOrg).mockResolvedValue(ORG);
    vi.mocked(api.listMembers).mockResolvedValue([]);
    vi.mocked(api.me).mockResolvedValue(ME);
  });

  it('shows edit form when edit button is clicked', async () => {
    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => screen.getByText('orgs.editOrg'));
    fireEvent.click(screen.getByText('orgs.editOrg'));

    await waitFor(() => {
      expect(screen.getByText('orgs.save')).toBeTruthy();
      expect(screen.getByText('orgs.cancel')).toBeTruthy();
    });
  });

  it('calls api.updateOrg and hides form on successful save', async () => {
    const updatedOrg = { id: 1, name: 'Updated Name', created_at: '', updated_at: '' };
    vi.mocked(api.updateOrg).mockResolvedValue(updatedOrg);

    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => screen.getByText('orgs.editOrg'));
    fireEvent.click(screen.getByText('orgs.editOrg'));

    await waitFor(() => screen.getByText('orgs.save'));

    const input = screen.getByDisplayValue('Test Org');
    fireEvent.change(input, { target: { value: 'Updated Name' } });
    fireEvent.click(screen.getByText('orgs.save'));

    await waitFor(() => {
      expect(api.updateOrg).toHaveBeenCalledWith(1, 'Updated Name');
      expect(screen.getByText('Updated Name')).toBeTruthy();
    });
  });

  it('hides the form when cancel is clicked', async () => {
    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => screen.getByText('orgs.editOrg'));
    fireEvent.click(screen.getByText('orgs.editOrg'));

    await waitFor(() => screen.getByText('orgs.cancel'));
    fireEvent.click(screen.getByText('orgs.cancel'));

    await waitFor(() => {
      expect(screen.getByText('orgs.editOrg')).toBeTruthy();
    });
  });
});

describe('OrgDetailPage — add member', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.getOrg).mockResolvedValue(ORG);
    vi.mocked(api.listMembers).mockResolvedValue([]);
    vi.mocked(api.me).mockResolvedValue(ME);
  });

  it('shows the add member form for root users', async () => {
    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => {
      expect(screen.getByText('orgs.addMember')).toBeTruthy();
      expect(screen.getByPlaceholderText('orgs.userIdPlaceholder')).toBeTruthy();
    });
  });

  it('calls api.addMember and appends to the list on success', async () => {
    const newMember = {
      id: 5, org_id: 1, user_id: 99, role: 'operational', created_at: '', updated_at: '',
      user: { id: 99, email: 'new@test.com', name: 'New Member', created_at: '', updated_at: '' },
    };
    vi.mocked(api.addMember).mockResolvedValue(newMember);

    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => screen.getByPlaceholderText('orgs.userIdPlaceholder'));

    fireEvent.change(screen.getByPlaceholderText('orgs.userIdPlaceholder'), {
      target: { value: '99' },
    });
    fireEvent.click(screen.getByText('orgs.add'));

    await waitFor(() => {
      expect(api.addMember).toHaveBeenCalledWith(1, 99, 'operational');
      expect(screen.getByText('New Member')).toBeTruthy();
    });
  });

  it('shows an error banner when adding a member fails', async () => {
    vi.mocked(api.addMember).mockRejectedValue(new ApiError(422, 'user not found'));

    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => screen.getByPlaceholderText('orgs.userIdPlaceholder'));

    fireEvent.change(screen.getByPlaceholderText('orgs.userIdPlaceholder'), {
      target: { value: '999' },
    });
    fireEvent.click(screen.getByText('orgs.add'));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
      expect(screen.getByText('user not found')).toBeTruthy();
    });
  });
});

describe('OrgDetailPage — member management', () => {
  const MEMBERS = [
    {
      id: 2, org_id: 1, user_id: 20, role: 'admin', created_at: '', updated_at: '',
      user: { id: 20, email: 'other@test.com', name: 'Other', created_at: '', updated_at: '' },
    },
  ];

  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.getOrg).mockResolvedValue(ORG);
    vi.mocked(api.listMembers).mockResolvedValue(MEMBERS);
    vi.mocked(api.me).mockResolvedValue(ME);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('shows Update Role and Remove buttons for manageable members', async () => {
    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => {
      expect(screen.getByText('orgs.updateRole')).toBeTruthy();
      expect(screen.getByText('orgs.remove')).toBeTruthy();
    });
  });

  it('shows a role selector when Update Role is clicked', async () => {
    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => screen.getByText('orgs.updateRole'));
    fireEvent.click(screen.getByText('orgs.updateRole'));

    await waitFor(() => {
      // The role change row replaces "Update Role" + "Remove" with a select + Save + Cancel.
      expect(screen.getByText('orgs.cancel')).toBeTruthy();
      expect(screen.queryByText('orgs.updateRole')).toBeNull();
    });
  });

  it('calls api.updateMember and closes selector on save', async () => {
    vi.mocked(api.updateMember).mockResolvedValue({ ...MEMBERS[0], role: 'owner' });

    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => screen.getByText('orgs.updateRole'));
    fireEvent.click(screen.getByText('orgs.updateRole'));

    // Wait for the role change row to appear (Cancel button is unique to this state).
    await waitFor(() => screen.getByText('orgs.cancel'));

    // Click Save in the role change row (orgs.save only appears there in this context).
    fireEvent.click(screen.getByText('orgs.save'));

    await waitFor(() => {
      expect(api.updateMember).toHaveBeenCalledWith(1, 20, expect.any(String));
    });
  });

  it('calls api.removeMember when Remove is clicked and confirmed', async () => {
    vi.stubGlobal('confirm', vi.fn(() => true));
    vi.stubGlobal('alert', vi.fn());
    vi.mocked(api.removeMember).mockResolvedValue(undefined);

    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => screen.getByText('orgs.remove'));
    fireEvent.click(screen.getByText('orgs.remove'));

    await waitFor(() => {
      expect(api.removeMember).toHaveBeenCalledWith(1, 20);
    });
  });

  it('does not call api.removeMember when Remove is cancelled', async () => {
    vi.stubGlobal('confirm', vi.fn(() => false));

    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => screen.getByText('orgs.remove'));
    fireEvent.click(screen.getByText('orgs.remove'));

    expect(api.removeMember).not.toHaveBeenCalled();
  });
});

describe('OrgDetailPage — load error state', () => {
  it('shows an error banner when loading the org fails', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.getOrg).mockRejectedValue(new ApiError(500, 'load failed'));
    vi.mocked(api.listMembers).mockResolvedValue([]);
    vi.mocked(api.me).mockResolvedValue(ME);

    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
      expect(screen.getByText('load failed')).toBeTruthy();
    });
  });
});

describe('OrgDetailPage — plan badge', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.listMembers).mockResolvedValue([]);
    vi.mocked(api.me).mockResolvedValue(ME);
  });

  it('shows billing.planFree badge for free org', async () => {
    vi.mocked(api.getOrg).mockResolvedValue({ ...ORG, plan: 'free' });
    render(<OrgDetailPage params={makeParams()} />);
    await waitFor(() => {
      expect(screen.getByText('billing.planFree')).toBeTruthy();
    });
  });

  it('shows billing.planPro badge for pro org', async () => {
    vi.mocked(api.getOrg).mockResolvedValue({ ...ORG, plan: 'pro' });
    render(<OrgDetailPage params={makeParams()} />);
    await waitFor(() => {
      expect(screen.getByText('billing.planPro')).toBeTruthy();
    });
  });

  it('shows billing.planEnterprise badge for enterprise org', async () => {
    vi.mocked(api.getOrg).mockResolvedValue({ ...ORG, plan: 'enterprise' });
    render(<OrgDetailPage params={makeParams()} />);
    await waitFor(() => {
      expect(screen.getByText('billing.planEnterprise')).toBeTruthy();
    });
  });
});

describe('OrgDetailPage — billing actions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(api.getOrg).mockResolvedValue({ ...ORG, plan: 'free' });
    vi.mocked(api.listMembers).mockResolvedValue([]);
    vi.mocked(api.me).mockResolvedValue(ME);
  });

  it('shows Upgrade to Pro for root on free org', async () => {
    vi.mocked(getRole).mockReturnValue('root');
    render(<OrgDetailPage params={makeParams()} />);
    await waitFor(() => {
      expect(screen.getByText('billing.upgradeToPro')).toBeTruthy();
    });
  });

  it('shows Upgrade to Pro for org owner (no system role)', async () => {
    vi.mocked(getRole).mockReturnValue(null);
    vi.mocked(api.listMembers).mockResolvedValue([
      { id: 1, org_id: 1, user_id: 10, role: 'owner', created_at: '', updated_at: '',
        user: { id: 10, email: 'me@test.com', name: 'Me', created_at: '', updated_at: '' } },
    ]);
    render(<OrgDetailPage params={makeParams()} />);
    await waitFor(() => {
      expect(screen.getByText('billing.upgradeToPro')).toBeTruthy();
    });
  });

  it('hides Upgrade to Pro for operational member', async () => {
    vi.mocked(getRole).mockReturnValue(null);
    vi.mocked(api.listMembers).mockResolvedValue([
      { id: 1, org_id: 1, user_id: 10, role: 'operational', created_at: '', updated_at: '',
        user: { id: 10, email: 'me@test.com', name: 'Me', created_at: '', updated_at: '' } },
    ]);
    render(<OrgDetailPage params={makeParams()} />);
    await waitFor(() => {
      expect(screen.queryByText('billing.upgradeToPro')).toBeNull();
    });
  });

  it('shows Activate Enterprise for root only', async () => {
    vi.mocked(getRole).mockReturnValue('root');
    render(<OrgDetailPage params={makeParams()} />);
    await waitFor(() => {
      expect(screen.getByText('billing.activateEnterprise')).toBeTruthy();
    });
  });

  it('hides Activate Enterprise for non-system user', async () => {
    vi.mocked(getRole).mockReturnValue(null);
    render(<OrgDetailPage params={makeParams()} />);
    await waitFor(() => {
      expect(screen.queryByText('billing.activateEnterprise')).toBeNull();
    });
  });

  it('calls assignEnterprisePlan and updates plan badge', async () => {
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.assignEnterprisePlan).mockResolvedValue({ ...ORG, plan: 'enterprise' });
    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => screen.getByText('billing.activateEnterprise'));
    fireEvent.click(screen.getByText('billing.activateEnterprise'));

    await waitFor(() => {
      expect(api.assignEnterprisePlan).toHaveBeenCalledWith(1);
      expect(screen.getByText('billing.planEnterprise')).toBeTruthy();
    });
  });

  it('shows Cancel subscription for root on pro org', async () => {
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.getOrg).mockResolvedValue({ ...ORG, plan: 'pro' });
    render(<OrgDetailPage params={makeParams()} />);
    await waitFor(() => {
      expect(screen.getByText('billing.cancelSubscription')).toBeTruthy();
    });
  });

  it('calls cancelSubscription when Cancel is confirmed', async () => {
    vi.stubGlobal('confirm', vi.fn(() => true));
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.getOrg).mockResolvedValue({ ...ORG, plan: 'pro' });
    vi.mocked(api.cancelSubscription).mockResolvedValue(undefined);
    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => screen.getByText('billing.cancelSubscription'));
    fireEvent.click(screen.getByText('billing.cancelSubscription'));

    await waitFor(() => {
      expect(api.cancelSubscription).toHaveBeenCalledWith(1);
    });
    vi.unstubAllGlobals();
  });

  it('calls createCheckoutSession and redirects on Upgrade click', async () => {
    vi.stubGlobal('location', { href: '' });
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.createCheckoutSession).mockResolvedValue({ url: 'https://checkout.stripe.com/test' });
    render(<OrgDetailPage params={makeParams()} />);

    await waitFor(() => screen.getByText('billing.upgradeToPro'));
    fireEvent.click(screen.getByText('billing.upgradeToPro'));

    await waitFor(() => {
      expect(api.createCheckoutSession).toHaveBeenCalledWith(1, 'en');
      expect(window.location.href).toBe('https://checkout.stripe.com/test');
    });
    vi.unstubAllGlobals();
  });
});
