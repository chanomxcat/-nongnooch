'use client';

import Link from 'next/link';
import { useEffect, useRef, useState } from 'react';
import { TicketCard } from '@/components/account/TicketCard';
import { ErrorBox } from '@/components/auth/ErrorBox';
import { Icon } from '@/components/ui/Icon';
import { useErrorText, useI18n } from '@/components/providers/Providers';
import { ApiError, commerce, ticket, type Order, type Ticket } from '@/lib/api/client';
import { formatTHB } from '@/lib/format';
import { advanceAttempt, classify, currentAttempt, needsTickets, pollDelay, retryKey, ticketsForOrder, type View } from './resultState';
import { clearPersisted } from './state';
import s from '@/components/account/account.module.css';

const session = () => {
  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
};

/**
 * Return page from mockext: poll the order (2 s, then 4 s), then this order's tickets. A slow webhook is
 * never treated as a failure; payment retry is offered only when commerce reports the payment failed.
 */
export function OrderResult({ orderId }: { orderId: string }) {
  const { t, locale } = useI18n();
  const errText = useErrorText();
  const [view, setView] = useState<View>('checking');
  const [order, setOrder] = useState<Order | null>(null);
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [retrying, setRetrying] = useState(false);
  const started = useRef(Date.now());

  useEffect(() => {
    let live = true;
    let timer: ReturnType<typeof setTimeout>;
    const tick = async () => {
      const elapsed = Date.now() - started.current;
      try {
        const o = await commerce.order(orderId);
        if (!live) return;
        setOrder(o);
        let mine: Ticket[] = [];
        if (needsTickets(o)) {
          mine = ticketsForOrder(await ticket.mine().catch(() => [] as Ticket[]), orderId);
          if (!live) return;
          setTickets(mine);
        }
        const next = classify(o, mine, elapsed);
        setView(next.view);
        if (next.view === 'success') clearPersisted();
        if (next.done) return;
      } catch (e) {
        if (!live) return;
        const code = (e as ApiError).code;
        if (code !== 'network') {
          setError(code);
          setView('error');
          return;
        }
      }
      timer = setTimeout(tick, pollDelay(elapsed));
    };
    void tick();
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [orderId]);

  async function retry() {
    if (retrying) return;
    setRetrying(true);
    setError(null);
    const store = session();
    try {
      const url = await commerce.pay(
        orderId,
        'card',
        `${window.location.origin}/${locale}/checkout/result/${orderId}`,
        retryKey(orderId, currentAttempt(orderId, store)),
      );
      advanceAttempt(orderId, store);
      window.location.assign(url);
    } catch (e) {
      setError((e as ApiError).code);
      setRetrying(false);
    }
  }

  const title: Record<View, string> = {
    checking: t('result.checking'),
    confirming: t('result.confirming'),
    issuing: t('result.issuing'),
    slow: t('result.issuing'),
    success: t('result.success'),
    failed: t('result.failed'),
    expired: t('result.expired'),
    error: errText(error),
  };

  return (
    <section className={s.page}>
      <div className={`nn-container ${s.wide}`}>
        <div className={`${s.center} ${s.head}`} role="status" aria-live="polite">
          {view === 'success' ? (
            <span className={s.successIcon}>
              <Icon name="check" size={40} />
            </span>
          ) : view === 'failed' || view === 'expired' || view === 'error' ? (
            <span className={s.failIcon}>
              <Icon name="alert" size={40} />
            </span>
          ) : (
            <div className="nn-spinner" />
          )}
          <h1 className={s.title}>{title[view]}</h1>
          {view === 'success' && <p className={s.lead}>{t('result.successLead')}</p>}
          {view === 'confirming' && <p className={s.lead}>{t('result.confirmingLead')}</p>}
          {view === 'slow' && <p className={s.lead}>{t('result.issuingLong')}</p>}
          {view === 'failed' && <p className={s.lead}>{t('result.failedLead')}</p>}
          {order?.orderNo && (
            <p className={s.badge} style={{ justifySelf: 'center' }}>
              {t('checkout.orderNo', { no: order.orderNo })}
              {order.totalSatang !== undefined && ` · ${formatTHB(order.totalSatang, locale)}`}
            </p>
          )}
        </div>

        {view === 'success' && tickets.length > 0 && (
          <ul className={s.ticketGrid} style={{ marginBottom: 32 }}>
            {tickets.map((tk) => (
              <TicketCard key={tk.ticketId} ticket={tk} />
            ))}
          </ul>
        )}

        {error && view === 'failed' && <ErrorBox text={errText(error)} />}

        <div className={s.actions}>
          {view === 'failed' && (
            <button type="button" className="nn-btn nn-btn--gold" onClick={retry} disabled={retrying}>
              {t('result.retry')}
            </button>
          )}
          {view === 'expired' && (
            <Link href={`/${locale}/checkout`} className="nn-btn nn-btn--gold">
              {t('checkout.startOver')}
            </Link>
          )}
          {(view === 'success' || view === 'slow' || view === 'issuing') && (
            <Link href={`/${locale}/tickets`} className="nn-btn nn-btn--dark">
              <Icon name="qr" size={20} />
              {t('result.viewTickets')}
            </Link>
          )}
          <Link href={`/${locale}`} className="nn-btn nn-btn--outline-dark">
            {t('result.home')}
          </Link>
        </div>
      </div>
    </section>
  );
}
