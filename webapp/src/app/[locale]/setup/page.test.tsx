import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import SetupPage from './page';

const mockReplace = vi.fn();

vi.mock('@/i18n/navigation', () => ({
  useRouter: () => ({ replace: mockReplace }),
}));

vi.mock('next-intl', () => ({
  useTranslations: () => (key: string) => key,
}));

vi.mock('@/lib/api', () => ({
  api: { systemStatus: vi.fn() },
}));

import { api } from '@/lib/api';

beforeEach(() => {
  vi.clearAllMocks();
});

describe('SetupPage — rendering', () => {
  it('renders the title, description, and check-again button', () => {
    render(<SetupPage />);
    expect(screen.getByText('title')).toBeTruthy();
    expect(screen.getByText('description')).toBeTruthy();
    expect(screen.getByText('checkAgain')).toBeTruthy();
  });
});

describe('SetupPage — check again', () => {
  it('calls api.systemStatus when the button is clicked', async () => {
    vi.mocked(api.systemStatus).mockResolvedValue({ configured: false });
    render(<SetupPage />);

    fireEvent.click(screen.getByText('checkAgain'));

    await waitFor(() => {
      expect(api.systemStatus).toHaveBeenCalledTimes(1);
    });
  });

  it('stays on the page when the system is not yet configured', async () => {
    vi.mocked(api.systemStatus).mockResolvedValue({ configured: false });
    render(<SetupPage />);

    fireEvent.click(screen.getByText('checkAgain'));

    await waitFor(() => {
      expect(api.systemStatus).toHaveBeenCalled();
      expect(mockReplace).not.toHaveBeenCalled();
    });
  });

  it('redirects to / when the system becomes configured', async () => {
    vi.mocked(api.systemStatus).mockResolvedValue({ configured: true });
    render(<SetupPage />);

    fireEvent.click(screen.getByText('checkAgain'));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith('/');
    });
  });
});
