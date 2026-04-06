'use client';

import { useEffect } from 'react';
import { usePathname, useRouter } from '@/i18n/navigation';
import { api } from '@/lib/api';

/**
 * SystemGuard checks whether the system has been configured (a root user exists).
 *
 * Behaviour:
 * - Not configured and not on /setup → redirects to /setup.
 * - Configured and on /setup → redirects to /.
 * - Network error → fails open (no redirect) so a server blip does not lock users out.
 *
 * Children are rendered immediately (no loading gate) so SSR is unaffected.
 * The redirect only fires client-side after hydration.
 *
 * usePathname() from next-intl strips the locale prefix (e.g. /en/setup → /setup).
 */
export default function SystemGuard({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    api
      .systemStatus()
      .then((status) => {
        const onSetup = pathname === '/setup';
        if (!status.configured && !onSetup) {
          router.replace('/setup');
        } else if (status.configured && onSetup) {
          router.replace('/');
        }
      })
      .catch(() => {
        // Fail open on network errors.
      });
  }, [pathname, router]);

  return <>{children}</>;
}
