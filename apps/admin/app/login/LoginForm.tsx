'use client';

import { useState } from 'react';
import { safeNext } from '@/lib/safeNext';

export default function LoginForm({ next }: { next?: string }) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      const r = await fetch('/bff/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'nn-web' },
        body: JSON.stringify({ email, password }),
      });
      if (r.ok) {
        window.location.assign(safeNext(next));
        return;
      }
      if (r.status === 401) setError('อีเมลหรือรหัสผ่านไม่ถูกต้อง');
      else if (r.status === 429) setError('ลองผิดหลายครั้งเกินไป กรุณารอสักครู่ / Too many attempts, wait a moment');
      else if (r.status === 403) setError('บัญชีนี้ถูกปิดใช้งาน / Account disabled');
      else setError('ระบบขัดข้อง กรุณาลองใหม่ / Something went wrong, try again');
    } catch {
      setError('เชื่อมต่อไม่ได้ กรุณาลองใหม่ / Network error');
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} style={{ display: 'contents' }}>
      <label className="field">
        อีเมล / Email
        <input className="in" type="email" autoComplete="username" required value={email} onChange={(e) => setEmail(e.target.value)} />
      </label>
      <label className="field">
        รหัสผ่าน / Password
        <input className="in" type="password" autoComplete="current-password" required value={password} onChange={(e) => setPassword(e.target.value)} />
      </label>
      {error && (
        <div className="alert" role="alert">
          {error}
        </div>
      )}
      <button className="btn" type="submit" disabled={busy}>
        {busy ? 'กำลังเข้าสู่ระบบ…' : 'เข้าสู่ระบบ / Sign in'}
      </button>
    </form>
  );
}
