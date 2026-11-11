import { NextResponse, type NextRequest } from 'next/server';
import { STAFF } from '@/lib/session/cookies';
import { upstreamPath } from '@/lib/session/bffPath';
import { csrfFail } from '@/lib/session/csrf';
import { baseHeaders, gatewayUrl } from '@/lib/session/upstream';

export const dynamic = 'force-dynamic';

type Ctx = { params: { svc: string; path: string[] } };

async function handle(req: NextRequest, { params }: Ctx) {
  const up = upstreamPath(params.svc, params.path);
  if (!up.ok) return NextResponse.json({ error: up.status === 404 ? 'not_found' : 'bad_request' }, { status: up.status });
  const bad = csrfFail(req);
  if (bad) return bad;
  const access = req.cookies.get(STAFF)?.value;
  if (!access) return NextResponse.json({ error: 'unauthorized' }, { status: 401 });

  const url = `${gatewayUrl()}${up.path}${req.nextUrl.search}`;
  const h = baseHeaders(req, access);
  const ct = req.headers.get('content-type');
  if (ct) h.set('content-type', ct);
  const body = req.method === 'GET' || req.method === 'HEAD' ? undefined : await req.arrayBuffer();

  try {
    const r = await fetch(url, { method: req.method, headers: h, body, cache: 'no-store', redirect: 'manual' });
    const res = new NextResponse(r.status === 204 ? null : await r.arrayBuffer(), { status: r.status });
    const rct = r.headers.get('content-type');
    if (rct) res.headers.set('content-type', rct);
    res.headers.set('Cache-Control', 'no-store');
    return res;
  } catch {
    return NextResponse.json({ error: 'upstream_unavailable' }, { status: 502 });
  }
}

export { handle as GET, handle as POST };
