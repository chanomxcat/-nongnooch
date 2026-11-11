'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useEffect, useRef, useState } from 'react';
import { Icon } from '@/components/ui/Icon';
import { useI18n, useSession } from '@/components/providers/Providers';
import { auth } from '@/lib/api/client';
import { locales, localeNames, type Locale } from '@/lib/locale';
import s from './shell.module.css';

const MENU = [
  { key: 'home', href: '' },
  { key: 'activities', href: '#activities' },
  { key: 'zone', href: '#gallery' },
  { key: 'auditorium', href: '#services' },
  { key: 'accommodation', href: '#services' },
  { key: 'contact', href: '#contact' },
] as const;

export function Nav() {
  const { t, locale } = useI18n();
  const session = useSession();
  const pathname = usePathname();
  const overlay = pathname === `/${locale}`;
  const router = useRouter();
  const [scrolled, setScrolled] = useState(false);
  const [progress, setProgress] = useState(0);
  const [open, setOpen] = useState(false);

  // Motion A (sticky shrink) + Motion B (scroll progress)
  useEffect(() => {
    let raf = 0;
    const onScroll = () => {
      cancelAnimationFrame(raf);
      raf = requestAnimationFrame(() => {
        const y = window.scrollY;
        setScrolled(y > 80);
        const max = document.documentElement.scrollHeight - window.innerHeight;
        setProgress(max > 0 ? Math.min(1, y / max) : 0);
      });
    };
    onScroll();
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener('scroll', onScroll);
    };
  }, []);

  useEffect(() => setOpen(false), [pathname]);

  const home = `/${locale}`;
  const isHome = pathname === home;
  const authed = session.status === 'authed';

  async function signOut() {
    await auth.logout().catch(() => null);
    await session.reload();
    router.push(home);
  }

  return (
    <header className={`${s.header} ${overlay && !scrolled ? s.overlay : ''} ${scrolled ? s.scrolled : ''}`}>
      <div className={s.utility}>
        <div className={s.utilityInfo}>
          <span className={s.brandLine}>
            <strong>{t('nav.brand')}</strong>
            <span className={s.hideSm}>{t('nav.tagline')}</span>
          </span>
          <span className={`${s.divider} ${s.hideSm}`} aria-hidden="true" />
          <span className={`${s.hours} ${s.hideSm}`}>
            <Icon name="clock" size={14} />
            {t('nav.hours')}
          </span>
        </div>
        <div className={s.utilityRight}>
          <LocaleSwitcher />
          <span className={s.divider} aria-hidden="true" />
          <span className={s.currency}>
            <Icon name="coin" size={16} />
            THB
          </span>
        </div>
      </div>

      <nav className={s.main} aria-label="Main">
        <Link href={home} className={s.logo} aria-label={t('nav.logoAlt')}>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src="/images/logo.svg" alt="" width={196} height={40} />
        </Link>

        <ul className={s.menu}>
          {MENU.map((m) => {
            const href = m.href.startsWith('#') ? `${home}${m.href}` : home;
            const active = m.key === 'home' && isHome;
            return (
              <li key={m.key}>
                <Link href={href} className={`${s.menuLink} ${active ? s.active : ''}`} aria-current={active ? 'page' : undefined}>
                  {t(`nav.${m.key}`)}
                </Link>
              </li>
            );
          })}
        </ul>

        <div className={s.actions}>
          {authed ? (
            <AccountMenu onSignOut={signOut} />
          ) : (
            <Link href={`/${locale}/login`} className={s.signIn}>
              <Icon name="login" size={18} />
              <span>{t('nav.signIn')}</span>
              <span className={s.slash} aria-hidden="true">
                /
              </span>
              <span>{t('nav.register')}</span>
            </Link>
          )}
          <Link href={`/${locale}/checkout`} className={`nn-btn nn-btn--gold nn-btn--sm ${s.buy}`}>
            <Icon name="ticket" size={18} />
            {t('nav.buyTickets')}
          </Link>
          <button type="button" className={s.burger} aria-expanded={open} aria-controls="nn-mobile-menu" onClick={() => setOpen((o) => !o)}>
            <Icon name={open ? 'close' : 'menu'} size={22} />
            <span className="nn-sr-only">{open ? t('nav.close') : t('nav.menu')}</span>
          </button>
        </div>
      </nav>

      <div className={s.progress} style={{ transform: `scaleX(${progress})` }} aria-hidden="true" />

      {open && (
        <div id="nn-mobile-menu" className={s.mobile}>
          <ul>
            {MENU.map((m) => (
              <li key={m.key}>
                <Link href={m.href.startsWith('#') ? `${home}${m.href}` : home} onClick={() => setOpen(false)}>
                  {t(`nav.${m.key}`)}
                </Link>
              </li>
            ))}
          </ul>
          <div className={s.mobileActions}>
            {authed ? (
              <>
                <Link href={`/${locale}/tickets`} className="nn-btn nn-btn--outline-light nn-btn--block">
                  {t('nav.myTickets')}
                </Link>
                <Link href={`/${locale}/points`} className="nn-btn nn-btn--outline-light nn-btn--block">
                  {t('nav.myPoints')}
                </Link>
              </>
            ) : (
              <Link href={`/${locale}/login`} className="nn-btn nn-btn--outline-light nn-btn--block">
                {t('nav.signIn')} / {t('nav.register')}
              </Link>
            )}
            <Link href={`/${locale}/checkout`} className="nn-btn nn-btn--gold nn-btn--block">
              {t('nav.buyTickets')}
            </Link>
          </div>
        </div>
      )}
    </header>
  );
}

