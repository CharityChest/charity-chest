import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import Navbar from './Navbar';

let mockPathname = '/dashboard';
const mockPush = vi.fn();

vi.mock('@/i18n/navigation', () => ({
  useRouter: () => ({ push: mockPush }),
  usePathname: () => mockPathname,
  Link: ({ href, children, ...props }: { href: string; children: React.ReactNode; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock('next-intl', () => ({
  useTranslations: () => (key: string) => key,
}));

vi.mock('@/lib/auth', () => ({
  isAuthenticated: vi.fn(),
  getRole: vi.fn(),
  clearToken: vi.fn(),
}));

vi.mock('@/components/LanguageSwitcher', () => ({
  default: () => <div data-testid="lang-switcher" />,
}));

import { isAuthenticated, getRole, clearToken } from '@/lib/auth';

beforeEach(() => {
  vi.clearAllMocks();
  mockPathname = '/dashboard';
});

describe('Navbar — unauthenticated state', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(false);
    vi.mocked(getRole).mockReturnValue(null);
  });

  it('shows Login and Register links', () => {
    render(<Navbar />);
    expect(screen.getByText('home.login')).toBeTruthy();
    expect(screen.getByText('home.register')).toBeTruthy();
  });

  it('does not show user menu button', () => {
    render(<Navbar />);
    expect(screen.queryByLabelText('common.userMenu')).toBeNull();
  });
});

describe('Navbar — authenticated with no system role', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue(null);
  });

  it('shows the user menu button', () => {
    render(<Navbar />);
    expect(screen.getByLabelText('common.userMenu')).toBeTruthy();
  });

  it('does not show Login/Register links', () => {
    render(<Navbar />);
    expect(screen.queryByText('home.login')).toBeNull();
    expect(screen.queryByText('home.register')).toBeNull();
  });

  it('opens the dropdown and shows Dashboard link', () => {
    render(<Navbar />);
    fireEvent.click(screen.getByLabelText('common.userMenu'));
    expect(screen.getByText('dashboard.title')).toBeTruthy();
  });

  it('does not show Organisations link in dropdown for roleless users', () => {
    render(<Navbar />);
    fireEvent.click(screen.getByLabelText('common.userMenu'));
    expect(screen.queryByText('orgs.title')).toBeNull();
  });

  it('does not show Admin Users link in dropdown for roleless users', () => {
    render(<Navbar />);
    fireEvent.click(screen.getByLabelText('common.userMenu'));
    expect(screen.queryByText('adminUsers.title')).toBeNull();
  });
});

describe('Navbar — authenticated as system', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('system');
  });

  it('shows Organisations link in dropdown', () => {
    render(<Navbar />);
    fireEvent.click(screen.getByLabelText('common.userMenu'));
    expect(screen.getByText('orgs.title')).toBeTruthy();
  });

  it('does not show Admin Users link in dropdown for system role', () => {
    render(<Navbar />);
    fireEvent.click(screen.getByLabelText('common.userMenu'));
    expect(screen.queryByText('adminUsers.title')).toBeNull();
  });
});

describe('Navbar — authenticated as root', () => {
  beforeEach(() => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');
  });

  it('shows both Organisations and Admin Users links', () => {
    render(<Navbar />);
    fireEvent.click(screen.getByLabelText('common.userMenu'));
    expect(screen.getByText('orgs.title')).toBeTruthy();
    expect(screen.getByText('adminUsers.title')).toBeTruthy();
  });
});

describe('Navbar — logout', () => {
  it('calls clearToken and pushes to / on logout', () => {
    vi.mocked(isAuthenticated).mockReturnValue(true);
    vi.mocked(getRole).mockReturnValue('root');

    render(<Navbar />);
    fireEvent.click(screen.getByLabelText('common.userMenu'));
    fireEvent.click(screen.getByText('common.logout'));

    expect(clearToken).toHaveBeenCalled();
    expect(mockPush).toHaveBeenCalledWith('/');
  });
});
