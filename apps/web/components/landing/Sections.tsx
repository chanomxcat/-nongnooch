import Image from 'next/image';
import Link from 'next/link';
import { Icon } from '@/components/ui/Icon';
import { Reveal } from '@/components/ui/Reveal';
import { formatTHB } from '@/lib/format';
import type { TFn } from '@/lib/i18n';
import type { Locale } from '@/lib/locale';
import s from './landing.module.css';

import { WELCOME_POINTS } from '@/lib/points';

type P = { t: TFn; locale: Locale };

export function Promo({ t, locale }: P) {
  return (
    <div className={`nn-container ${s.promoWrap}`}>
      <Reveal className={s.promo}>
        <Image src="/images/pass-day.webp" alt="" fill sizes="(min-width: 1280px) 1200px, 100vw" className={s.promoImg} />
        <div className={s.promoShade} aria-hidden="true" />
        <div className={s.promoBody}>
          <span className={s.promoEyebrow}>
            <Icon name="gift" size={18} />
            {t('promo.eyebrow')}
          </span>
          <h2 className={s.promoTitle}>{t('promo.title', { points: WELCOME_POINTS })}</h2>
          <p className={s.promoText}>{t('promo.body')}</p>
          <Link href={`/${locale}/register`} className="nn-btn nn-btn--gold">
            {t('promo.cta')}
            <Icon name="arrowR" size={20} />
          </Link>
        </div>
        <div className={s.promoBadge} aria-hidden="true">
          <span>{WELCOME_POINTS}</span>
          <small>pts</small>
        </div>
      </Reveal>
    </div>
  );
}

const SERVICES: { key: string; icon: string; img: string }[] = [
  { key: 'accommodation', icon: 'bed', img: '/images/act-garden-in-the-sky.webp' },
  { key: 'auditorium', icon: 'hall', img: '/images/act-thai-cultural-show.webp' },
  { key: 'restaurant', icon: 'dish', img: '/images/prod-bakery.webp' },
  { key: 'service', icon: 'car', img: '/images/act-botanical-garden.webp' },
];

export function Services({ t }: P) {
  return (
    <section className={s.services} id="services" aria-labelledby="services-title">
      <div className="nn-container">
        <Reveal className={s.sectionHead}>
          <h2 id="services-title" className={s.h2}>
            {t('services.title')}
          </h2>
          <p className={s.lead}>{t('services.subtitle')}</p>
        </Reveal>
        <ul className={s.serviceGrid}>
          {SERVICES.map((sv, i) => (
            <Reveal as="li" key={sv.key} delay={i * 100} className={s.serviceCard}>
              <Image src={sv.img} alt="" fill sizes="(min-width: 1280px) 282px, (min-width: 768px) 45vw, 80vw" className={s.serviceImg} />
              <span className={s.serviceShade} aria-hidden="true" />
              <div className={s.serviceBody}>
                <span className={s.serviceIcon} aria-hidden="true">
                  <ServiceGlyph name={sv.icon} />
                </span>
                <h3 className={s.serviceTitle}>{t(`services.${sv.key}.title`)}</h3>
                <p className={s.serviceText}>{t(`services.${sv.key}.body`)}</p>
                <span className={s.serviceArrow} aria-hidden="true">
                  <Icon name="arrowR" size={18} />
                </span>
              </div>
            </Reveal>
          ))}
        </ul>
      </div>
    </section>
  );
}

/** bed / hall / dish / car are in the Figma icon list but were not exported yet; simple line glyphs meanwhile. */
function ServiceGlyph({ name }: { name: string }) {
  const paths: Record<string, string> = {
    bed: 'M3 18V7m0 7h18m0 4v-6a3 3 0 0 0-3-3h-7v6M7 11.5a1.5 1.5 0 1 0 0-.01',
    hall: 'M3 20h18M5 20V10l7-5 7 5v10M9 20v-5h6v5',
    dish: 'M4 15h16M5 15a7 7 0 0 1 14 0M12 8V6M3 18h18',
    car: 'M5 16h14M6 16l1.5-5h9L18 16M7 19v-3m10 3v-3M8 13.5h.01M16 13.5h.01',
  };
  return (
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
      <path d={paths[name] ?? paths.hall} />
    </svg>
  );
}

