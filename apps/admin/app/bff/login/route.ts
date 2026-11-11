import { NextResponse, type NextRequest } from 'next/server';
import { setStaffCookie } from '@/lib/session/cookies';
import { csrfFail } from '@/lib/session/csrf';
import { baseHeaders, gatewayUrl } from '@/lib/session/upstream';

export const dynamic = 'force-dynamic';

export async function POST(req: NextRequest) {
  const bad = csrfFail(req);
  if (bad) return bad;
  const { email, password } = (await req.json().catch(() => ({}))) as { email?: string; password?: string };
  const h = baseHeaders(req);
  h.set('Content-Type', 'application/json');
  let r: Response;
  try {
    r = await fetch(`${gatewayUrl()}/api/identity/auth/staff/login`, {
      method: 'POST',
      headers: h,
      body: JSON.stringify({ email: String(email ?? '').trim(), password: String(password ?? '') }),
      cache: 'no-store',
    });
  } catch {
    return NextResponse.json({ error: 'upstream_unavailable' }, { status: 502 });
  }
  if (!r.ok) {
    const res = NextResponse.json({ error: r.status === 401 ? 'invalid_credentials' : `status_${r.status}` }, { status: r.status });
    const ra = r.headers.get('retry-after');
    if (ra) res.headers.set('retry-after', ra);
    return res;
  }
  const t = (await r.json()) as { access: string; expiresIn: number };
  const res = NextResponse.json({ ok: true });
  setStaffCookie(res, t.access, t.expiresIn);
  return res;
}
