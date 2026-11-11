import { NextResponse, type NextRequest } from 'next/server';
import { csrfFail } from '@/lib/session/csrf';
import { clearSessionCookies, RT } from '@/lib/session/cookies';
import { baseHeaders, gatewayUrl } from '@/lib/session/upstream';

export const dynamic = 'force-dynamic';

export async function POST(req: NextRequest) {
  const bad = csrfFail(req);
  if (bad) return bad;
  const rt = req.cookies.get(RT)?.value;
  if (rt) {
    const h = baseHeaders(req);
    h.set('Content-Type', 'application/json');
    await fetch(`${gatewayUrl()}/api/identity/auth/logout`, { method: 'POST', headers: h, body: JSON.stringify({ refreshToken: rt }) }).catch(() => null);
  }
  const res = new NextResponse(null, { status: 204 });
  clearSessionCookies(res);
  return res;
}
