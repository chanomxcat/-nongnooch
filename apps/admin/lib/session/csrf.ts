import { NextResponse, type NextRequest } from 'next/server';

/** Non-GET BFF calls must come from our own pages: custom header + same-origin Origin. */
export function csrfFail(req: NextRequest): NextResponse | null {
  if (req.method === 'GET' || req.method === 'HEAD') return null;
  const okHeader = req.headers.get('x-requested-with') === 'nn-web';
  const origin = req.headers.get('origin');
  const host = req.headers.get('x-forwarded-host') ?? req.headers.get('host');
  let okOrigin = true;
  if (origin) {
    try {
      okOrigin = new URL(origin).host === host;
    } catch {
      okOrigin = false;
    }
  }
  return okHeader && okOrigin ? null : NextResponse.json({ error: 'csrf_failed' }, { status: 403 });
}
