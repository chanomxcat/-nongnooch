'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

export default function Nav({ name, roles }: { name: string; roles: string[] }) {
  const path = usePathname();
  const cur = (p: string) => (path === p ? ('page' as const) : undefined);

  async function logout() {
    await fetch('/bff/logout', { method: 'POST', headers: { 'X-Requested-With': 'nn-web' } }).catch(() => null);
    window.location.assign('/login');
  }

  return (
    <header className="nav">
      <div className="nav-in">
        <Link className="brand" href="/dashboard" aria-label="Nongnooch">
          <img src="/logo.svg" alt="Nongnooch" />
          <span>Staff</span>
        </Link>
        <nav className="links" aria-label="Main">
          <Link href="/dashboard" aria-current={cur('/dashboard')}>
            Dashboard
          </Link>
          <Link href="/gate" aria-current={cur('/gate')}>
            Gate
          </Link>
        </nav>
        <div className="who">
          <div className="who-t">
            <b>{name}</b>
            <small>{roles.join(', ')}</small>
          </div>
          <button className="btn btn-ghost" onClick={logout}>
            ออกจากระบบ
          </button>
        </div>
      </div>
    </header>
  );
}
