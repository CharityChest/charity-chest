import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import ForgotPasswordPage from './page';

vi.mock('@/i18n/navigation', () => ({
  Link: ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock('next-intl', () => ({
  useTranslations: () => (key: string) => key,
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
      forgotPassword: vi.fn(),
    },
  };
});

import { api, ApiError } from '@/lib/api';

beforeEach(() => {
  vi.clearAllMocks();
});

describe('ForgotPasswordPage', () => {
  it('renders the email form with translation keys', () => {
    render(<ForgotPasswordPage />);
    expect(screen.getByLabelText('email')).toBeTruthy();
    expect(screen.getByText('send')).toBeTruthy();
  });

  it('shows the success panel after a successful submission', async () => {
    vi.mocked(api.forgotPassword).mockResolvedValue(undefined);
    render(<ForgotPasswordPage />);

    fireEvent.change(screen.getByLabelText('email'), { target: { value: 'user@example.com' } });
    fireEvent.click(screen.getByText('send'));

    await waitFor(() => {
      expect(api.forgotPassword).toHaveBeenCalledWith('user@example.com');
      expect(screen.getByText('successTitle')).toBeTruthy();
      expect(screen.getByText('successBody')).toBeTruthy();
    });
  });

  it('shows an error banner when the API rejects', async () => {
    vi.mocked(api.forgotPassword).mockRejectedValue(new ApiError(500, 'boom'));
    render(<ForgotPasswordPage />);

    fireEvent.change(screen.getByLabelText('email'), { target: { value: 'user@example.com' } });
    fireEvent.click(screen.getByText('send'));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
      expect(screen.getByText('boom')).toBeTruthy();
    });
  });

  it('still shows the success panel even for unknown emails (enumeration-safe)', async () => {
    // The server returns 204 for unknown emails — the page must treat that the
    // same as a known email so the disabled state is not leaked.
    vi.mocked(api.forgotPassword).mockResolvedValue(undefined);
    render(<ForgotPasswordPage />);

    fireEvent.change(screen.getByLabelText('email'), { target: { value: 'ghost@example.com' } });
    fireEvent.click(screen.getByText('send'));

    await waitFor(() => {
      expect(screen.getByText('successTitle')).toBeTruthy();
    });
  });

  it('renders a back-to-login link', () => {
    render(<ForgotPasswordPage />);
    const link = screen.getByText('backToLogin').closest('a');
    expect(link).not.toBeNull();
    expect(link!.getAttribute('href')).toBe('/login');
  });
});
