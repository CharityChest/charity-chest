import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import ResetPasswordPage from './page';

const mockReplace = vi.fn();

vi.mock('@/i18n/navigation', () => ({
  useRouter: () => ({ replace: mockReplace, push: vi.fn() }),
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
      resetPassword: vi.fn(),
    },
  };
});

import { api, ApiError } from '@/lib/api';

// setLocationSearch swaps the JSDOM window.location.search so the page's
// useEffect can pick up the token. Restored automatically by JSDOM between
// tests because each `render` mounts a fresh tree.
function setLocationSearch(search: string) {
  // window.history.replaceState updates window.location.search without a
  // navigation, which JSDOM supports out of the box.
  window.history.replaceState({}, '', `/en/reset-password${search}`);
}

beforeEach(() => {
  vi.clearAllMocks();
  setLocationSearch('');
});

describe('ResetPasswordPage', () => {
  it('renders the form with translation keys', () => {
    setLocationSearch('?token=abc123');
    render(<ResetPasswordPage />);
    expect(screen.getByLabelText(/^password/)).toBeTruthy();
    expect(screen.getByLabelText('confirm')).toBeTruthy();
    expect(screen.getByText('submit')).toBeTruthy();
  });

  it('strips the token from window.location after reading it', async () => {
    setLocationSearch('?token=secret-token&keep=me');
    render(<ResetPasswordPage />);
    await waitFor(() => {
      expect(window.location.search).toBe('?keep=me');
    });
  });

  it('shows missingToken banner when ?token= is absent', async () => {
    setLocationSearch(''); // no token
    render(<ResetPasswordPage />);
    await waitFor(() => {
      expect(screen.getByText('missingToken')).toBeTruthy();
    });
  });

  it('blocks submit when passwords do not match', async () => {
    setLocationSearch('?token=abc123');
    render(<ResetPasswordPage />);

    fireEvent.change(screen.getByLabelText(/^password/), { target: { value: 'longenough1' } });
    fireEvent.change(screen.getByLabelText('confirm'), { target: { value: 'differentone1' } });
    fireEvent.click(screen.getByText('submit'));

    await waitFor(() => {
      expect(screen.getByText('mismatch')).toBeTruthy();
      expect(api.resetPassword).not.toHaveBeenCalled();
    });
  });

  it('blocks submit when password is shorter than 8 chars', async () => {
    setLocationSearch('?token=abc123');
    render(<ResetPasswordPage />);

    fireEvent.change(screen.getByLabelText(/^password/), { target: { value: 'short' } });
    fireEvent.change(screen.getByLabelText('confirm'), { target: { value: 'short' } });
    // The native minLength on the input blocks short submits, so we bypass it
    // by calling the form directly via the button's click handler. We use a
    // semi-realistic value just long enough to satisfy the native minLength
    // attribute then assert the client-side check rejects it.
    // Re-fill with a value of length 7 that bypasses the native minLength check
    // by setting the property directly (JSDOM accepts shorter values when
    // submitted programmatically; the page's own guard catches the rest).
    const pw = screen.getByLabelText(/^password/) as HTMLInputElement;
    const cf = screen.getByLabelText('confirm') as HTMLInputElement;
    // JSDOM permits programmatic mismatch; the page's own check is the
    // contract we're asserting.
    fireEvent.change(pw, { target: { value: '1234567' } });
    fireEvent.change(cf, { target: { value: '1234567' } });
    fireEvent.click(screen.getByText('submit'));

    await waitFor(() => {
      // tooShort wins because the page checks length before mismatch.
      expect(screen.getByText('tooShort')).toBeTruthy();
      expect(api.resetPassword).not.toHaveBeenCalled();
    });
  });

  it('submits successfully and shows the confirmation panel', async () => {
    setLocationSearch('?token=happy-token');
    vi.mocked(api.resetPassword).mockResolvedValue(undefined);
    render(<ResetPasswordPage />);

    fireEvent.change(screen.getByLabelText(/^password/), { target: { value: 'brandnewpass1' } });
    fireEvent.change(screen.getByLabelText('confirm'), { target: { value: 'brandnewpass1' } });
    fireEvent.click(screen.getByText('submit'));

    await waitFor(() => {
      expect(api.resetPassword).toHaveBeenCalledWith('happy-token', 'brandnewpass1');
      expect(screen.getByText('successTitle')).toBeTruthy();
    });
  });

  it('redirects to /login when the user clicks the post-success button', async () => {
    setLocationSearch('?token=happy-token');
    vi.mocked(api.resetPassword).mockResolvedValue(undefined);
    render(<ResetPasswordPage />);

    fireEvent.change(screen.getByLabelText(/^password/), { target: { value: 'brandnewpass1' } });
    fireEvent.change(screen.getByLabelText('confirm'), { target: { value: 'brandnewpass1' } });
    fireEvent.click(screen.getByText('submit'));

    await waitFor(() => screen.getByText('successTitle'));
    fireEvent.click(screen.getByText('backToLogin'));
    expect(mockReplace).toHaveBeenCalledWith('/login');
  });

  it('surfaces server validation errors via ErrorBanner', async () => {
    setLocationSearch('?token=stale-token');
    vi.mocked(api.resetPassword).mockRejectedValue(new ApiError(400, 'this password reset link is invalid or has expired'));
    render(<ResetPasswordPage />);

    fireEvent.change(screen.getByLabelText(/^password/), { target: { value: 'brandnewpass1' } });
    fireEvent.change(screen.getByLabelText('confirm'), { target: { value: 'brandnewpass1' } });
    fireEvent.click(screen.getByText('submit'));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
      expect(screen.getByText('this password reset link is invalid or has expired')).toBeTruthy();
    });
  });
});
