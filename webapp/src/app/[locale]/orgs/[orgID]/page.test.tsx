import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import OrgDetailPage from './page';

const mockReplace = vi.fn();

vi.mock('@/i18n/navigation', () => ({
  useRouter: () => ({ replace: mockReplace }),
  Link: ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock('next-intl', () => ({
  useTranslations: () => (key: string) => key,
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
    },
  };
});

import { isAuthenticated, getRole } from '@/lib/auth';
import { api, ApiError } from '@/lib/api';

const ORG = { id: 1, name: 'Test Org', created_at: '', updated_at: '' };
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
