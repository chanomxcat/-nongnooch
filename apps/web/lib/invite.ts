/** contracts §10: invite codes are [A-Z0-9]{4,32}; strip spaces / dashes and upper-case before checking. */
export function normalizeInvite(raw: string | null | undefined): string | null {
  if (!raw) return null;
  const c = raw.replace(/[\s-]/g, '').toUpperCase();
  return /^[A-Z0-9]{4,32}$/.test(c) ? c : null;
}