const GALLERY = [
  '/images/act-dinosaur-valley.webp',
  '/images/act-beautiful-garden.webp',
  '/images/act-garden-in-the-sky.webp',
  '/images/pass-day.webp',
  '/images/act-thai-cultural-show.webp',
  '/images/act-arapaima.webp',
  '/images/act-botanical-garden.webp',
];

export function Gallery({ t }: P) {
  return (
    <section className={s.gallery} id="gallery" aria-labelledby="gallery-title">
      <div className="nn-container">
        <Reveal className={`${s.sectionHead} ${s.onDark}`}>
          <span className={s.eyebrowGold}>{t('gallery.eyebrow')}</span>
          <h2 id="gallery-title" className={s.h2}>
            {t('gallery.title')}
          </h2>
          <p className={s.lead}>{t('gallery.subtitle')}</p>
        </Reveal>
        <Reveal className={s.collage}>
          {GALLERY.map((src, i) => (
            <div key={src} className={`${s.tile} ${s[`tile${i}`]}`} aria-hidden="true">
              <Image src={src} alt="" fill sizes="(min-width: 1280px) 420px, 40vw" />
            </div>
          ))}
        </Reveal>
      </div>
    </section>
  );
}

const NEWS = [
  { key: 'n1', img: '/images/act-botanical-garden.webp', d: '24', m: 'MAY', y: '2027' },
  { key: 'n2', img: '/images/prod-herbal.webp', d: '12', m: 'APR', y: '2027' },
  { key: 'n3', img: '/images/act-garden-in-the-sky.webp', d: '28', m: 'FEB', y: '2027' },
  { key: 'n4', img: '/images/act-thai-cultural-show.webp', d: '15', m: 'JAN', y: '2027' },
];

export function News({ t }: P) {
  return (
    <section className={s.news} id="news" aria-labelledby="news-title">
      <div className="nn-container">
        <Reveal className={s.sectionHead}>
          <h2 id="news-title" className={s.h2}>
            {t('news.title')}
          </h2>
          <p className={s.lead}>{t('news.subtitle')}</p>
        </Reveal>
        <div className={s.newsGrid}>
          <Reveal as="article" className={s.newsFeatured}>
            <div className={s.newsFeaturedImg}>
              <Image src="/images/act-beautiful-garden.webp" alt="" fill sizes="(min-width: 1280px) 590px, 100vw" />
              <span className={s.newsLatest}>{t('news.latest')}</span>
            </div>
            <div className={s.newsFeaturedBody}>
              <DateBadge d="20" m="OCT" y="2026" />
              <h3>{t('news.featured')}</h3>
            </div>
          </Reveal>
          <ul className={s.newsList}>
            {NEWS.map((n, i) => (
              <Reveal as="li" key={n.key} delay={i * 120} className={s.newsItem}>
                <DateBadge d={n.d} m={n.m} y={n.y} />
                <div className={s.newsThumb}>
                  <Image src={n.img} alt="" fill sizes="96px" />
                </div>
                <h3>{t(`news.${n.key}`)}</h3>
              </Reveal>
            ))}
          </ul>
        </div>
        <div className={s.center}>
          <a href="#news" className="nn-btn nn-btn--outline-dark nn-btn--sm">
            {t('news.viewAll')}
            <Icon name="arrowR" size={18} />
          </a>
        </div>
      </div>
    </section>
  );
}

function DateBadge({ d, m, y }: { d: string; m: string; y: string }) {
  return (
    <span className={s.dateBadge} aria-hidden="true">
      <strong>{d}</strong>
      <span>{m}</span>
      <small>{y}</small>
    </span>
  );
}

