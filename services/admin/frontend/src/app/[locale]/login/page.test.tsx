import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import LoginPage from './page';

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
  useLocale: () => 'en',
}));

vi.mock('@/lib/auth', () => ({
  isAuthenticated: vi.fn(),
  setToken: vi.fn(),
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
      login: vi.fn(),
      mfaVerify: vi.fn(),
      googleAuthUrl: vi.fn(() => 'http://api/v1/auth/google?locale=en'),
    },
  };
});

import { isAuthenticated, setToken } from '@/lib/auth';
import { api, ApiError } from '@/lib/api';

beforeEach(() => {
  vi.clearAllMocks();
});

describe('LoginPage — access control', () => {
  it('redirects to /dashboard when already authenticated', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    render(<LoginPage />);
    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith('/dashboard');
    });
  });

  it('renders the credentials form when not authenticated', () => {
    vi.mocked(isAuthenticated).mockReturnValue(false);
    render(<LoginPage />);
    expect(screen.getByLabelText('login.email')).toBeTruthy();
    expect(screen.getByLabelText('login.password')).toBeTruthy();
  });
});

describe('LoginPage — credentials step', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(false);
  });

  it('stores token and pushes to /dashboard on successful login', async () => {
    vi.mocked(api.login).mockResolvedValue({ token: 'my-jwt' });
    render(<LoginPage />);

    fireEvent.change(screen.getByLabelText('login.email'), { target: { value: 'u@u.com' } });
    fireEvent.change(screen.getByLabelText('login.password'), { target: { value: 'pass123' } });
    fireEvent.click(screen.getByText('login.submit'));

    await waitFor(() => {
      expect(setToken).toHaveBeenCalledWith('my-jwt');
      expect(mockPush).toHaveBeenCalledWith('/dashboard');
    });
  });

  it('shows MFA step when login returns mfa_required', async () => {
    vi.mocked(api.login).mockResolvedValue({ mfa_required: true, mfa_token: 'mfa-pending' });
    render(<LoginPage />);

    fireEvent.change(screen.getByLabelText('login.email'), { target: { value: 'u@u.com' } });
    fireEvent.change(screen.getByLabelText('login.password'), { target: { value: 'pass' } });
    fireEvent.click(screen.getByText('login.submit'));

    await waitFor(() => {
      expect(screen.getByLabelText('login.mfaStep')).toBeTruthy();
    });
  });

  it('shows error banner on failed login', async () => {
    vi.mocked(api.login).mockRejectedValue(new ApiError(401, 'invalid credentials'));
    render(<LoginPage />);

    fireEvent.change(screen.getByLabelText('login.email'), { target: { value: 'u@u.com' } });
    fireEvent.change(screen.getByLabelText('login.password'), { target: { value: 'wrong' } });
    fireEvent.click(screen.getByText('login.submit'));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
      expect(screen.getByText('invalid credentials')).toBeTruthy();
    });
  });
});

describe('LoginPage — MFA step', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(false);
    vi.mocked(api.login).mockResolvedValue({ mfa_required: true, mfa_token: 'mfa-tok' });
  });

  async function reachMfaStep() {
    render(<LoginPage />);
    fireEvent.change(screen.getByLabelText('login.email'), { target: { value: 'u@u.com' } });
    fireEvent.change(screen.getByLabelText('login.password'), { target: { value: 'pass' } });
    fireEvent.click(screen.getByText('login.submit'));
    await waitFor(() => screen.getByLabelText('login.mfaStep'));
  }

  it('stores token and pushes to /dashboard after successful MFA', async () => {
    vi.mocked(api.mfaVerify).mockResolvedValue({ token: 'full-jwt' });
    await reachMfaStep();

    fireEvent.change(screen.getByLabelText('login.mfaStep'), { target: { value: '123456' } });
    fireEvent.click(screen.getByText('login.verify'));

    await waitFor(() => {
      expect(api.mfaVerify).toHaveBeenCalledWith('mfa-tok', '123456');
      expect(setToken).toHaveBeenCalledWith('full-jwt');
      expect(mockPush).toHaveBeenCalledWith('/dashboard');
    });
  });

  it('shows error banner on failed MFA verification', async () => {
    vi.mocked(api.mfaVerify).mockRejectedValue(new ApiError(401, 'invalid code'));
    await reachMfaStep();

    fireEvent.change(screen.getByLabelText('login.mfaStep'), { target: { value: '999999' } });
    fireEvent.click(screen.getByText('login.verify'));

    await waitFor(() => {
      expect(screen.getByText('invalid code')).toBeTruthy();
    });
  });

  it('returns to credentials form when back button is clicked', async () => {
    await reachMfaStep();
    fireEvent.click(screen.getByText('login.backToLogin'));
    expect(screen.getByLabelText('login.email')).toBeTruthy();
  });
});
