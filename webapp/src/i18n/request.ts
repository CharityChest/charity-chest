import { getRequestConfig } from 'next-intl/server';
import { routing } from './routing';

/**
 * next-intl server-side config: resolves the active locale for each request and
 * loads the corresponding message JSON from `messages/<locale>.json`.
 * Unknown or missing locales fall back to the routing default (`en`).
 */
export default getRequestConfig(async ({ requestLocale }) => {
  let locale = await requestLocale;

  if (!locale || !(routing.locales as readonly string[]).includes(locale)) {
    locale = routing.defaultLocale;
  }

  return {
    locale,
    messages: (await import(`../../messages/${locale}.json`)).default,
  };
});
