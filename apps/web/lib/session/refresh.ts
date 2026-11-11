import type { NextRequest } from 'next/server';
import type { Tokens } from './cookies';
import { baseHeaders, gatewayUrl } from './upstream';

const inflight = new Map<string, Promise<Tokens | null>>();

/**
 * Single-flight refresh per refresh token: concurrent 401s share one /auth/refresh call,
 * so identity never sees the same refresh token twice (which would revoke the session).
 */
export function refreshOnce(req: NextRequest, rt: string): Promise<Tokens | null> {
  let p = inflight.get(rt);
  if (!p) {
    p = (async () => {
      const h = baseHeaders(req);
      h.set('Content-Type', 'application/json');
      const r = await fetch(`${gatewayUrl()}/api/identity/auth/refresh`, {
        method: 'POST',
        headers: h,
        body: JSON.stringify({ refreshToken: rt }),
        cache: 'no-store',
      });
      return r.ok ? ((await r.json()) as Tokens) : null;
    })().finally(() => setTimeout(() => inflight.delete(rt), 5_000));
    inflight.set(rt, p);
  }
  return p;
}
