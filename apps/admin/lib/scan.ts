export type ScanApi = {
  result?: string;
  kind?: string;
  holderName?: string;
  idLast4?: string;
  visitDate?: string;
  usedAt?: string;
};

export type Tone = 'ok' | 'used' | 'warn' | 'bad';
export type ScanView = { tone: Tone; title: string; en: string; lines: string[] };

const KIND: Record<string, string> = { day: 'บัตรรายวัน / Day Pass', annual: 'บัตรรายปี / Annual Pass' };
export const kindLabel = (k?: string) => (k ? KIND[k] ?? k : '');

/** Maps the ticket service verdict to what the big result panel shows. `hhmm` formats usedAt (Bangkok). */
export function mapScan(r: ScanApi, hhmm: (iso?: string) => string): ScanView {
  switch (r.result) {
    case 'ok':
      return {
        tone: 'ok',
        title: 'ผ่าน',
        en: 'PASS',
        lines: [r.holderName ?? '', r.idLast4 ? `•••• ${r.idLast4}` : '', kindLabel(r.kind), r.visitDate ? `วันเข้าชม ${r.visitDate}` : ''].filter(Boolean),
      };
    case 'used':
      return { tone: 'used', title: 'ใช้แล้ว', en: 'ALREADY USED', lines: [`ใช้ไปเมื่อ ${hhmm(r.usedAt)}`, r.holderName ?? ''].filter(Boolean) };
    case 'wrong_date':
      return { tone: 'warn', title: 'ไม่ใช่วันเข้าชม', en: 'WRONG DATE', lines: r.visitDate ? [`วันเข้าชม ${r.visitDate}`] : [] };
    default:
      return { tone: 'bad', title: 'บัตรไม่ถูกต้อง', en: 'INVALID TICKET', lines: [] };
  }
}

/** Ignores the same QR within `windowMs`. Returns true when the scan should be processed. */
export function createDebouncer(windowMs = 3000, now: () => number = Date.now) {
  const seen = new Map<string, number>();
  return (qr: string): boolean => {
    const t = now();
    for (const [k, at] of seen) if (t - at >= windowMs) seen.delete(k);
    if (seen.has(qr)) return false;
    seen.set(qr, t);
    return true;
  };
}
