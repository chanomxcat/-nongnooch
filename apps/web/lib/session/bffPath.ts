/** Services the browser may reach through the BFF. Identity has its own /bff/auth/* routes. */
export const BFF_SERVICES = new Set(['member', 'commerce', 'ticket', 'content', 'notify']);

export type PathResult = { ok: true; path: string } | { ok: false; status: 400 | 404 };

/** Decode until stable (bounded) so %252e%252e etc. can't hide a dot segment; null on malformed escapes. */
function fullyDecode(seg: string): string | null {
  let cur = seg;
  for (let i = 0; i < 4; i++) {
    let next: string;
    try {
      next = decodeURIComponent(cur);
    } catch {
      return null;
    }
    if (next === cur) return cur;
    cur = next;
  }
  return cur;
}

/**
 * Upstream path for a BFF request, or a rejection. Every segment must decode to a plain name:
 * no empty / "." / ".." segments, no "/" or "\" inside a segment, never "internal". The result is
 * re-checked after URL normalisation so it can only ever point at /api/<svc>/….
 */
export function upstreamPath(svc: string, segments: string[], gatewayBase = 'http://gateway:8080'): PathResult {
  if (!BFF_SERVICES.has(svc)) return { ok: false, status: 404 };
  if (segments.length === 0) return { ok: false, status: 400 };
  const clean: string[] = [];
  for (const raw of segments) {
    const seg = fullyDecode(raw);
    if (seg === null || seg === '' || seg === '.' || seg === '..' || /[/\\]/.test(seg) || /[\u0000-\u001f]/.test(seg)) {
      return { ok: false, status: 400 };
    }
    if (seg.toLowerCase() === 'internal') return { ok: false, status: 404 };
    clean.push(seg);
  }
  const path = `/api/${svc}/${clean.map(encodeURIComponent).join('/')}`;
  const prefix = `/api/${svc}/`;
  if (!new URL(path, gatewayBase).pathname.startsWith(prefix)) return { ok: false, status: 400 };
  return { ok: true, path };
}
