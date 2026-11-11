'use client';

import { useEffect, useState } from 'react';
import { ErrorBox } from '@/components/auth/ErrorBox';
import { Icon } from '@/components/ui/Icon';
import { useErrorText, useI18n, useSession } from '@/components/providers/Providers';
import { ApiError, member, type Points } from '@/lib/api/client';
import { formatDateTime, formatNumber } from '@/lib/format';
import { LoginGate } from './MyTickets';
import s from './account.module.css';

export function MyPoints() {
  const { t, locale } = useI18n();
  const errText = useErrorText();
  const session = useSession();
  const [data, setData] = useState<Points | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (session.status !== 'authed') return;
    member.points().then(setData, (e: ApiError) => setError(e.code));
  }, [session.status]);

  const reason = (r: string) => {
    const k = `points.reason.${r}`;
    return t(k) === k ? r : t(k);
  };

  return (
    <section className={s.page}>
      <div className={`nn-container ${s.narrow}`}>
        <div className={s.head}>
          <h1 className={s.title}>{t('points.title')}</h1>
        </div>
        {session.status === 'anon' ? (
          <LoginGate />
        ) : error ? (
          <ErrorBox text={errText(error)} />
        ) : !data ? (
          <div className={s.center}>
            <div className="nn-spinner" aria-label={t('common.loading')} />
          </div>
        ) : (
          <div className={s.stack}>
            <div className={s.pointsHero}>
              <span className={s.pointsLabel}>{t('points.balance')}</span>
              <span className={s.pointsValue} data-testid="points-balance">
                {formatNumber(data.balance, locale)}
                <small>pts</small>
              </span>
              {data.pending > 0 && <span>{t('points.pending', { points: formatNumber(data.pending, locale) })}</span>}
            </div>

            <div className={s.soon}>
              <span className={s.soonIcon} aria-hidden="true">
                <Icon name="gift" size={22} />
              </span>
              <div>
                <h2>{t('points.soonTitle')}</h2>
                <p className={s.lead}>{t('points.soonBody')}</p>
              </div>
            </div>

            <div className={s.card}>
              <h2 className={s.sectionTitle}>{t('points.history')}</h2>
              {data.items.length === 0 ? (
                <p className={s.lead}>{t('points.empty')}</p>
              ) : (
                <ul className={s.history}>
                  {data.items.map((e) => (
                    <li key={e.id}>
                      <div className={s.historyReason}>
                        <strong>{reason(e.reason)}</strong>
                        {e.createdAt && <span>{formatDateTime(e.createdAt, locale)}</span>}
                      </div>
                      <span className={`${s.delta} ${e.deltaPoints < 0 ? s.deltaNeg : ''}`}>
                        {e.deltaPoints > 0 ? '+' : ''}
                        {formatNumber(e.deltaPoints, locale)}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        )}
      </div>
    </section>
  );
}
