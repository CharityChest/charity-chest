import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import LanguageSwitcher from './LanguageSwitcher';

const mockReplace = vi.fn();
const mockPathname = '/some/path';

vi.mock('@/i18n/navigation', () => ({
  useRouter: () => ({ replace: mockReplace }),
  usePathname: () => mockPathname,
}));

vi.mock('next-intl', () => ({
  useLocale: vi.fn(),
}));

vi.mock('@/i18n/routing', () => ({
  routing: { locales: ['en', 'it'] },
}));

import { useLocale } from 'next-intl';

beforeEach(() => {
  vi.clearAllMocks();
});

describe('LanguageSwitcher', () => {
  it('renders buttons for all supported locales', () => {
    vi.mocked(useLocale).mockReturnValue('en');
    render(<LanguageSwitcher />);
    expect(screen.getByText('EN')).toBeTruthy();
    expect(screen.getByText('IT')).toBeTruthy();
  });

  it('marks the active locale button with aria-current', () => {
    vi.mocked(useLocale).mockReturnValue('en');
    render(<LanguageSwitcher />);
    expect(screen.getByText('EN').getAttribute('aria-current')).toBe('true');
    expect(screen.getByText('IT').getAttribute('aria-current')).toBeNull();
  });

  it('marks IT as active when locale is it', () => {
    vi.mocked(useLocale).mockReturnValue('it');
    render(<LanguageSwitcher />);
    expect(screen.getByText('IT').getAttribute('aria-current')).toBe('true');
    expect(screen.getByText('EN').getAttribute('aria-current')).toBeNull();
  });

  it('calls router.replace with the new locale when a button is clicked', () => {
    vi.mocked(useLocale).mockReturnValue('en');
    render(<LanguageSwitcher />);
    fireEvent.click(screen.getByText('IT'));
    expect(mockReplace).toHaveBeenCalledWith(mockPathname, { locale: 'it' });
  });
});
