/** Server clock offset from the HTTP Date header, so a wrong device clock doesn't break the hold timer. */
export function serverOffset(dateHeader: string | null, clientNowMs: number): number {
  if (!dateHeader) return 0;
  const server = Date.parse(dateHeader);
  return Number.isNaN(server) ? 0 : server - clientNowMs;
}

export function remainingMs(expiresAt: string, offsetMs: number, clientNowMs: number): number {
  const exp = Date.parse(expiresAt);
  if (Number.isNaN(exp)) return 0;
  return Math.max(0, exp - (clientNowMs + offsetMs));
}

export function mmss(ms: number): string {
  const total = Math.ceil(ms / 1000);
  return `${String(Math.floor(total / 60)).padStart(2, '0')}:${String(total % 60).padStart(2, '0')}`;
}