const PRODUCTS = [
  { key: 'bakery', cat: 'cafe', img: '/images/prod-bakery.webp', price: 6500, badge: 'new' },
  { key: 'cat', cat: 'experience', img: '/images/prod-cat-cafe.webp', price: 15000 },
  { key: 'coffee', cat: 'cafe', img: '/images/prod-sky-coffee.webp', price: 7000 },
  { key: 'herbal', cat: 'souvenir', img: '/images/prod-herbal.webp', price: 5900, badge: 'bestSeller' },
];

export function Products({ t, locale }: P) {
  return (
    <section className={s.products} aria-labelledby="products-title">
      <div className="nn-container">
        <Reveal className={s.productsHead}>
          <div>
            <span className={s.eyebrow}>{t('products.eyebrow')}</span>
            <h2 id="products-title" className={s.h2}>
              {t('products.title')}
            </h2>
            <p className={s.lead}>{t('products.subtitle')}</p>
          </div>
          <ul className={s.chips} aria-hidden="true">
            {['all', 'cafe', 'experience', 'souvenir'].map((c, i) => (
              <li key={c} className={i === 0 ? s.chipOn : ''}>
                {t(`products.${c}`)}
              </li>
            ))}
          </ul>
        </Reveal>
        <ul className={s.productGrid}>
          {PRODUCTS.map((p, i) => (
            <Reveal as="li" key={p.key} delay={i * 100} className={s.productCard}>
              <div className={s.productImg}>
                <Image src={p.img} alt="" fill sizes="(min-width: 1280px) 282px, (min-width: 768px) 45vw, 80vw" />
                {p.badge && <span className={s.productBadge}>{t(`products.${p.badge}`)}</span>}
                <span className={s.productHeart} aria-hidden="true">
                  <Icon name="heart" size={18} />
                </span>
              </div>
              <div className={s.productBody}>
                <span className={s.productCat}>{t(`products.${p.cat}`)}</span>
                <h3 className={s.productTitle}>{t(`products.${p.key}.title`)}</h3>
                <p className={s.productText}>{t(`products.${p.key}.body`)}</p>
                <div className={s.productFoot}>
                  <strong>{t('products.from', { price: formatTHB(p.price, locale) })}</strong>
                  <span className={s.productBag} aria-hidden="true">
                    <Icon name="bag" size={18} />
                  </span>
                </div>
              </div>
            </Reveal>
          ))}
        </ul>
      </div>
    </section>
  );
}

export function MemberCta({ t, locale }: P) {
  return (
    <section className={s.ctaSection} aria-labelledby="cta-title">
      <div className="nn-container">
        <Reveal className={s.cta}>
          <div className={s.ctaCopy}>
            <span className={s.eyebrowGold}>{t('cta.eyebrow')}</span>
            <h2 id="cta-title" className={s.ctaTitle}>
              {t('cta.title')}
            </h2>
            <p className={s.ctaBody}>{t('cta.body', { points: WELCOME_POINTS })}</p>
            <ul className={s.perks}>
              {(['p1', 'p2', 'p3'] as const).map((k) => (
                <li key={k}>
                  <span className={s.perkCheck} aria-hidden="true">
                    <Icon name="check" size={14} />
                  </span>
                  {t(`cta.${k}`, { points: WELCOME_POINTS })}
                </li>
              ))}
            </ul>
            <div className={s.ctaActions}>
              <Link href={`/${locale}/register`} className="nn-btn nn-btn--gold">
                {t('cta.button')}
                <Icon name="arrowR" size={20} />
              </Link>
              <p className={s.ctaNote}>{t('cta.marketing')}</p>
            </div>
          </div>
          <div className={s.ctaArt} aria-hidden="true">
            <div className={s.ctaCard}>
              <span className={s.ctaCardLabel}>NONGNOOCH MEMBER</span>
              <span className={s.ctaCardPoints}>{WELCOME_POINTS}</span>
              <span className={s.ctaCardUnit}>POINTS</span>
              <span className={s.ctaCardQr}>
                <Icon name="qr" size={40} />
              </span>
            </div>
          </div>
        </Reveal>
      </div>
    </section>
  );
}
