import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import ProfilePage from './page';

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
  clearToken: vi.fn(),
}));

vi.mock('qrcode.react', () => ({
  QRCodeSVG: ({ value }: { value: string }) => <div data-testid="qr-code" data-value={value} />,
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
      me: vi.fn(),
      mfaSetup: vi.fn(),
      mfaEnable: vi.fn(),
      mfaDisable: vi.fn(),
    },
  };
});

import { isAuthenticated } from '@/lib/auth';
import { api } from '@/lib/api';

const BASE_USER = {
  id: 1,
  email: 'u@u.com',
  name: 'User',
  mfa_enabled: false,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
};

beforeEach(() => {
  vi.resetAllMocks();
});

describe('ProfilePage — access control', () => {
  it('redirects to /login when not authenticated', async () => {
    vi.mocked(isAuthenticated).mockReturnValue(false);

    render(<ProfilePage />);

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith('/login');
    });
  });
});

describe('ProfilePage — MFA disabled state', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(api.me).mockResolvedValue({ ...BASE_USER, mfa_enabled: false });
  });

  it('shows disabled state and Enable MFA button', async () => {
    render(<ProfilePage />);

    await waitFor(() => {
      expect(screen.getByText('profile.mfaDisabled')).toBeTruthy();
      expect(screen.getByText('profile.enableMFA')).toBeTruthy();
    });
  });

  it('shows QR code after clicking Enable MFA', async () => {
    vi.mocked(api.mfaSetup).mockResolvedValue({
      uri: 'otpauth://totp/test?secret=JBSWY3DPEHPK3PXP',
      secret: 'JBSWY3DPEHPK3PXP',
    });

    render(<ProfilePage />);

    await waitFor(() => screen.getByText('profile.enableMFA'));
    fireEvent.click(screen.getByText('profile.enableMFA'));

    await waitFor(() => {
      expect(screen.getByTestId('qr-code')).toBeTruthy();
      expect(screen.getByText('JBSWY3DPEHPK3PXP')).toBeTruthy();
      expect(screen.getByText('profile.verifyEnable')).toBeTruthy();
    });
  });

  it('shows error when setup API call fails', async () => {
    const { ApiError } = await import('@/lib/api');
    vi.mocked(api.mfaSetup).mockRejectedValue(new ApiError(500, 'Setup error'));

    render(<ProfilePage />);

    await waitFor(() => screen.getByText('profile.enableMFA'));
    fireEvent.click(screen.getByText('profile.enableMFA'));

    await waitFor(() => {
      expect(screen.getByText('Setup error')).toBeTruthy();
    });
  });
});

describe('ProfilePage — MFA enabled state', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(api.me).mockResolvedValue({ ...BASE_USER, mfa_enabled: true });
  });

  it('shows enabled badge and Disable MFA button', async () => {
    render(<ProfilePage />);

    await waitFor(() => {
      expect(screen.getByText('profile.mfaEnabled')).toBeTruthy();
      expect(screen.getByText('profile.disableMFA')).toBeTruthy();
    });
  });

  it('shows code input after clicking Disable MFA', async () => {
    render(<ProfilePage />);

    await waitFor(() => screen.getByText('profile.disableMFA'));
    fireEvent.click(screen.getByText('profile.disableMFA'));

    await waitFor(() => {
      expect(screen.getByText('profile.verifyDisable')).toBeTruthy();
    });
  });

  it('shows error when disable API call fails', async () => {
    const { ApiError } = await import('@/lib/api');
    vi.mocked(api.mfaDisable).mockRejectedValue(new ApiError(401, 'Invalid code'));

    render(<ProfilePage />);

    await waitFor(() => screen.getByText('profile.disableMFA'));
    fireEvent.click(screen.getByText('profile.disableMFA'));

    await waitFor(() => screen.getByText('profile.verifyDisable'));

    // Type a 6-digit code and submit.
    const input = screen.getByPlaceholderText('profile.codePlaceholder');
    fireEvent.change(input, { target: { value: '123456' } });
    fireEvent.click(screen.getByText('profile.verifyDisable'));

    await waitFor(() => {
      expect(screen.getByText('Invalid code')).toBeTruthy();
    });
  });
});

describe('ProfilePage — enable MFA flow', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(api.me).mockResolvedValue({ ...BASE_USER, mfa_enabled: false });
    vi.mocked(api.mfaSetup).mockResolvedValue({
      uri: 'otpauth://totp/test?secret=ABC',
      secret: 'ABC',
    });
  });

  it('calls mfaEnable with the entered code', async () => {
    vi.mocked(api.mfaEnable).mockResolvedValue({ mfa_enabled: true });

    render(<ProfilePage />);

    await waitFor(() => screen.getByText('profile.mfaDisabled'));
    fireEvent.click(screen.getByText('profile.enableMFA'));

    await waitFor(() => screen.getByTestId('qr-code'));

    const input = screen.getByPlaceholderText('profile.codePlaceholder');
    fireEvent.change(input, { target: { value: '123456' } });
    fireEvent.click(screen.getByText('profile.verifyEnable'));

    await waitFor(() => {
      expect(api.mfaEnable).toHaveBeenCalledWith('123456');
    });
  });
});
