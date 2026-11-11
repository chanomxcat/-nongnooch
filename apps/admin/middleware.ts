import { NextResponse, type NextRequest } from 'next/server';

/** Cheap gate: no staff cookie, no staff pages. Expiry/role checks happen server-side in the (staff) layout. */
export function middleware(req: NextRequest) {
  const { pathname } = req.nextUrl;
  if (pathname === '/') return NextResponse.redirect(new URL('/dashboard', req.url));
  if (!req.cookies.get('nn_staff')?.value) {
    const u = new URL('/login', req.url);
    u.searchParams.set('next', pathname);
    return NextResponse.redirect(u);
  }
  return NextResponse.next();
}

export const config = { matcher: ['/', '/gate', '/dashboard'] };
