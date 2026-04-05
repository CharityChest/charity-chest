import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';

export default function HomePage() {
  const t = useTranslations();

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-8 px-4 py-12 sm:px-6 lg:px-8">
      <div className="text-center">
        <h1 className="text-3xl font-bold tracking-tight text-emerald-700 sm:text-4xl">
          {t('common.appName')}
        </h1>
        <p className="mt-2 text-lg text-gray-500">{t('common.tagline')}</p>
      </div>

      <div className="flex w-full max-w-xs flex-col gap-3 sm:max-w-none sm:flex-row sm:justify-center">
        <Link
          href="/login"
          className="rounded-md bg-emerald-600 px-6 py-3 text-center text-sm font-medium text-white hover:bg-emerald-700 sm:py-2"
        >
          {t('home.login')}
        </Link>
        <Link
          href="/register"
          className="rounded-md border border-emerald-600 px-6 py-3 text-center text-sm font-medium text-emerald-700 hover:bg-emerald-50 sm:py-2"
        >
          {t('home.register')}
        </Link>
      </div>
    </main>
  );
}
