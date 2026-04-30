import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import DashboardPage from './page';

const mockReplace = vi.fn();
const mockPush = vi.fn();

vi.mock('@/i18n/navigation', () => ({
  useRouter: () => ({ replace: mockReplace, push: mockPush }),
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
    api: { me: vi.fn(), health: vi.fn() },
  };
});

import { isAuthenticated, getRole, clearToken } from '@/lib/auth';
import { api, ApiError } from '@/lib/api';

const BASE_USER = {
  id: 1,
  email: 'u@u.com',
  name: 'User',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe('DashboardPage — access control', () => {
  it('redirects to /login when not authenticated', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(false);
    vi.mocked(getRole).mockReturnValue(null);

    render(<DashboardPage />);

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith('/login');
    });
  });
});

describe('DashboardPage — role-based quick links', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(api.me).mockResolvedValue(BASE_USER);
  });

  it('shows Manage Orgs and Manage Users links for root', async () => {
    vi.mocked(getRole).mockReturnValue('root');

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText('dashboard.manageOrgs')).toBeTruthy();
      expect(screen.getByText('dashboard.manageUsers')).toBeTruthy();
    });
  });

  it('shows only Manage Orgs link for system', async () => {
    vi.mocked(getRole).mockReturnValue('system');

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText('dashboard.manageOrgs')).toBeTruthy();
      expect(screen.queryByText('dashboard.manageUsers')).toBeNull();
    });
  });

  it('shows the org access form for roleless users', async () => {
    vi.mocked(getRole).mockReturnValue(null);

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText('dashboard.orgAccess')).toBeTruthy();
      expect(screen.queryByText('dashboard.manageOrgs')).toBeNull();
      expect(screen.queryByText('dashboard.manageUsers')).toBeNull();
    });
  });

  it('shows the org access form for org-level users (owner/admin/operational)', async () => {
    vi.mocked(getRole).mockReturnValue(null); // org roles are NOT in JWT

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText('dashboard.orgAccess')).toBeTruthy();
    });
  });

  it('shows user profile info', async () => {
    vi.mocked(getRole).mockReturnValue('root');

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText('u@u.com')).toBeTruthy();
      expect(screen.getByText('User')).toBeTruthy();
    });
  });

  it('shows role name when user has a system role', async () => {
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.me).mockResolvedValue({ ...BASE_USER, role: 'root' });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText('Root')).toBeTruthy();
    });
  });
});

describe('DashboardPage — logout', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.me).mockResolvedValue(BASE_USER);
  });

  it('clears token and pushes to / when logout is clicked', async () => {
    render(<DashboardPage />);

    await waitFor(() => screen.getByText('common.logout'));
    fireEvent.click(screen.getByText('common.logout'));

    expect(clearToken).toHaveBeenCalled();
    expect(mockPush).toHaveBeenCalledWith('/');
  });
});

describe('DashboardPage — org access form', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue(null);
    vi.mocked(api.me).mockResolvedValue(BASE_USER);
  });

  it('pushes to /orgs/:id when a valid org ID is submitted', async () => {
    render(<DashboardPage />);

    await waitFor(() => screen.getByText('dashboard.orgAccess'));

    fireEvent.change(screen.getByPlaceholderText('dashboard.orgIdPlaceholder'), {
      target: { value: '42' },
    });
    fireEvent.click(screen.getByText('dashboard.goToOrg'));

    expect(mockPush).toHaveBeenCalledWith('/orgs/42');
  });
});

describe('DashboardPage — api.me error handling', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue(null);
  });

  it('shows error banner when api.me() fails with a server error', async () => {
    vi.mocked(api.me).mockRejectedValue(new ApiError(500, 'server error'));

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
      expect(screen.getByText('server error')).toBeTruthy();
    });
  });

  it('clears token and redirects to /login on 401 from api.me()', async () => {
    vi.mocked(api.me).mockRejectedValue(new ApiError(401, 'unauthorized'));

    render(<DashboardPage />);

    await waitFor(() => {
      expect(clearToken).toHaveBeenCalled();
      expect(mockReplace).toHaveBeenCalledWith('/login');
    });
  });
});
