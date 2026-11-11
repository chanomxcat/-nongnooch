import { NextResponse, type NextRequest } from 'next/server';
import { clearStaffCookie } from '@/lib/session/cookies';
import { csrfFail } from '@/lib/session/csrf';

export const dynamic = 'force-dynamic';

export function POST(req: NextRequest) {
  const bad = csrfFail(req);
  if (bad) return bad;
  const res = new NextResponse(null, { status: 204 });
  clearStaffCookie(res);
  return res;
}
