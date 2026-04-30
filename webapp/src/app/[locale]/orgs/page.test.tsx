import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, render, screen, fireEvent, waitFor } from '@testing-library/react';
import OrgsPage from './page';

const mockRouter = { replace: vi.fn() };

vi.mock('@/i18n/navigation', () => ({
  // Stable object reference — prevents useEffect([router]) from re-running on every render.
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
      listOrgs: vi.fn(),
      createOrg: vi.fn(),
      deleteOrg: vi.fn(),
    },
  };
});

import { isAuthenticated, getRole, clearToken } from '@/lib/auth';
import { api, ApiError } from '@/lib/api';

const baseOrgs = [
  { id: 1, name: 'Alpha', created_at: '', updated_at: '' },
  { id: 2, name: 'Beta', created_at: '', updated_at: '' },
];

beforeEach(() => {
  vi.clearAllMocks();
});

describe('OrgsPage — access control', () => {
  it('redirects to /login when not authenticated', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(false);
    vi.mocked(getRole).mockReturnValue(null);

    render(<OrgsPage />);

    await waitFor(() => {
      expect(mockRouter.replace).toHaveBeenCalledWith('/login');
    });
  });

  it('redirects to /dashboard when authenticated but role is not system/root', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue(null);
    vi.mocked(api.listOrgs).mockResolvedValue([]);

    render(<OrgsPage />);

    await waitFor(() => {
      expect(mockRouter.replace).toHaveBeenCalledWith('/dashboard');
    });
  });

  it('renders the page when authenticated as system', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('system');
    vi.mocked(api.listOrgs).mockResolvedValue(baseOrgs);

    render(<OrgsPage />);

    await waitFor(() => {
      expect(screen.getByText('orgs.title')).toBeTruthy();
    });
  });

  it('renders the page when authenticated as root', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.listOrgs).mockResolvedValue(baseOrgs);

    render(<OrgsPage />);

    await waitFor(() => {
      expect(screen.getByText('orgs.title')).toBeTruthy();
    });
  });
});

describe('OrgsPage — org list', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
  });

  it('renders a list of organisations', async () => {
    vi.mocked(api.listOrgs).mockResolvedValue(baseOrgs);

    render(<OrgsPage />);

    await waitFor(() => {
      expect(screen.getByText('Alpha')).toBeTruthy();
      expect(screen.getByText('Beta')).toBeTruthy();
    });
  });

  it('shows the empty-state message when there are no orgs', async () => {
    vi.mocked(api.listOrgs).mockResolvedValue([]);

    render(<OrgsPage />);

    await waitFor(() => {
      expect(screen.getByText('orgs.noOrgs')).toBeTruthy();
    });
  });
});

describe('OrgsPage — create org', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.listOrgs).mockResolvedValue([]);
  });

  async function typeAndSubmit(value: string) {
    await act(async () => {
      fireEvent.change(
        screen.getByPlaceholderText('orgs.orgNamePlaceholder'),
        { target: { value } },
      );
    });
    // Click the submit button (now enabled after state flush).
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'orgs.create' }));
    });
  }

  it('adds the new org to the list on successful create', async () => {
    const newOrg = { id: 3, name: 'Gamma', created_at: '', updated_at: '' };
    vi.mocked(api.createOrg).mockResolvedValue(newOrg);

    render(<OrgsPage />);
    await waitFor(() => expect(screen.getByText('orgs.title')).toBeTruthy());

    await typeAndSubmit('Gamma');

    await waitFor(() => {
      expect(api.createOrg).toHaveBeenCalledWith('Gamma');
      expect(screen.getByText('Gamma')).toBeTruthy();
    });
  });

  it('shows an error banner when create fails', async () => {
    vi.mocked(api.createOrg).mockRejectedValue(new ApiError(500, 'server error'));

    render(<OrgsPage />);
    await waitFor(() => expect(screen.getByText('orgs.title')).toBeTruthy());

    await typeAndSubmit('Bad');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
      expect(screen.getByText('server error')).toBeTruthy();
    });
  });
});

describe('OrgsPage — delete org', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
    vi.mocked(api.listOrgs).mockResolvedValue(baseOrgs);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('removes the org from the list when confirmed', async () => {
    vi.stubGlobal('confirm', vi.fn(() => true));
    vi.mocked(api.deleteOrg).mockResolvedValue(undefined);

    render(<OrgsPage />);
    await waitFor(() => screen.getByText('Alpha'));

    fireEvent.click(screen.getAllByText('orgs.deleteOrg')[0]);

    await waitFor(() => {
      expect(api.deleteOrg).toHaveBeenCalledWith(1);
      expect(screen.queryByText('Alpha')).toBeNull();
    });
  });

  it('does not remove the org when the confirm dialog is cancelled', async () => {
    vi.stubGlobal('confirm', vi.fn(() => false));

    render(<OrgsPage />);
    await waitFor(() => screen.getByText('Alpha'));

    fireEvent.click(screen.getAllByText('orgs.deleteOrg')[0]);

    expect(api.deleteOrg).not.toHaveBeenCalled();
    expect(screen.getByText('Alpha')).toBeTruthy();
  });
});

describe('OrgsPage — listOrgs error handling', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
  });

  it('clears token and redirects to /login on 401 from listOrgs', async () => {
    vi.mocked(api.listOrgs).mockRejectedValue(new ApiError(401, 'unauthorized'));

    render(<OrgsPage />);

    await waitFor(() => {
      expect(clearToken).toHaveBeenCalled();
      expect(mockRouter.replace).toHaveBeenCalledWith('/login');
    });
  });

  it('shows a load error banner when listOrgs fails with a non-401 error', async () => {
    vi.mocked(api.listOrgs).mockRejectedValue(new ApiError(500, 'internal error'));

    render(<OrgsPage />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
      expect(screen.getByText('internal error')).toBeTruthy();
    });
  });
});