function AccountMenu({ onSignOut }: { onSignOut: () => void }) {
  const { t, locale } = useI18n();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent | KeyboardEvent) => {
      if (e instanceof KeyboardEvent ? e.key === 'Escape' : !ref.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', close);
    return () => {
      document.removeEventListener('mousedown', close);
      document.removeEventListener('keydown', close);
    };
  }, [open]);
  return (
    <div className={s.dropdownWrap} ref={ref}>
      <button type="button" className={s.signIn} aria-expanded={open} aria-haspopup="menu" onClick={() => setOpen((o) => !o)}>
        <Icon name="user" size={18} />
        <span>{t('nav.account')}</span>
        <Icon name="down" size={12} />
      </button>
      {open && (
        <ul className={s.dropdown} role="menu">
          <li role="none">
            <Link role="menuitem" href={`/${locale}/tickets`} onClick={() => setOpen(false)}>
              <Icon name="qr" size={18} /> {t('nav.myTickets')}
            </Link>
          </li>
          <li role="none">
            <Link role="menuitem" href={`/${locale}/points`} onClick={() => setOpen(false)}>
              <Icon name="star" size={18} /> {t('nav.myPoints')}
            </Link>
          </li>
          <li role="none">
            <button role="menuitem" type="button" onClick={onSignOut}>
              <Icon name="logout" size={18} /> {t('nav.signOut')}
            </button>
          </li>
        </ul>
      )}
    </div>
  );
}

function LocaleSwitcher() {
  const { t, locale } = useI18n();
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent | KeyboardEvent) => {
      if (e instanceof KeyboardEvent ? e.key === 'Escape' : !ref.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', close);
    return () => {
      document.removeEventListener('mousedown', close);
      document.removeEventListener('keydown', close);
    };
  }, [open]);
  const swap = (l: Locale) => pathname.replace(/^\/[a-z]{2}(?=\/|$)/, `/${l}`);
  return (
    <div className={s.dropdownWrap} ref={ref}>
      <button
        type="button"
        className={s.utilityBtn}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-label={`${t('nav.language')}: ${localeNames[locale]}`}
        onClick={() => setOpen((o) => !o)}
      >
        <Icon name="globe" size={16} />
        <span>{localeNames[locale]}</span>
        <Icon name="down" size={12} />
      </button>
      {open && (
        <ul className={`${s.dropdown} ${s.localeList}`}>
          {locales.map((l) => (
            <li key={l}>
              <a href={swap(l)} lang={l === 'zh' ? 'zh-Hans' : l} aria-current={l === locale ? 'true' : undefined} className={l === locale ? s.current : ''}>
                {localeNames[l]}
                {l === locale && <Icon name="check" size={14} />}
              </a>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
