'use client';

import { useEffect, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Link, usePathname, useRouter } from '@/i18n/navigation';
import { clearToken, isAuthenticated } from '@/lib/auth';
import LanguageSwitcher from '@/components/LanguageSwitcher';

export default function Navbar() {
  const t = useTranslations();
  const router = useRouter();
  const pathname = usePathname();
  const [loggedIn, setLoggedIn] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setLoggedIn(isAuthenticated());
  }, [pathname]);

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  function handleLogout() {
    clearToken();
    setLoggedIn(false);
    setMenuOpen(false);
    router.push('/');
  }

  return (
    <nav className="fixed inset-x-0 top-0 z-50 flex h-14 items-center justify-between border-b border-gray-200 bg-white/90 px-4 backdrop-blur sm:px-6 lg:px-8">
      <Link
        href="/"
        className="text-base font-bold text-emerald-700 hover:text-emerald-800"
      >
        {t('common.appName')}
      </Link>

      <div className="flex items-center gap-3">
        <LanguageSwitcher />

        {loggedIn ? (
          <div className="relative" ref={menuRef}>
            <button
              onClick={() => setMenuOpen((o) => !o)}
              aria-expanded={menuOpen}
              aria-haspopup="true"
              aria-label={t('common.userMenu')}
              className="flex h-8 w-8 items-center justify-center rounded-full bg-emerald-600 text-white hover:bg-emerald-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500"
            >
              <UserIcon />
            </button>

            {menuOpen && (
              <div
                role="menu"
                className="absolute right-0 mt-2 w-44 rounded-md border border-gray-200 bg-white py-1 shadow-lg"
              >
                <Link
                  href="/dashboard"
                  role="menuitem"
                  onClick={() => setMenuOpen(false)}
                  className="block px-4 py-2 text-sm text-gray-700 hover:bg-gray-50"
                >
                  {t('dashboard.title')}
                </Link>
                <button
                  role="menuitem"
                  onClick={handleLogout}
                  className="w-full px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-50"
                >
                  {t('common.logout')}
                </button>
              </div>
            )}
          </div>
        ) : (
          <div className="flex items-center gap-2">
            <Link
              href="/login"
              className="rounded-md px-3 py-1.5 text-sm font-medium text-gray-600 hover:text-gray-900"
            >
              {t('home.login')}
            </Link>
            <Link
              href="/register"
              className="rounded-md bg-emerald-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-700"
            >
              {t('home.register')}
            </Link>
          </div>
        )}
      </div>
    </nav>
  );
}

function UserIcon() {
  return (
    <svg className="h-4 w-4" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M12 12c2.761 0 5-2.239 5-5s-2.239-5-5-5-5 2.239-5 5 2.239 5 5 5zm0 2c-3.314 0-10 1.657-10 5v1h20v-1c0-3.343-6.686-5-10-5z" />
    </svg>
  );
}