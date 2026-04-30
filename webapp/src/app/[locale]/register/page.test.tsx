import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import RegisterPage from './page';

const mockPush = vi.fn();

vi.mock('@/i18n/navigation', () => ({
  useRouter: () => ({ push: mockPush }),
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
    api: { register: vi.fn() },
  };
});

import { setToken } from '@/lib/auth';
import { api, ApiError } from '@/lib/api';

beforeEach(() => {
  vi.clearAllMocks();
});

describe('RegisterPage — rendering', () => {
  it('shows name, email, password fields and a submit button', () => {
    render(<RegisterPage />);
    expect(screen.getByLabelText('register.name')).toBeTruthy();
    expect(screen.getByLabelText('register.email')).toBeTruthy();
    expect(screen.getByPlaceholderText('••••••••')).toBeTruthy();
    expect(screen.getByText('register.submit')).toBeTruthy();
  });

  it('shows a link to the login page', () => {
    render(<RegisterPage />);
    const link = screen.getByText('register.signIn');
    expect(link.closest('a')!.getAttribute('href')).toBe('/login');
  });
});

describe('RegisterPage — form submission', () => {
  it('stores token and pushes to /dashboard on success', async () => {
    vi.mocked(api.register).mockResolvedValue({
      token: 'new-jwt',
      user: { id: 1, email: 'u@u.com', name: 'User', created_at: '', updated_at: '' },
    });

    render(<RegisterPage />);

    fireEvent.change(screen.getByLabelText('register.name'), { target: { value: 'Alice' } });
    fireEvent.change(screen.getByLabelText('register.email'), { target: { value: 'alice@example.com' } });
    fireEvent.change(screen.getByPlaceholderText('••••••••'), { target: { value: 'password1' } });
    fireEvent.click(screen.getByText('register.submit'));

    await waitFor(() => {
      expect(api.register).toHaveBeenCalledWith('alice@example.com', 'password1', 'Alice');
      expect(setToken).toHaveBeenCalledWith('new-jwt');
      expect(mockPush).toHaveBeenCalledWith('/dashboard');
    });
  });

  it('shows error banner on failed registration', async () => {
    vi.mocked(api.register).mockRejectedValue(new ApiError(409, 'email already in use'));

    render(<RegisterPage />);

    fireEvent.change(screen.getByLabelText('register.name'), { target: { value: 'Bob' } });
    fireEvent.change(screen.getByLabelText('register.email'), { target: { value: 'bob@example.com' } });
    fireEvent.change(screen.getByPlaceholderText('••••••••'), { target: { value: 'password1' } });
    fireEvent.click(screen.getByText('register.submit'));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeTruthy();
      expect(screen.getByText('email already in use')).toBeTruthy();
    });
  });
});
