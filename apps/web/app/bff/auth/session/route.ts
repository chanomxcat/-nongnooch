import { NextResponse, type NextRequest } from 'next/server';
import { AT, clearSessionCookies, RT, setSessionCookies } from '@/lib/session/cookies';
import { decodeClaims, isFresh, type Claims } from '@/lib/session/jwt';
import { refreshOnce } from '@/lib/session/refresh';

export const dynamic = 'force-dynamic';

export async function GET(req: NextRequest) {
  let claims = decodeClaims(req.cookies.get(AT)?.value);
  if (isFresh(claims)) return NextResponse.json(view(claims));
  const rt = req.cookies.get(RT)?.value;
  if (!rt) return NextResponse.json({ loggedIn: false, memberId: null });
  const tokens = await refreshOnce(req, rt);
  if (!tokens) {
    const res = NextResponse.json({ loggedIn: false, memberId: null });
    clearSessionCookies(res);
    return res;
  }
  claims = decodeClaims(tokens.access);
  const res = NextResponse.json(view(claims));
  setSessionCookies(res, tokens);
  return res;
}

function view(c: Claims | null) {
  return { loggedIn: !!c, memberId: c?.mid || null, lang: c?.lang ?? null, contactType: c?.ctype ?? null };
}
