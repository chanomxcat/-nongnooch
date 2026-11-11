'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { ErrorBox } from '@/components/auth/ErrorBox';
import { Icon } from '@/components/ui/Icon';
import { useErrorText, useI18n, useSession } from '@/components/providers/Providers';
import { ApiError, ticket, type Ticket } from '@/lib/api/client';
import { TicketCard } from './TicketCard';
import s from './account.module.css';

export function LoginGate() {
  const { t, locale } = useI18n();
  const here = typeof window !== 'undefined' ? window.location.pathname : `/${locale}`;
  return (
    <div className={`${s.card} ${s.center}`}>
      <Icon name="lock" size={28} />
      <p>{t('account.loginRequired')}</p>
      <Link href={`/${locale}/login?next=${encodeURIComponent(here)}`} className="nn-btn nn-btn--dark">
        {t('account.loginButton')}
      </Link>
    </div>
  );
}

export function MyTickets() {
  const { t, locale } = useI18n();
  const errText = useErrorText();
  const session = useSession();
  const [items, setItems] = useState<Ticket[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (session.status !== 'authed') return;
    ticket.mine().then(
      (list) => setItems([...list].sort((a, b) => (a.status === 'active' ? 0 : 1) - (b.status === 'active' ? 0 : 1) || (a.visitDate ?? '').localeCompare(b.visitDate ?? ''))),
      (e: ApiError) => setError(e.code),
    );
  }, [session.status]);

  return (
    <section className={s.page}>
      <div className={`nn-container ${s.wide}`}>
        <div className={s.head}>
          <h1 className={s.title}>{t('tickets.title')}</h1>
          <p className={s.lead}>{t('tickets.lead')}</p>
        </div>
        {session.status === 'anon' ? (
          <LoginGate />
        ) : error ? (
          <ErrorBox text={errText(error)} />
        ) : items === null ? (
          <div className={s.center}>
            <div className="nn-spinner" aria-label={t('common.loading')} />
          </div>
        ) : items.length === 0 ? (
          <div className={`${s.card} ${s.center}`}>
            <Icon name="ticket" size={32} />
            <p>{t('tickets.empty')}</p>
            <Link href={`/${locale}/checkout`} className="nn-btn nn-btn--gold">
              {t('tickets.buy')}
            </Link>
          </div>
        ) : (
          <>
            <ul className={s.ticketGrid}>
              {items.map((tk) => (
                <TicketCard key={tk.ticketId} ticket={tk} />
              ))}
            </ul>
            <p className={s.secure}>
              <Icon name="info" size={16} />
              {t('tickets.brightness')}
            </p>
          </>
        )}
      </div>
    </section>
  );
}
