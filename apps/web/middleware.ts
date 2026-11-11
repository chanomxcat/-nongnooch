import { NextResponse, type NextRequest } from 'next/server';
import { isLocale, negotiate } from './lib/locale';
import { normalizeInvite } from './lib/invite';

const INVITE_COOKIE = 'nn_invite';
const DEVICE_COOKIE = 'nn_did';

export function middleware(req: NextRequest) {
  const { pathname, searchParams } = req.nextUrl;
  const seg = pathname.split('/');
  const first = seg[1];

  // No locale prefix (e.g. /tickets from a push link, /r/CODE from a guide QR): pick one from Accept-Language.
  if (!isLocale(first)) {
    const url = req.nextUrl.clone();
    url.pathname = `/${negotiate(req.headers.get('accept-language'))}${pathname === '/' ? '' : pathname}`;
    return withDevice(req, NextResponse.redirect(url));
  }

  let res: NextResponse;
  let invite: string | null = null;
  if (seg[2] === 'r' && seg[3]) {
    // /<l>/r/{inviteCode} → remember the code, continue to registration
    invite = normalizeInvite(decodeURIComponent(seg[3]));
    const url = req.nextUrl.clone();
    url.pathname = `/${first}/register`;
    url.search = '';
    res = NextResponse.redirect(url);
  } else {
    invite = normalizeInvite(searchParams.get('invite'));
    res = NextResponse.next();
  }
  if (invite) {
    res.cookies.set(INVITE_COOKIE, invite, { path: '/', maxAge: 30 * 24 * 3600, sameSite: 'lax', httpOnly: false });
  }
  return withDevice(req, res);
}

function withDevice(req: NextRequest, res: NextResponse) {
  if (!req.cookies.get(DEVICE_COOKIE)) {
    res.cookies.set(DEVICE_COOKIE, crypto.randomUUID(), { path: '/', maxAge: 2 * 365 * 24 * 3600, sameSite: 'lax', httpOnly: true });
  }
  return res;
}

export const config = {
  matcher: ['/((?!bff|_next|healthz|icons.svg|fonts|images|favicon.ico|.*\\..*).*)'],
};
