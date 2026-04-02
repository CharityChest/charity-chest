import createMiddleware from 'next-intl/middleware';
import { routing } from './i18n/routing';

export default createMiddleware(routing);

export const config = {
  // Match the root and any path under a supported locale.
  // Static files (_next, favicon, etc.) are intentionally excluded.
  matcher: ['/', '/(en|it)/:path*'],
};
