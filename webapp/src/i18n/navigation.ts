import { createNavigation } from 'next-intl/navigation';
import { routing } from './routing';

// Locale-aware navigation helpers.
// Import Link, useRouter, usePathname, and redirect from here — NOT from
// 'next/link' or 'next/navigation' — so locale is preserved on every navigation.
export const { Link, useRouter, usePathname, redirect } = createNavigation(routing);
