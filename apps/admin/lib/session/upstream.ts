import type { NextRequest } from 'next/server';

export const gatewayUrl = () => (process.env.GATEWAY_URL ?? 'http://gateway:8080').replace(/\/$/, '');

export function baseHeaders(req: NextRequest, access?: string | null): Headers {
  const h = new Headers();
  const xff = req.headers.get('x-forwarded-for') ?? req.headers.get('x-real-ip');
  if (xff) h.set('X-Forwarded-For', xff);
  if (access) h.set('Authorization', `Bearer ${access}`);
  return h;
}
