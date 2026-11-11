'use client';

import Link from 'next/link';
import { useEffect, useRef, useState } from 'react';
import { TicketCard } from '@/components/account/TicketCard';
import { ErrorBox } from '@/components/auth/ErrorBox';
import { Icon } from '@/components/ui/Icon';
import { useErrorText, useI18n } from '@/components/providers/Providers';
import { ApiError, commerce, newKey, ticket, type Order, type Ticket } from '@/lib/api/client';
import { formatTHB } from '@/lib/format';
import { clearPersisted } from './state';
import s from '@/components/account/account.module.css';

const POLL_MS = 2000;
const MAX_MS = 60_000;
/** The KBank webhook lands within a second or two; still unpaid after this means the payment did not go through. */
const PENDING_GIVE_UP_MS = 12_000;

type View = 'checking' | 'issuing' | 'success' | 'slow' | 'failed' | 'expired' | 'error';

const isPaid = (o: Order) => ['confirmed', 'paid', 'completed'].includes(o.status) || o.paymentStatus === 'paid';
const isFailed = (o: Order) => ['failed', 'cancelled'].includes(o.paymentStatus) || o.status === 'payment_failed';
const isExpired = (o: Order) => ['expired', 'cancelled'].includes(o.status);

/** Return page from mockext: poll the order, then the tickets, every 2 s for up to 60 s (Plan 13 Task 6). */
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
      try {
        const o = await commerce.order(orderId);
        if (!live) return;
        setOrder(o);
        if (isPaid(o)) {
          const all = await ticket.mine().catch(() => [] as Ticket[]);
          if (!live) return;
          const mine = all.filter((tk) =>
            tk.orderId || tk.orderNo ? tk.orderId === orderId || (!!o.orderNo && tk.orderNo === o.orderNo) : !!o.visitDate && tk.visitDate === o.visitDate,
          );
          setTickets(mine);
          const complete = mine.length > 0 && (!o.ticketCount || mine.length >= o.ticketCount);
          if (complete) {
            clearPersisted();
            setView('success');
            return;
          }
          setView('issuing');
        } else if (isFailed(o)) {
          setView('failed');
          return;
        } else if (isExpired(o)) {
          setView('expired');
          return;
        } else if (Date.now() - started.current > PENDING_GIVE_UP_MS) {
          // commerce keeps a failed / cancelled payment as a pending order until the hold expires
          setView('failed');
          return;
        }
      } catch (e) {
        if (!live) return;
        const code = (e as ApiError).code;
        if (code !== 'network') {
          setError(code);
          setView('error');
          return;
        }
      }
      if (Date.now() - started.current > MAX_MS) {
        setView((v) => (v === 'issuing' ? 'slow' : v === 'checking' ? 'slow' : v));
        return;
      }
      timer = setTimeout(tick, POLL_MS);
    };
    void tick();
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [orderId]);

  async function retry() {
    setRetrying(true);
    try {
      const url = await commerce.pay(orderId, 'card', `${window.location.origin}/${locale}/checkout/result/${orderId}`, newKey());
      window.location.assign(url);
    } catch (e) {
      setError((e as ApiError).code);
      setRetrying(false);
    }
  }

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
          <h1 className={s.title}>
            {view === 'success'
              ? t('result.success')
              : view === 'issuing'
                ? t('result.issuing')
                : view === 'slow'
                  ? t('result.issuing')
                  : view === 'failed'
                    ? t('result.failed')
                    : view === 'expired'
                      ? t('result.expired')
                      : view === 'error'
                        ? errText(error)
                        : t('result.checking')}
          </h1>
          {view === 'success' && <p className={s.lead}>{t('result.successLead')}</p>}
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
