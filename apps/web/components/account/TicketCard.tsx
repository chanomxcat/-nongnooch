'use client';

import { useState } from 'react';
import { Icon } from '@/components/ui/Icon';
import { useI18n } from '@/components/providers/Providers';
import type { Ticket } from '@/lib/api/client';
import { formatVisitDate, maskId } from '@/lib/format';
import { QrCode } from './QrCode';
import s from './account.module.css';

const DEMO_TOOLS = process.env.NEXT_PUBLIC_DEMO_TOOLS === 'true';

/** Demo only: copies the QR string so the staff gate page (paste field) can be shown on a laptop without a second camera. */
function CopyQr({ qr }: { qr: string }) {
  const [done, setDone] = useState(false);
  return (
    <button
      type="button"
      className="nn-btn nn-btn--block"
      data-copy-qr
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(qr);
          setDone(true);
          setTimeout(() => setDone(false), 2000);
        } catch {
          window.prompt('Copy QR', qr);
        }
      }}
    >
      {done ? 'คัดลอกแล้ว · Copied' : 'คัดลอกรหัส QR (เดโม) · Copy QR (demo)'}
    </button>
  );
}

export function TicketCard({ ticket }: { ticket: Ticket }) {
  const { t, locale } = useI18n();
  const active = ticket.status === 'active';
  const kindKey = `tickets.kind.${ticket.kind}`;
  const kind = t(kindKey) === kindKey ? ticket.kind : t(kindKey);
  const statusKey = `tickets.status.${ticket.status}`;
  return (
    <li className={s.ticket} data-ticket-id={ticket.ticketId}>
      <div className={s.ticketTop}>
        <span className={s.ticketKind}>
          <Icon name={ticket.kind === 'annual' ? 'infinity' : 'ticket'} size={22} />
          {kind}
        </span>
        <span className={`${s.status} ${active ? '' : s.statusMuted}`}>{t(statusKey) === statusKey ? ticket.status : t(statusKey)}</span>
      </div>
      <div className={s.ticketBody}>
        {ticket.qr ? (
          <QrCode value={ticket.qr} label={t('tickets.qrAlt', { name: ticket.holderName })} className={`${s.qr} ${active ? '' : s.qrUsed}`} />
        ) : (
          <div className={s.qr} />
        )}
        <dl className={s.facts}>
          <div>
            <dt>{t('tickets.holder')}</dt>
            <dd>{ticket.holderName || '–'}</dd>
          </div>
          <div>
            <dt>{t('tickets.id')}</dt>
            <dd>{ticket.idLast4 ? maskId(ticket.idLast4) : '–'}</dd>
          </div>
          <div>
            <dt>{t('tickets.visitDate')}</dt>
            <dd>{ticket.visitDate ? formatVisitDate(ticket.visitDate, locale, { dateStyle: 'medium' }) : t('tickets.anyDay')}</dd>
          </div>
        </dl>
        <span className={s.ticketId}>{ticket.ticketId}</span>
        {DEMO_TOOLS && ticket.qr && <CopyQr qr={ticket.qr} />}
      </div>
    </li>
  );
}
