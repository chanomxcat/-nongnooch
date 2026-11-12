'use client';

import Image from 'next/image';
import Link from 'next/link';
import { useEffect, useRef, useState } from 'react';
import { Icon } from '@/components/ui/Icon';
import { useI18n } from '@/components/providers/Providers';
import { formatNumber } from '@/lib/format';
import { str, type ContentItem } from '@/lib/content';
import s from './landing.module.css';

const ACTIVITIES = [
  { key: 'garden', img: '/images/act-beautiful-garden.webp' },
  { key: 'dino', img: '/images/act-dinosaur-valley.webp' },
  { key: 'show', img: '/images/act-thai-cultural-show.webp' },
  { key: 'sky', img: '/images/act-garden-in-the-sky.webp' },
  { key: 'arapaima', img: '/images/act-arapaima.webp' },
  { key: 'botanical', img: '/images/act-botanical-garden.webp' },
] as const;

const STATS = [
  { value: 1700, key: 'area' },
  { value: 60, key: 'zones', suffix: '+' },
  { value: 12000, key: 'species', suffix: '+' },
] as const;

type Card = { key: string; img: string; title: string; tag: string };

/** `slides` come from the CMS (hero_slide: first = headline, rest = activity cards); the i18n texts are the fallback. */
export function Hero({ slides }: { slides?: ContentItem[] | null }) {
  const { t, locale } = useI18n();
  const [active, setActive] = useState(0);
  const track = useRef<HTMLUListElement>(null);
  const head = slides?.[0];
  const cards: Card[] =
    slides && slides.length > 1
      ? slides.slice(1).map((sl) => ({ key: sl.id, img: str(sl.attrs.imageUrl) || '/images/hero.webp', title: sl.texts.title ?? '', tag: sl.texts.subtitle ?? '' }))
      : ACTIVITIES.map((act) => ({ key: act.key, img: act.img, title: t(`activities.${act.key}.title`), tag: t(`activities.${act.key}.tag`) }));
  const a = cards[active] ?? cards[0];
  const headTitle = head?.texts.title || t('hero.title');
  const headSub = head ? head.texts.subtitle ?? '' : t('hero.subtitle');
  const href = str(head?.attrs.ctaHref) || '/checkout';
  // The API only accepts "/path" or https URLs; guard again so a bad value can never become a javascript: link.
  const ctaHref = href.startsWith('/') && !href.startsWith('//') ? `/${locale}${href}` : href.startsWith('https://') ? href : `/${locale}/checkout`;

  // Motion F: arrows move one card; the track itself scrolls / snaps (drag & swipe on touch).
  function go(i: number) {
    const n = (i + cards.length) % cards.length;
    setActive(n);
    const card = track.current?.children[n] as HTMLElement | undefined;
    if (card && track.current) {
      const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
      track.current.scrollTo({ left: card.offsetLeft - track.current.offsetLeft, behavior: reduce ? 'auto' : 'smooth' });
    }
  }

  return (
    <section className={s.hero} aria-labelledby="hero-title">
      <Image src={str(head?.attrs.imageUrl) || '/images/hero.webp'} alt="" fill priority sizes="100vw" className={s.heroBg} />
      <div className={s.heroShade} aria-hidden="true" />

      <div className={`nn-container ${s.heroInner}`}>
        <div className={s.heroCopy}>
          <span className={`${s.chip} ${s.rise}`}>
            <Icon name="clock" size={16} />
            {t('hero.chip')}
          </span>
          <div className={s.rise} style={{ animationDelay: '90ms' }} key={a.key} aria-live="polite">
            <h1 id="hero-title" className={s.heroTitle}>
              {active === 0 ? headTitle : a.title}
            </h1>
            <p className={s.heroSubtitle}>{active === 0 ? headSub : a.tag}</p>
          </div>
          <p className={`${s.heroBody} ${s.rise}`} style={{ animationDelay: '180ms' }}>
            {t('hero.body')}
          </p>
          <div className={`${s.heroCtas} ${s.rise}`} style={{ animationDelay: '270ms' }}>
            <Link href={ctaHref} className="nn-btn nn-btn--gold">
              <Icon name="ticket" size={20} />
              {head?.texts.ctaLabel || t('hero.ctaTickets')}
            </Link>
            <a href="#activities" className="nn-btn nn-btn--outline-light">
              {t('hero.cta')}
              <Icon name="arrowR" size={20} />
            </a>
          </div>
          <dl className={s.stats}>
            {STATS.map((st) => (
              <StatItem key={st.key} value={st.value} suffix={'suffix' in st ? st.suffix : ''} label={t(`stats.${st.key}`)} locale={locale} />
            ))}
          </dl>
        </div>

        <div className={s.heroCards} id="activities">
          <h2 className="nn-sr-only">{t('hero.activitiesLabel')}</h2>
          <ul className={s.cardTrack} ref={track}>
            {cards.map((act, i) => (
              <li key={act.key}>
                <button
                  type="button"
                  className={`${s.actCard} ${i === active ? s.actActive : ''}`}
                  aria-pressed={i === active}
                  onClick={() => go(i)}
                >
                  <Image src={act.img} alt="" fill sizes="220px" className={s.actImg} />
                  <span className={s.actShade} aria-hidden="true" />
                  <span className={s.actInfo}>
                    <span className={s.actNum}>{String(i + 1).padStart(2, '0')}</span>
                    <span className={s.actTitle}>{act.title}</span>
                    <span className={s.actTag}>{act.tag}</span>
                  </span>
                </button>
              </li>
            ))}
          </ul>
          <div className={s.cardControls}>
            <button type="button" className={s.arrowBtn} onClick={() => go(active - 1)} aria-label={t('hero.prev')}>
              <Icon name="arrowL" size={20} />
            </button>
            <button type="button" className={`${s.arrowBtn} ${s.arrowGold}`} onClick={() => go(active + 1)} aria-label={t('hero.next')}>
              <Icon name="arrowR" size={20} />
            </button>
            <span className={s.counter}>{t('hero.counter', { current: String(active + 1).padStart(2, '0'), total: String(cards.length).padStart(2, '0') })}</span>
            <span className={s.progressTrack} aria-hidden="true">
              <span className={s.progressFill} style={{ transform: `scaleX(${(active + 1) / cards.length})` }} />
            </span>
          </div>
        </div>
      </div>
    </section>
  );
}

