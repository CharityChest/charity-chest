import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, render, screen, fireEvent, waitFor } from '@testing-library/react';
import AdminUsersPage from './page';

const mockRouter = { replace: vi.fn(), push: vi.fn() };

vi.mock('@/i18n/navigation', () => ({
  useRouter: () => mockRouter,
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
    api: { assignSystemRole: vi.fn() },
  };
});

import { isAuthenticated, getRole } from '@/lib/auth';
import { api, ApiError } from '@/lib/api';

beforeEach(() => {
  vi.clearAllMocks();
});

describe('AdminUsersPage — access control', () => {
  it('redirects to /login when not authenticated', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(false);
    vi.mocked(getRole).mockReturnValue(null);

    render(<AdminUsersPage />);

    await waitFor(() => {
      expect(mockRouter.replace).toHaveBeenCalledWith('/login');
    });
  });

  it('redirects to /dashboard when authenticated but not root', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('system');

    render(<AdminUsersPage />);

    await waitFor(() => {
      expect(mockRouter.replace).toHaveBeenCalledWith('/dashboard');
    });
  });

  it('renders the form when authenticated as root', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');

    render(<AdminUsersPage />);

    await waitFor(() => {
      expect(screen.getByText('adminUsers.title')).toBeTruthy();
    });
  });
});

describe('AdminUsersPage — form submission', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
  });

  async function fillAndSubmit(value: string, role?: string) {
    await act(async () => {
      fireEvent.change(screen.getByRole('spinbutton'), { target: { value } });
      if (role) {
        fireEvent.change(screen.getByRole('combobox'), { target: { value: role } });
      }
    });
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'adminUsers.assign' }));
    });
  }

  it('calls assignSystemRole and shows result on success', async () => {
    const user = { id: 5, email: 'u@u.com', name: 'U', role: 'system', created_at: '', updated_at: '' };
    vi.mocked(api.assignSystemRole).mockResolvedValue(user);

    render(<AdminUsersPage />);
    await waitFor(() => expect(screen.getByText('adminUsers.title')).toBeTruthy());

    await fillAndSubmit('5');

    await waitFor(() => {
      expect(api.assignSystemRole).toHaveBeenCalledWith(5, 'system');
      expect(screen.getByText('adminUsers.result')).toBeTruthy();
    });
  });

  it('sends empty string for role="none"', async () => {
    const user = { id: 7, email: 'u@u.com', name: 'U', role: null, created_at: '', updated_at: '' };
    vi.mocked(api.assignSystemRole).mockResolvedValue(user);

    render(<AdminUsersPage />);
    await waitFor(() => expect(screen.getByText('adminUsers.title')).toBeTruthy());

    await fillAndSubmit('7', 'none');

    await waitFor(() => {
      expect(api.assignSystemRole).toHaveBeenCalledWith(7, '');
    });
  });

  it('shows error banner on API failure', async () => {
    vi.mocked(api.assignSystemRole).mockRejectedValue(new ApiError(403, 'forbidden'));

    render(<AdminUsersPage />);
    await waitFor(() => expect(screen.getByText('adminUsers.title')).toBeTruthy());

    await fillAndSubmit('9');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
      expect(screen.getByText('forbidden')).toBeTruthy();
    });
  });

  it('redirects to /login on 401', async () => {
    vi.mocked(api.assignSystemRole).mockRejectedValue(new ApiError(401, 'unauthorized'));

    render(<AdminUsersPage />);
    await waitFor(() => expect(screen.getByText('adminUsers.title')).toBeTruthy());

    await fillAndSubmit('9');

    await waitFor(() => {
      expect(mockRouter.replace).toHaveBeenCalledWith('/login');
    });
  });
});
