import { defineRouting } from 'next-intl/routing';

/** next-intl routing config: supported locales (`en`, `it`) and the default (`en`). */
export const routing = defineRouting({
  locales: ['en', 'it'],
  defaultLocale: 'en',
});
