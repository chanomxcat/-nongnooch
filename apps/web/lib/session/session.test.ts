import { describe, expect, it } from 'vitest';
import { upstreamPath } from './bffPath';
import { forwardedFor } from './upstream';

const h = (xff?: string) => ({ get: (n: string) => (n.toLowerCase() === 'x-forwarded-for' ? (xff ?? null) : null) });

describe('bff upstream path', () => {
  it('builds /api/<svc>/… for normal segments', () => {
    expect(upstreamPath('commerce', ['orders', '0193-ab', 'payments'])).toEqual({ ok: true, path: '/api/commerce/orders/0193-ab/payments' });
    expect(upstreamPath('member', ['members', 'me', 'points'])).toEqual({ ok: true, path: '/api/member/members/me/points' });
  });
  it('rejects unknown services and internal routes', () => {
    expect(upstreamPath('identity', ['auth', 'otp'])).toEqual({ ok: false, status: 404 });
    expect(upstreamPath('member', ['InTeRnAl', 'seed'])).toEqual({ ok: false, status: 404 });
    expect(upstreamPath('member', ['%69nternal', 'seed'])).toEqual({ ok: false, status: 404 });
  });
  it.each([
    [['..', 'identity', 'auth']],
    [['.', 'orders']],
    [['%2e%2e', 'identity']],
    [['%2E%2e', 'identity']],
    [['%252e%252e', 'identity']],
    [['a%2Fb']],
    [['a%5Cb']],
    [['..%2F..%2Fidentity']],
    [['']],
    [['%E0%A4%A']],
    [[]],
  ])('rejects traversal / malformed segments %j', (segs) => {
    expect(upstreamPath('commerce', segs as string[])).toEqual({ ok: false, status: 400 });
  });
});

describe('X-Forwarded-For', () => {
  it('never forwards a client header when no trusted proxy fronts web', () => {
    expect(forwardedFor(h('6.6.6.6'), false)).toBeNull();
    expect(forwardedFor(h(), false)).toBeNull();
  });
  it('behind a trusted proxy keeps only the right-most (proxy-appended) entry', () => {
    expect(forwardedFor(h('6.6.6.6, 203.0.113.9'), true)).toBe('203.0.113.9');
    expect(forwardedFor(h('2001:db8::1'), true)).toBe('2001:db8::1');
    expect(forwardedFor(h('1.2.3.4, evil<script>'), true)).toBeNull();
    expect(forwardedFor(h(), true)).toBeNull();
  });
});