/** Motion E: count up once when visible. Screen readers always get the final value. */
function StatItem({ value, suffix, label, locale }: { value: number; suffix: string; label: string; locale: Parameters<typeof formatNumber>[1] }) {
  const ref = useRef<HTMLDivElement>(null);
  const [shown, setShown] = useState(value);
  useEffect(() => {
    const el = ref.current;
    if (!el || window.matchMedia('(prefers-reduced-motion: reduce)').matches || typeof IntersectionObserver === 'undefined') return;
    setShown(0);
    let raf = 0;
    const io = new IntersectionObserver(([e]) => {
      if (!e.isIntersecting) return;
      io.disconnect();
      const start = performance.now();
      const tick = (now: number) => {
        const p = Math.min(1, (now - start) / 1200);
        setShown(Math.round(value * (1 - Math.pow(1 - p, 3))));
        if (p < 1) raf = requestAnimationFrame(tick);
      };
      raf = requestAnimationFrame(tick);
    });
    io.observe(el);
    return () => {
      io.disconnect();
      cancelAnimationFrame(raf);
    };
  }, [value]);
  const final = `${formatNumber(value, locale)}${suffix}`;
  return (
    <div className={s.stat} ref={ref}>
      <dt className="nn-sr-only">{label}</dt>
      <dd>
        <span className="nn-sr-only">{final}</span>
        <span className={s.statValue} aria-hidden="true">
          {formatNumber(shown, locale)}
          {suffix}
        </span>
        <span className={s.statLabel} aria-hidden="true">
          {label}
        </span>
      </dd>
    </div>
  );
}
