import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';

export default function HomePage() {
  const t = useTranslations();

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-8 p-8">
      <div className="text-center">
        <h1 className="text-4xl font-bold tracking-tight text-emerald-700">
          {t('common.appName')}
        </h1>
        <p className="mt-2 text-lg text-gray-500">{t('common.tagline')}</p>
      </div>

      <div className="flex gap-4">
        <Link
          href="/login"
          className="rounded-md bg-emerald-600 px-6 py-2 text-sm font-medium text-white hover:bg-emerald-700"
        >
          {t('home.login')}
        </Link>
        <Link
          href="/register"
          className="rounded-md border border-emerald-600 px-6 py-2 text-sm font-medium text-emerald-700 hover:bg-emerald-50"
        >
          {t('home.register')}
        </Link>
      </div>
    </main>
  );
}
