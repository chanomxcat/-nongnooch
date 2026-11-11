import type { NextResponse } from 'next/server';

export const AT = 'nn_at';
export const RT = 'nn_rt';
export type Tokens = { access: string; refresh: string; expiresIn: number };

const secure = () => (process.env.SITE_URL ?? '').startsWith('https://');

/** httpOnly, scoped to /bff so page JavaScript never sees the tokens. */
export function setSessionCookies(res: NextResponse, t: Tokens) {
  const base = { httpOnly: true, sameSite: 'lax' as const, path: '/bff', secure: secure() };
  res.cookies.set(AT, t.access, { ...base, maxAge: t.expiresIn > 0 ? t.expiresIn : 900 });
  res.cookies.set(RT, t.refresh, { ...base, maxAge: 30 * 24 * 3600 });
}

export function clearSessionCookies(res: NextResponse) {
  for (const n of [AT, RT]) res.cookies.set(n, '', { httpOnly: true, path: '/bff', maxAge: 0 });
}
