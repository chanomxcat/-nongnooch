import { NextResponse, type NextRequest } from 'next/server';
import { csrfFail } from '@/lib/session/csrf';
import { setSessionCookies, type Tokens } from '@/lib/session/cookies';
import { decodeClaims } from '@/lib/session/jwt';
import { baseHeaders, gatewayUrl } from '@/lib/session/upstream';

export const dynamic = 'force-dynamic';

export async function POST(req: NextRequest) {
  const bad = csrfFail(req);
  if (bad) return bad;
  const h = baseHeaders(req);
  h.set('Content-Type', 'application/json');
  const r = await fetch(`${gatewayUrl()}/api/identity/auth/verify`, { method: 'POST', headers: h, body: await req.text(), cache: 'no-store' });
  if (!r.ok) return new NextResponse(await r.text(), { status: r.status, headers: { 'Content-Type': 'application/json' } });
  const tokens = (await r.json()) as Tokens;
  const res = NextResponse.json({ memberId: decodeClaims(tokens.access)?.mid || null });
  setSessionCookies(res, tokens);
  return res;
}
