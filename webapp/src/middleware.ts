import createMiddleware from 'next-intl/middleware';
import { routing } from './i18n/routing';

/** next-intl middleware: detects the locale from the URL and redirects accordingly. */
export default createMiddleware(routing);

/**
 * Next.js middleware matcher config.
 * Applies to the root path and all locale-prefixed paths; static assets are excluded.
 */
export const config = {
  matcher: ['/', '/(en|it)/:path*'],
};
