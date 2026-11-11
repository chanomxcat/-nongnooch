import type { Locale } from './locale';

const intlTag = (l: Locale) => (l === 'zh' ? 'zh-Hans' : l === 'th' ? 'th-TH' : l);

/** THB from integer satang (contracts §1). Shows decimals only when needed. */
export function formatTHB(satang: number, locale: Locale): string {
  const baht = satang / 100;
  return new Intl.NumberFormat(intlTag(locale), {
    style: 'currency',
    currency: 'THB',
    minimumFractionDigits: Number.isInteger(baht) ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(baht);
}

export function formatNumber(n: number, locale: Locale): string {
  return new Intl.NumberFormat(intlTag(locale)).format(n);
}

/** YYYY-MM-DD in Asia/Bangkok (visitDate). */
export function todayInBangkok(now: Date = new Date()): string {
  return new Date(now.getTime() + 7 * 3600_000).toISOString().slice(0, 10);
}

export function addDays(ymd: string, days: number): string {
  const d = new Date(`${ymd}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

export function formatVisitDate(ymd: string, locale: Locale, opts: Intl.DateTimeFormatOptions = { dateStyle: 'full' }): string {
  return new Intl.DateTimeFormat(intlTag(locale), { ...opts, timeZone: 'UTC' }).format(new Date(`${ymd}T00:00:00Z`));
}

export function dateParts(ymd: string, locale: Locale) {
  const d = new Date(`${ymd}T00:00:00Z`);
  const f = (o: Intl.DateTimeFormatOptions) => new Intl.DateTimeFormat(intlTag(locale), { ...o, timeZone: 'UTC' }).format(d);
  return { weekday: f({ weekday: 'short' }), day: String(d.getUTCDate()).padStart(2, '0'), month: f({ month: 'short' }) };
}

export function formatDateTime(iso: string, locale: Locale): string {
  return new Intl.DateTimeFormat(intlTag(locale), { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Asia/Bangkok' }).format(new Date(iso));
}

export const maskId = (idLast4: string) => `•••• ${idLast4}`;
