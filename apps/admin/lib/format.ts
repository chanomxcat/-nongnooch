/** satang to "฿1,234.50" (drops decimals when whole). */
export function formatBaht(satang: number): string {
  const baht = (Number.isFinite(satang) ? satang : 0) / 100;
  const whole = Number.isInteger(baht);
  return '฿' + baht.toLocaleString('en-US', { minimumFractionDigits: whole ? 0 : 2, maximumFractionDigits: 2 });
}

const hm = new Intl.DateTimeFormat('en-GB', { timeZone: 'Asia/Bangkok', hour: '2-digit', minute: '2-digit', hour12: false });
const hms = new Intl.DateTimeFormat('en-GB', { timeZone: 'Asia/Bangkok', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });

export function timeHHmm(iso: string | Date | undefined): string {
  const d = iso instanceof Date ? iso : new Date(iso ?? '');
  return Number.isNaN(d.getTime()) ? '--:--' : hm.format(d);
}
export const timeHHmmss = (d: Date) => hms.format(d);

export const num = (n: number | undefined) => (typeof n === 'number' ? n.toLocaleString('en-US') : '—');
