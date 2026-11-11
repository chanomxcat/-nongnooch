'use client';

import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { useEffect, useState } from 'react';
import { Icon } from '@/components/ui/Icon';
import { useI18n } from '@/components/providers/Providers';
import { member } from '@/lib/api/client';
import { formatNumber } from '@/lib/format';
import { safeNext } from '@/lib/safeNext';
import s from '@/components/account/account.module.css';

/** Registration done: shows the real balance from member (welcome 100 + invite 50 = 150 in the demo). */
export function Welcome() {
  const { t, locale } = useI18n();
  const params = useSearchParams();
  const next = safeNext(params.get('next'), locale);
  const [points, setPoints] = useState<number | null>(null);

  useEffect(() => {
    member
      .points()
      .then((p) => setPoints(p.balance + p.pending))
      .catch(() => setPoints(null));
  }, []);

  const buyHref = next === `/${locale}` ? `/${locale}/checkout` : next;
  return (
    <section className={s.page}>
      <div className={`nn-container ${s.narrow} ${s.center}`}>
        <div className={s.burst} aria-hidden="true">
          <div>
            <strong>{points === null ? '…' : formatNumber(points, locale)}</strong>
            <small>POINTS</small>
          </div>
        </div>
        <h1 className={s.title}>{t('done.title')}</h1>
        {points !== null && (
          <p className={s.badge} style={{ justifySelf: 'center' }} role="status">
            <Icon name="star" size={16} />
            {t('done.points', { points: formatNumber(points, locale) })}
          </p>
        )}
        <p className={s.lead}>{t('done.lead')}</p>
        <div className={s.actions}>
          <Link href={buyHref} className="nn-btn nn-btn--gold">
            <Icon name="ticket" size={20} />
            {t('done.buy')}
          </Link>
          <Link href={`/${locale}/points`} className="nn-btn nn-btn--outline-dark">
            {t('done.viewPoints')}
          </Link>
        </div>
      </div>
    </section>
  );
}
