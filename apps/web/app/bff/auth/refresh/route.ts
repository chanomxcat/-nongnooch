import { NextResponse, type NextRequest } from 'next/server';
import { csrfFail } from '@/lib/session/csrf';
import { clearSessionCookies, RT, setSessionCookies } from '@/lib/session/cookies';
import { decodeClaims } from '@/lib/session/jwt';
import { refreshOnce } from '@/lib/session/refresh';

export const dynamic = 'force-dynamic';

/** Called after registration so the new token carries `mid` (contracts section 3). */
export async function POST(req: NextRequest) {
  const bad = csrfFail(req);
  if (bad) return bad;
  const rt = req.cookies.get(RT)?.value;
  const tokens = rt ? await refreshOnce(req, rt) : null;
  if (!tokens) {
    const res = NextResponse.json({ error: 'refresh_invalid' }, { status: 401 });
    clearSessionCookies(res);
    return res;
  }
  const res = NextResponse.json({ memberId: decodeClaims(tokens.access)?.mid || null });
  setSessionCookies(res, tokens);
  return res;
}
