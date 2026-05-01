import { createNavigation } from 'next-intl/navigation';
import { routing } from './routing';

/**
 * Locale-aware navigation helpers generated from the app's routing config.
 * Always import `Link`, `useRouter`, `usePathname`, and `redirect` from this module —
 * never from `next/link` or `next/navigation` — so the current locale is preserved on
 * every navigation without having to pass it manually.
 */
export const { Link, useRouter, usePathname, redirect } = createNavigation(routing);
