'use client';

import Link from 'next/link';
import { useI18n } from '@/components/providers/Providers';
import s from '@/components/account/account.module.css';

export default function NotFound() {
  const { t, locale } = useI18n();
  return (
    <section className={s.page}>
      <div className={`nn-container ${s.narrow} ${s.center}`}>
        <h1 className={s.title}>{t('notFound.title')}</h1>
        <Link href={`/${locale}`} className="nn-btn nn-btn--dark">
          {t('notFound.home')}
        </Link>
      </div>
    </section>
  );
}
