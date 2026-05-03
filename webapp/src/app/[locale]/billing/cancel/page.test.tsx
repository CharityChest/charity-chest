import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import BillingCancelPage from './page';

vi.mock('next/navigation', () => ({
  useSearchParams: vi.fn(() => new URLSearchParams('org_id=42')),
}));

vi.mock('next-intl', () => ({
  useTranslations: () => (key: string) => key,
}));

vi.mock('@/i18n/navigation', () => ({
  Link: ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  ),
}));

describe('BillingCancelPage', () => {
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
});
