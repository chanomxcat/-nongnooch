import type { NextResponse } from 'next/server';

export const STAFF = 'nn_staff';
const secure = () => (process.env.SITE_URL ?? '').startsWith('https://');

/** httpOnly; the staff JWT never reaches page JavaScript. */
export function setStaffCookie(res: NextResponse, access: string, expiresIn: number) {
  res.cookies.set(STAFF, access, {
    httpOnly: true,
    sameSite: 'lax',
    path: '/',
    secure: secure(),
    maxAge: expiresIn > 0 ? expiresIn : 8 * 3600,
  });
}

export function clearStaffCookie(res: NextResponse) {
  res.cookies.set(STAFF, '', { httpOnly: true, path: '/', maxAge: 0 });
}
