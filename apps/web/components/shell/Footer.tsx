import Link from 'next/link';
import { Icon } from '@/components/ui/Icon';
import type { TFn } from '@/lib/i18n';
import type { Locale } from '@/lib/locale';
import s from './shell.module.css';

export function Footer({ t, locale }: { t: TFn; locale: Locale }) {
  const l = `/${locale}`;
  const cols: { title: string; links: [string, string][] }[] = [
    {
      title: t('footer.explore'),
      links: [
        [t('footer.beautifulGarden'), `${l}#activities`],
        [t('footer.dinoValley'), `${l}#activities`],
        [t('footer.botanical'), `${l}#activities`],
        [t('footer.culturalShow'), `${l}#activities`],
      ],
    },
    {
      title: t('footer.plan'),
      links: [
        [t('footer.buyTickets'), `${l}/checkout`],
        [t('footer.accommodation'), `${l}#services`],
        [t('footer.mice'), `${l}#services`],
        [t('footer.map'), `${l}#gallery`],
      ],
    },
    {
      title: t('footer.about'),
      links: [
        [t('footer.aboutUs'), l],
        [t('footer.news'), `${l}#news`],
        [t('footer.careers'), l],
        [t('footer.privacy'), l],
        [t('footer.agents'), l],
      ],
    },
  ];
  return (
    <footer className={s.footer} id="contact">
      <div className={`nn-container ${s.footerGrid}`}>
        <div className={s.footerBrand}>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src="/images/logo.svg" alt={t('nav.logoAlt')} width={196} height={40} />
          <p>{t('footer.tagline')}</p>
          <p className={s.footerHours}>
            <Icon name="clock" size={14} />
            {t('nav.hours')}
          </p>
        </div>
        {cols.map((c) => (
          <div key={c.title}>
            <h2 className={s.footerTitle}>{c.title}</h2>
            <ul className={s.footerLinks}>
              {c.links.map(([label, href]) => (
                <li key={label}>
                  <Link href={href}>{label}</Link>
                </li>
              ))}
            </ul>
          </div>
        ))}
        <div>
          <h2 className={s.footerTitle}>{t('footer.contact')}</h2>
          <ul className={s.footerContact}>
            <li>
              <Icon name="pin" size={18} />
              <span>{t('footer.address')}</span>
            </li>
            <li>
              <Icon name="phone" size={18} />
              <a href="tel:+6638238061">038-238-061</a>
            </li>
            <li>
              <Icon name="mail" size={18} />
              <a href="mailto:info@nongnoochtropicalgarden.com">info@nongnoochtropicalgarden.com</a>
            </li>
          </ul>
          <div className={s.social}>
            <a href="https://www.facebook.com/" aria-label="Facebook" rel="noopener noreferrer" target="_blank">
              <Icon name="facebook" size={18} />
            </a>
            <a href="https://www.instagram.com/" aria-label="Instagram" rel="noopener noreferrer" target="_blank">
              <Icon name="instagram" size={18} />
            </a>
            <a href="https://line.me/" aria-label="LINE" rel="noopener noreferrer" target="_blank">
              <Icon name="line" size={18} />
            </a>
          </div>
        </div>
      </div>
      <div className={`nn-container ${s.footerBottom}`}>
        <span>{t('footer.rights')}</span>
        <span className={s.footerFamily}>{t('footer.family')}</span>
      </div>
    </footer>
  );
}
