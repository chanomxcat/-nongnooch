import type { NextRequest } from 'next/server';

export const gatewayUrl = () => (process.env.GATEWAY_URL ?? 'http://gateway:8080').replace(/\/$/, '');

type HeaderSource = { get(name: string): string | null };

/**
 * Client IP chain for the gateway (contracts section 2). The browser's own X-Forwarded-For is never
 * forwarded as-is (it is spoofable). Only when a trusted reverse proxy sits in front of `web`
 * (TRUST_PROXY_HEADERS=true, e.g. Caddy on the demo host) do we pass the right-most entry, which is
 * the address that proxy appended. Otherwise no XFF is sent and the gateway sees the web container.
 */
export function forwardedFor(headers: HeaderSource, trustProxy = process.env.TRUST_PROXY_HEADERS === 'true'): string | null {
  if (!trustProxy) return null;
  const parts = (headers.get('x-forwarded-for') ?? '')
    .split(',')
    .map((p) => p.trim())
    .filter(Boolean);
  const last = parts.at(-1);
  return last && /^[0-9A-Fa-f:.]{2,45}$/.test(last) ? last : null;
}

/** Headers every BFF → gateway call carries. */
export function baseHeaders(req: NextRequest, access?: string | null): Headers {
  const h = new Headers();
  const xff = forwardedFor(req.headers);
  if (xff) h.set('X-Forwarded-For', xff);
  const al = req.headers.get('accept-language');
  if (al) h.set('Accept-Language', al);
  const did = req.cookies.get('nn_did')?.value;
  if (did) h.set('X-Device-Id', did);
  if (access) h.set('Authorization', `Bearer ${access}`);
  return h;
}
