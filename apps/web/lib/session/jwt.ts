export type Claims = { sub: string; ctype?: string; mid?: string; lang?: string; exp: number; typ?: string };

/** Decode only (display / routing). Signature checks belong to the gateway. */
export function decodeClaims(token: string | undefined | null): Claims | null {
  if (!token) return null;
  const part = token.split('.')[1];
  if (!part) return null;
  try {
    const json = Buffer.from(part.replace(/-/g, '+').replace(/_/g, '/'), 'base64').toString('utf8');
    const c = JSON.parse(json);
    return typeof c.exp === 'number' && typeof c.sub === 'string' ? c : null;
  } catch {
    return null;
  }
}

export const isFresh = (c: Claims | null, nowSec = Date.now() / 1000) => !!c && c.exp - 15 > nowSec;
