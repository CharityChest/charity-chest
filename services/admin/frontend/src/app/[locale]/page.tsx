'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import { isAuthenticated } from '@/lib/auth';

export default function HomePage() {
  const t = useTranslations('home');
  const [loggedIn, setLoggedIn] = useState(false);

  useEffect(() => {
    setLoggedIn(isAuthenticated());
  }, []);

  return (
    <main>
      {/* Hero */}
      <section className="px-4 py-16 sm:px-6 sm:py-24 lg:px-8">
        <div className="mx-auto grid max-w-6xl grid-cols-1 items-center gap-12 lg:grid-cols-2">
          {/* Text */}
          <div className="flex flex-col gap-6 text-center lg:text-left">
            <h1 className="text-4xl font-extrabold tracking-tight text-gray-900 sm:text-5xl lg:text-6xl">
              {t('headline')}
            </h1>
            <p className="text-lg text-gray-500 sm:text-xl">
              {t('description')}
            </p>

            {loggedIn ? (
              <div className="flex justify-center lg:justify-start">
                <Link
                  href="/dashboard"
                  className="rounded-md bg-emerald-600 px-8 py-3 text-base font-medium text-white hover:bg-emerald-700"
                >
                  {t('dashboard')}
                </Link>
              </div>
            ) : (
              <div className="flex w-full flex-col gap-3 sm:flex-row sm:justify-center lg:justify-start">
                <Link
                  href="/register"
                  className="rounded-md bg-emerald-600 px-8 py-3 text-center text-base font-medium text-white hover:bg-emerald-700"
                >
                  {t('getStarted')}
                </Link>
                <Link
                  href="/login"
                  className="rounded-md border border-gray-300 px-8 py-3 text-center text-base font-medium text-gray-700 hover:bg-gray-50"
                >
                  {t('login')}
                </Link>
              </div>
            )}
          </div>

          {/* Illustration */}
          <div className="flex justify-center">
            <ShippedPackIllustration />
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="bg-white px-4 py-16 sm:px-6 lg:px-8">
        <div className="mx-auto max-w-5xl">
          <div className="grid grid-cols-1 gap-8 sm:grid-cols-3">
            <FeatureCard
              icon={<CatalogIcon />}
              title={t('feature1Title')}
              description={t('feature1Desc')}
            />
            <FeatureCard
              icon={<StoreIcon />}
              title={t('feature2Title')}
              description={t('feature2Desc')}
            />
            <FeatureCard
              icon={<TrackIcon />}
              title={t('feature3Title')}
              description={t('feature3Desc')}
            />
          </div>
        </div>
      </section>
    </main>
  );
}

