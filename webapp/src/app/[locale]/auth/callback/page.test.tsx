import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import AuthCallbackPage from './page';

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
  setToken: vi.fn(),
}));

import { setToken } from '@/lib/auth';

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('AuthCallbackPage — token in URL', () => {
  it('stores the token and redirects to /dashboard', async () => {
    vi.stubGlobal('location', { search: '?token=my-jwt' });
    render(<AuthCallbackPage />);

    await waitFor(() => {
      expect(setToken).toHaveBeenCalledWith('my-jwt');
      expect(mockReplace).toHaveBeenCalledWith('/dashboard');
    });
  });
});

describe('AuthCallbackPage — no token in URL', () => {
  it('shows an error banner when error param is present', async () => {
    vi.stubGlobal('location', { search: '?error=access_denied' });
    render(<AuthCallbackPage />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
    });
  });

  it('shows an error banner with a default message when URL has no params', async () => {
    vi.stubGlobal('location', { search: '' });
    render(<AuthCallbackPage />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
    });
  });

  it('shows a try-again link back to /login', async () => {
    vi.stubGlobal('location', { search: '?error=oops' });
    render(<AuthCallbackPage />);

    await waitFor(() => {
      const link = screen.getByText('tryAgain');
      expect(link.closest('a')!.getAttribute('href')).toBe('/login');
    });
  });
});
