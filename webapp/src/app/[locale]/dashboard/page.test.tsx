import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
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
    api: { me: vi.fn() },
  };
});

import { isAuthenticated, getRole } from '@/lib/auth';
import { api } from '@/lib/api';

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
});
