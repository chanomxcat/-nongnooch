import { NextResponse, type NextRequest } from 'next/server';
import { csrfFail } from '@/lib/session/csrf';
import { baseHeaders, gatewayUrl } from '@/lib/session/upstream';

export const dynamic = 'force-dynamic';

export async function POST(req: NextRequest) {
  const bad = csrfFail(req);
  if (bad) return bad;
  const h = baseHeaders(req);
  h.set('Content-Type', 'application/json');
  const r = await fetch(`${gatewayUrl()}/api/identity/auth/otp`, { method: 'POST', headers: h, body: await req.text(), cache: 'no-store' });
  const res = new NextResponse(await r.text(), { status: r.status, headers: { 'Content-Type': 'application/json' } });
  const ra = r.headers.get('retry-after');
  if (ra) res.headers.set('Retry-After', ra);
  return res;
}
