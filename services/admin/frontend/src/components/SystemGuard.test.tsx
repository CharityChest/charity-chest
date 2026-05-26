import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@testing-library/react';
import SystemGuard from './SystemGuard';

// Mock next-intl navigation so usePathname and useRouter are controllable.
const mockReplace = vi.fn();
let mockPathname = '/dashboard';

vi.mock('@/i18n/navigation', () => ({
  useRouter: () => ({ replace: mockReplace }),
  usePathname: () => mockPathname,
}));

// Mock the api module.
vi.mock('@/lib/api', () => ({
  api: {
    systemStatus: vi.fn(),
  },
}));

import { api } from '@/lib/api';

beforeEach(() => {
  vi.clearAllMocks();
  mockPathname = '/dashboard';
});

describe('SystemGuard', () => {
  it('redirects to /setup when system is not configured and not already on /setup', async () => {
    vi.mocked(api.systemStatus).mockResolvedValue({ configured: false });

    render(<SystemGuard><div>content</div></SystemGuard>);

    // Wait for the effect to run.
    await vi.waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith('/setup');
    });
  });

  it('does not redirect when system is not configured but already on /setup', async () => {
    mockPathname = '/setup';
    vi.mocked(api.systemStatus).mockResolvedValue({ configured: false });

    render(<SystemGuard><div>content</div></SystemGuard>);

    await vi.waitFor(() => {
      expect(api.systemStatus).toHaveBeenCalled();
    });
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it('redirects to / when system is configured and on /setup', async () => {
    mockPathname = '/setup';
    vi.mocked(api.systemStatus).mockResolvedValue({ configured: true });

    render(<SystemGuard><div>content</div></SystemGuard>);

    await vi.waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith('/');
    });
  });

  it('does not redirect when system is configured and not on /setup', async () => {
    vi.mocked(api.systemStatus).mockResolvedValue({ configured: true });

    render(<SystemGuard><div>content</div></SystemGuard>);

    await vi.waitFor(() => {
      expect(api.systemStatus).toHaveBeenCalled();
    });
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it('fails open on network error — no redirect', async () => {
    vi.mocked(api.systemStatus).mockRejectedValue(new Error('network error'));

    render(<SystemGuard><div>content</div></SystemGuard>);

    await vi.waitFor(() => {
      expect(api.systemStatus).toHaveBeenCalled();
    });
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it('renders children immediately without a loading gate', () => {
    vi.mocked(api.systemStatus).mockReturnValue(new Promise(() => {})); // never resolves

    const { getByText } = render(<SystemGuard><div>child content</div></SystemGuard>);

    expect(getByText('child content')).toBeTruthy();
  });
});