function ShippedPackIllustration() {
  return (
    <svg
      viewBox="0 0 480 400"
      className="w-full max-w-sm lg:max-w-md"
      aria-hidden="true"
      xmlns="http://www.w3.org/2000/svg"
    >
      {/* ── Background circle ── */}
      <circle cx="220" cy="215" r="178" fill="#ECFDF5" />

      {/* ── Dashed travel path ── */}
      <path
        d="M32 335 Q130 180 220 255 Q305 320 430 155"
        fill="none"
        stroke="#6EE7B7"
        strokeWidth="2"
        strokeDasharray="7 5"
      />

      {/* ── Origin dot ── */}
      <circle cx="32" cy="335" r="8" fill="#10B981" />
      <circle cx="32" cy="335" r="15" fill="none" stroke="#6EE7B7" strokeWidth="2" />

      {/* ── Destination pin ── */}
      <path
        d="M430 118 C419 118 410 128 410 140 C410 156 430 174 430 174 C430 174 450 156 450 140 C450 128 441 118 430 118Z"
        fill="#10B981"
      />
      <circle cx="430" cy="140" r="7" fill="white" />

      {/* ── Box: right side face (drawn first / behind front) ── */}
      <polygon
        points="260,165 310,115 310,275 260,325"
        fill="#FCD34D"
        stroke="#D97706"
        strokeWidth="2"
        strokeLinejoin="round"
      />

      {/* ── Box: top face ── */}
      <polygon
        points="100,165 260,165 310,115 150,115"
        fill="#FDE68A"
        stroke="#D97706"
        strokeWidth="2"
        strokeLinejoin="round"
      />

      {/* ── Box: front face (square 160×160) ── */}
      <rect x="100" y="165" width="160" height="160" fill="#FEF3C7" stroke="#D97706" strokeWidth="2" />

      {/* ── Tape: vertical strip continuing over top ── */}
      <polygon
        points="170,165 192,165 242,115 220,115"
        fill="#A7F3D0"
        opacity="0.8"
      />
      <rect x="170" y="165" width="22" height="160" fill="#A7F3D0" opacity="0.8" />

      {/* ── Tape: horizontal strip continuing on right side ── */}
      <polygon
        points="260,235 310,185 310,207 260,257"
        fill="#A7F3D0"
        opacity="0.8"
      />
      <rect x="100" y="235" width="160" height="22" fill="#A7F3D0" opacity="0.8" />

      {/* ── Heart label (top-left quadrant of front face) ── */}
      <path
        d="M135 215 C135 215 115 201 115 188
           C115 179 122 175 135 183
           C148 175 155 179 155 188
           C155 201 135 215 135 215Z"
        fill="#10B981"
      />

      {/* ── Shadow under box ── */}
      <ellipse cx="190" cy="342" rx="90" ry="11" fill="#D1FAE5" />

      {/* ── Motion lines (left of box) ── */}
      <g stroke="#6EE7B7" strokeWidth="2.5" strokeLinecap="round">
        <line x1="58" y1="210" x2="82" y2="210" />
        <line x1="52" y1="228" x2="80" y2="228" />
        <line x1="60" y1="246" x2="82" y2="246" />
      </g>

      {/* ── Sparkles ── */}
      <g fill="#34D399">
        {/* top-left star */}
        <polygon points="75,95 78,104 87,104 80,110 83,119 75,113 67,119 70,110 63,104 72,104" />
        {/* small dot cluster */}
        <circle cx="355" cy="240" r="5" />
        <circle cx="368" cy="255" r="3" />
        <circle cx="348" cy="258" r="3" />
      </g>
    </svg>
  );
}

function FeatureCard({
  icon,
  title,
  description,
}: {
  icon: React.ReactNode;
  title: string;
  description: string;
}) {
  return (
    <div className="flex flex-col items-center gap-4 rounded-xl border border-gray-100 p-6 text-center shadow-sm">
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-emerald-50 text-emerald-600">
        {icon}
      </div>
      <h3 className="text-lg font-semibold text-gray-900">{title}</h3>
      <p className="text-sm text-gray-500">{description}</p>
    </div>
  );
}

function CatalogIcon() {
  return (
    <svg className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24" aria-hidden="true">
      <path strokeLinecap="round" strokeLinejoin="round" d="M9 12h6m-6 4h6M5 8h14M7 4h10a2 2 0 012 2v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6a2 2 0 012-2z" />
    </svg>
  );
}

function StoreIcon() {
  return (
    <svg className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24" aria-hidden="true">
      <path strokeLinecap="round" strokeLinejoin="round" d="M20 7H4a2 2 0 00-2 2v10a2 2 0 002 2h16a2 2 0 002-2V9a2 2 0 00-2-2zM4 7V5a2 2 0 012-2h12a2 2 0 012 2v2M12 12v5m-3-2.5h6" />
    </svg>
  );
}

function TrackIcon() {
  return (
    <svg className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24" aria-hidden="true">
      <path strokeLinecap="round" strokeLinejoin="round" d="M3.055 11H5a2 2 0 012 2v1a2 2 0 002 2 2 2 0 012 2v2.945M8 3.935V5.5A2.5 2.5 0 0010.5 8h.5a2 2 0 012 2 2 2 0 104 0 2 2 0 012-2h1.064M15 20.488V18a2 2 0 012-2h3.064M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );
}