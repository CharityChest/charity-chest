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
    api: {
      assignSystemRole: vi.fn(),
      searchUsers: vi.fn(),
    },
  };
});

import { isAuthenticated, getRole, clearToken } from '@/lib/auth';
import { api, ApiError } from '@/lib/api';

const emptyResult = {
  data: [],
  metadata: { page: 1, size: 20, total: 0, total_pages: 1 },
};

const sampleResult = {
  data: [
    {
      id: 3,
      email: 'alice@example.com',
      name: 'Alice',
      role: 'system',
      mfa_enabled: false,
      created_at: '',
      updated_at: '',
      organizations: [{ id: 1, name: 'Acme', role: 'owner' }],
    },
    {
      id: 7,
      email: 'bob@example.com',
      name: 'Bob',
      role: null,
      mfa_enabled: false,
      created_at: '',
      updated_at: '',
      organizations: [],
    },
  ],
  metadata: { page: 1, size: 20, total: 2, total_pages: 1 },
};

beforeEach(() => {
  vi.resetAllMocks();
});

// ---------------------------------------------------------------------------
// Access control
// ---------------------------------------------------------------------------

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

  it('renders the page when authenticated as root', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');

    render(<AdminUsersPage />);

    await waitFor(() => {
      expect(screen.getByText('adminUsers.title')).toBeTruthy();
    });
  });
});

// ---------------------------------------------------------------------------
// Role assignment form (existing behaviour)
// ---------------------------------------------------------------------------

describe('AdminUsersPage — role assignment', () => {
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
    const user = { id: 5, email: 'u@u.com', name: 'U', role: 'system', mfa_enabled: false, created_at: '', updated_at: '' };
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
    const user = { id: 7, email: 'u@u.com', name: 'U', role: null, mfa_enabled: false, created_at: '', updated_at: '' };
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

// ---------------------------------------------------------------------------
// User search
// ---------------------------------------------------------------------------

describe('AdminUsersPage — user search', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
  });

  async function submitSearch(email = '') {
    await act(async () => {
      const input = screen.getByPlaceholderText('adminUsers.searchEmailPlaceholder');
      fireEvent.change(input, { target: { value: email } });
    });
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'adminUsers.search' }));
    });
  }

  it('renders search input and button when ready', async () => {
    render(<AdminUsersPage />);
    await waitFor(() => expect(screen.getByText('adminUsers.searchSection')).toBeTruthy());
    expect(screen.getByPlaceholderText('adminUsers.searchEmailPlaceholder')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'adminUsers.search' })).toBeTruthy();
  });

  it('calls api.searchUsers with the typed email and page=1', async () => {
    vi.mocked(api.searchUsers).mockResolvedValue(emptyResult);

    render(<AdminUsersPage />);
    await waitFor(() => expect(screen.getByText('adminUsers.searchSection')).toBeTruthy());

    await submitSearch('alice');

    await waitFor(() => {
      expect(api.searchUsers).toHaveBeenCalledWith('alice', 1, 20);
    });
  });

  it('renders table rows with id, email, role and org names', async () => {
    vi.mocked(api.searchUsers).mockResolvedValue(sampleResult);

    render(<AdminUsersPage />);
    await waitFor(() => expect(screen.getByText('adminUsers.searchSection')).toBeTruthy());

    await submitSearch();

    await waitFor(() => {
      expect(screen.getByText('alice@example.com')).toBeTruthy();
      expect(screen.getByText('bob@example.com')).toBeTruthy();
      expect(screen.getByText('Acme (owner)')).toBeTruthy();
    });
  });

  it('shows empty-state message when results are empty', async () => {
    vi.mocked(api.searchUsers).mockResolvedValue(emptyResult);

    render(<AdminUsersPage />);
    await waitFor(() => expect(screen.getByText('adminUsers.searchSection')).toBeTruthy());

    await submitSearch('nobody');

    await waitFor(() => {
      expect(screen.getByText('adminUsers.noResults')).toBeTruthy();
    });
  });

  it('advances to page 2 when Next is clicked', async () => {
    const page1 = {
      data: sampleResult.data,
      metadata: { page: 1, size: 20, total: 40, total_pages: 2 },
    };
    const page2 = {
      data: [{ ...sampleResult.data[0], id: 99, email: 'carol@example.com' }],
      metadata: { page: 2, size: 20, total: 40, total_pages: 2 },
    };
    vi.mocked(api.searchUsers).mockResolvedValueOnce(page1).mockResolvedValueOnce(page2);

    render(<AdminUsersPage />);
    await waitFor(() => expect(screen.getByText('adminUsers.searchSection')).toBeTruthy());
    await submitSearch();
    await waitFor(() => expect(screen.getByText('alice@example.com')).toBeTruthy());

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /adminUsers\.nextPage/ }));
    });

    await waitFor(() => {
      expect(api.searchUsers).toHaveBeenCalledWith('', 2, 20);
      expect(screen.getByText('carol@example.com')).toBeTruthy();
    });
  });

  it('steps back to page 1 when Previous is clicked', async () => {
    const page1 = {
      data: sampleResult.data,
      metadata: { page: 1, size: 20, total: 40, total_pages: 2 },
    };
    const page2 = {
      data: [{ ...sampleResult.data[0], id: 99, email: 'carol@example.com', organizations: [] }],
      metadata: { page: 2, size: 20, total: 40, total_pages: 2 },
    };

    // Initial search → page 1
    vi.mocked(api.searchUsers).mockResolvedValueOnce(page1);
    render(<AdminUsersPage />);
    await waitFor(() => expect(screen.getByText('adminUsers.searchSection')).toBeTruthy());
    await submitSearch();
    await waitFor(() => expect(screen.getByText('alice@example.com')).toBeTruthy());

    // Click Next → page 2
    vi.mocked(api.searchUsers).mockResolvedValueOnce(page2);
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /adminUsers\.nextPage/ }));
    });
    await waitFor(() => {
      expect(api.searchUsers).toHaveBeenCalledWith('', 2, 20);
      expect(screen.getByText('carol@example.com')).toBeTruthy();
    });

    // Click Prev → back to page 1
    vi.mocked(api.searchUsers).mockResolvedValueOnce(page1);
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /adminUsers\.prevPage/ }));
    });
    await waitFor(() => {
      expect(api.searchUsers).toHaveBeenCalledWith('', 1, 20);
      expect(screen.getByText('alice@example.com')).toBeTruthy();
    });
  });

  it('redirects to /login on 401 during search', async () => {
    vi.mocked(api.searchUsers).mockRejectedValue(new ApiError(401, 'unauthorized'));

    render(<AdminUsersPage />);
    await waitFor(() => expect(screen.getByText('adminUsers.searchSection')).toBeTruthy());

    await submitSearch('test');

    await waitFor(() => {
      expect(clearToken).toHaveBeenCalled();
      expect(mockRouter.replace).toHaveBeenCalledWith('/login');
    });
  });

  it('shows error banner on search failure', async () => {
    vi.mocked(api.searchUsers).mockRejectedValue(new ApiError(500, 'server error'));

    render(<AdminUsersPage />);
    await waitFor(() => expect(screen.getByText('adminUsers.searchSection')).toBeTruthy());

    await submitSearch('test');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
      expect(screen.getByText('server error')).toBeTruthy();
    });
  });
});
