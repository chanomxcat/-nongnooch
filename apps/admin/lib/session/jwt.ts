export type StaffClaims = { sub: string; typ?: string; name?: string; roles?: string[]; exp: number };

/** Decode only (display / routing). Signature checks belong to the gateway. */
export function decodeStaff(token: string | undefined | null): StaffClaims | null {
  if (!token) return null;
  const part = token.split('.')[1];
  if (!part) return null;
  try {
    const c = JSON.parse(Buffer.from(part.replace(/-/g, '+').replace(/_/g, '/'), 'base64').toString('utf8'));
    if (typeof c.exp !== 'number' || typeof c.sub !== 'string') return null;
    return { ...c, roles: Array.isArray(c.roles) ? c.roles.filter((r: unknown) => typeof r === 'string') : [] };
  } catch {
    return null;
  }
}

export const isFresh = (c: StaffClaims | null, nowSec = Date.now() / 1000) => !!c && c.exp - 5 > nowSec;
export const canScan = (roles: string[] = []) => roles.includes('gate') || roles.includes('admin');
