import type { NextRequest } from 'next/server';

export const gatewayUrl = () => (process.env.GATEWAY_URL ?? 'http://gateway:8080').replace(/\/$/, '');

type HeaderSource = { get(name: string): string | null };

/**
 * The browser's own X-Forwarded-For is never forwarded as-is (spoofable). Only behind a trusted
 * reverse proxy (TRUST_PROXY_HEADERS=true) do we pass its right-most entry; otherwise none.
 * x-real-ip is ignored.
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

export function baseHeaders(req: NextRequest, access?: string | null): Headers {
  const h = new Headers();
  const xff = forwardedFor(req.headers);
  if (xff) h.set('X-Forwarded-For', xff);
  if (access) h.set('Authorization', `Bearer ${access}`);
  return h;
}
