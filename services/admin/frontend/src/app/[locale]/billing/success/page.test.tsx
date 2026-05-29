import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';

const mockUseSearchParams = vi.fn(() => new URLSearchParams('org_uuid=00000000-0000-0000-0000-000000000042'));

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

import BillingSuccessPage from './page';

describe('BillingSuccessPage', () => {
  beforeEach(() => {
    mockUseSearchParams.mockReturnValue(new URLSearchParams('org_uuid=00000000-0000-0000-0000-000000000042'));
  });

  it('renders success title', () => {
    render(<BillingSuccessPage />);
    expect(screen.getByText('successTitle')).toBeTruthy();
  });

  it('renders processing message', () => {
    render(<BillingSuccessPage />);
    expect(screen.getByText('successBody')).toBeTruthy();
  });

  it('renders back-to-org link with correct org_uuid', () => {
    render(<BillingSuccessPage />);
    const link = screen.getByRole('link');
    expect(link.getAttribute('href')).toBe('/orgs/00000000-0000-0000-0000-000000000042');
  });

  it('renders without crash when org_uuid is absent', () => {
    mockUseSearchParams.mockReturnValue(new URLSearchParams(''));
    render(<BillingSuccessPage />);
    expect(screen.getByText('successTitle')).toBeTruthy();
    expect(screen.queryByRole('link')).toBeNull();
  });
});
