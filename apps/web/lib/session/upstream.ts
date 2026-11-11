import type { NextRequest } from 'next/server';

export const gatewayUrl = () => (process.env.GATEWAY_URL ?? 'http://gateway:8080').replace(/\/$/, '');

/** Client IP chain for the gateway (contracts §2): keep the incoming XFF; Next adds the peer address to it. */
export function forwardedFor(req: NextRequest): string | null {
  const xff = req.headers.get('x-forwarded-for');
  const real = req.headers.get('x-real-ip');
  if (xff) return xff;
  return real ?? null;
}

/** Headers every BFF → gateway call carries. */
export function baseHeaders(req: NextRequest, access?: string | null): Headers {
  const h = new Headers();
  const xff = forwardedFor(req);
  if (xff) h.set('X-Forwarded-For', xff);
  const al = req.headers.get('accept-language');
  if (al) h.set('Accept-Language', al);
  const did = req.cookies.get('nn_did')?.value;
  if (did) h.set('X-Device-Id', did);
  if (access) h.set('Authorization', `Bearer ${access}`);
  return h;
}
