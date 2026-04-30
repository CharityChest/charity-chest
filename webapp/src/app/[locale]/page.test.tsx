import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import HomePage from './page';

vi.mock('@/i18n/navigation', () => ({
  Link: ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock('next-intl', () => ({
  useTranslations: () => (key: string) => key,
}));

vi.mock('@/lib/auth', () => ({
  isAuthenticated: vi.fn(),
}));

import { isAuthenticated } from '@/lib/auth';

beforeEach(() => {
  vi.clearAllMocks();
});

describe('HomePage — unauthenticated', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(false);
  });

  it('renders the page headline', async () => {
    render(<HomePage />);
    await waitFor(() => {
      expect(screen.getByText('headline')).toBeTruthy();
    });
  });

  it('shows Get Started and Login links', async () => {
    render(<HomePage />);
    await waitFor(() => {
      const getStarted = screen.getByText('getStarted');
      expect(getStarted.closest('a')!.getAttribute('href')).toBe('/register');
      const login = screen.getByText('login');
      expect(login.closest('a')!.getAttribute('href')).toBe('/login');
    });
  });

  it('does not show the Dashboard link', async () => {
    render(<HomePage />);
    await waitFor(() => {
      expect(screen.queryByText('dashboard')).toBeNull();
    });
  });
});

describe('HomePage — authenticated', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
  });

  it('shows the Dashboard link', async () => {
    render(<HomePage />);
    await waitFor(() => {
      const link = screen.getByText('dashboard');
      expect(link.closest('a')!.getAttribute('href')).toBe('/dashboard');
    });
  });

  it('does not show Get Started or Login links', async () => {
    render(<HomePage />);
    await waitFor(() => {
      expect(screen.queryByText('getStarted')).toBeNull();
    });
  });
});
