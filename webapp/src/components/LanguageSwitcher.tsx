'use client';

import { useLocale } from 'next-intl';
import { usePathname, useRouter } from '@/i18n/navigation';
import { routing } from '@/i18n/routing';

const LABELS: Record<string, string> = { en: 'EN', it: 'IT' };

export default function LanguageSwitcher() {
  const locale = useLocale();
  const router = useRouter();
  const pathname = usePathname();

  return (
    <div className="flex gap-1">
      {routing.locales.map((l) => (
        <button
          key={l}
          onClick={() => router.replace(pathname, { locale: l })}
          className={`rounded px-2 py-1 text-xs font-semibold transition-colors ${
            l === locale
              ? 'bg-emerald-600 text-white'
              : 'text-gray-400 hover:text-gray-600'
          }`}
          aria-current={l === locale ? 'true' : undefined}
        >
          {LABELS[l]}
        </button>
      ))}
    </div>
  );
}
