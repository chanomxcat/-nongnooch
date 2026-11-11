import { NextResponse, type NextRequest } from 'next/server';
import { AT, RT, setSessionCookies, type Tokens } from '@/lib/session/cookies';
import { csrfFail } from '@/lib/session/csrf';
import { decodeClaims, isFresh } from '@/lib/session/jwt';
import { refreshOnce } from '@/lib/session/refresh';
import { baseHeaders, gatewayUrl } from '@/lib/session/upstream';

export const dynamic = 'force-dynamic';

/** Services the browser may reach through the BFF. Identity has its own /bff/auth/* routes. */
const ALLOWED = new Set(['member', 'commerce', 'ticket', 'content', 'notify']);
const PASS_HEADERS = ['content-type', 'idempotency-key', 'x-queue-token', 'x-queue-ticket', 'x-order-token'];

type Ctx = { params: { svc: string; path: string[] } };

async function handle(req: NextRequest, { params }: Ctx) {
  if (!ALLOWED.has(params.svc) || params.path.some((s) => s.toLowerCase() === 'internal')) {
    return NextResponse.json({ error: 'not_found' }, { status: 404 });
  }
  const bad = csrfFail(req);
  if (bad) return bad;

  const url = `${gatewayUrl()}/api/${params.svc}/${params.path.map(encodeURIComponent).join('/')}${req.nextUrl.search}`;
  const body = req.method === 'GET' || req.method === 'HEAD' ? undefined : await req.arrayBuffer();

  let access = req.cookies.get(AT)?.value ?? null;
  const rt = req.cookies.get(RT)?.value;
  let refreshed: Tokens | null = null;
  // Expired access token: refresh up-front instead of spending a request on a certain 401.
  if (rt && !isFresh(decodeClaims(access))) {
    refreshed = await refreshOnce(req, rt);
    access = refreshed?.access ?? null;
  }

  const call = (token: string | null) => {
    const h = baseHeaders(req, token);
    for (const name of PASS_HEADERS) {
      const v = req.headers.get(name);
      if (v) h.set(name, v);
    }
    return fetch(url, { method: req.method, headers: h, body, cache: 'no-store', redirect: 'manual' });
  };

  let r = await call(access);
  if (r.status === 401 && rt && !refreshed) {
    refreshed = await refreshOnce(req, rt);
    if (refreshed) r = await call(refreshed.access);
  }

  const res = new NextResponse(r.status === 204 ? null : await r.arrayBuffer(), { status: r.status });
  for (const name of ['content-type', 'date', 'retry-after', 'x-request-id']) {
    const v = r.headers.get(name);
    if (v) res.headers.set(name, v);
  }
  res.headers.set('Cache-Control', 'no-store');
  if (refreshed) setSessionCookies(res, refreshed);
  return res;
}

export { handle as GET, handle as POST, handle as PUT, handle as PATCH, handle as DELETE };
