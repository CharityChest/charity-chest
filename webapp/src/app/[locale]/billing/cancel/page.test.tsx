import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';

const mockUseSearchParams = vi.fn(() => new URLSearchParams('org_id=42'));

vi.mock('next/navigation', () => ({
  useSearchParams: () => mockUseSearchParams(),
}));

vi.mock('next-intl', () => ({
  useTranslations: () => (key: string) => key,
}));

vi.mock('@/i18n/navigation', () => ({
  Link: ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  ),
}));

import BillingCancelPage from './page';

describe('BillingCancelPage', () => {
  beforeEach(() => {
    mockUseSearchParams.mockReturnValue(new URLSearchParams('org_id=42'));
  });

  it('renders cancel title', () => {
    render(<BillingCancelPage />);
    expect(screen.getByText('cancelTitle')).toBeTruthy();
  });

  it('renders no-payment message', () => {
    render(<BillingCancelPage />);
    expect(screen.getByText('cancelBody')).toBeTruthy();
  });

  it('renders back-to-org link with correct org_id', () => {
    render(<BillingCancelPage />);
    const link = screen.getByRole('link');
    expect(link.getAttribute('href')).toBe('/orgs/42');
  });

  it('renders without crash when org_id is absent and shows no link', () => {
    mockUseSearchParams.mockReturnValue(new URLSearchParams(''));
    render(<BillingCancelPage />);
    expect(screen.getByText('cancelTitle')).toBeTruthy();
    expect(screen.queryByRole('link')).toBeNull();
  });
});
