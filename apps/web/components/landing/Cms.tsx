import Image from 'next/image';
import { Icon } from '@/components/ui/Icon';
import { str, type ContentItem } from '@/lib/content';
import type { Locale } from '@/lib/locale';
import s from './landing.module.css';

// ponytail: section labels live here (TH/EN + EN fallback) instead of 7 message files; move to messages/*.json with the full CMS.
const L = {
  th: { shows: 'รอบการแสดงวันนี้', showsLead: 'เวลาแสดงตามเวลาประเทศไทย', faq: 'คำถามที่พบบ่อย', preview: 'โหมดตัวอย่าง (ยังไม่เผยแพร่)' },
  en: { shows: "Today's shows", showsLead: 'Times are Thailand time (ICT)', faq: 'Frequently asked questions', preview: 'Preview mode (not published)' },
} as const;
const label = (locale: Locale) => (locale === 'th' ? L.th : L.en);

export function PreviewBanner({ locale }: { locale: Locale }) {
  return (
    <div className={s.previewBanner} role="status">
      <Icon name="clock" size={16} />
      {/* The Thai label is shown in every language: the banner is for staff checking a draft. */}
      {L.th.preview}
      {locale !== 'th' ? ` · ${L.en.preview}` : null}
    </div>
  );
}

export function Shows({ items, locale }: { items: ContentItem[]; locale: Locale }) {
  if (items.length === 0) return null;
  const l = label(locale);
  return (
    <section className={s.cmsSection} id="shows" aria-labelledby="shows-title">
      <div className="nn-container">
        <div className={s.sectionHead}>
          <h2 id="shows-title" className={s.h2}>
            {l.shows}
          </h2>
          <p className={s.lead}>{l.showsLead}</p>
        </div>
        <ul className={s.showGrid}>
          {items.map((it) => {
            const times = Array.isArray(it.attrs.times) ? (it.attrs.times as string[]) : [];
            const img = str(it.attrs.imageUrl);
            return (
              <li key={it.id} className={s.showCard}>
                {img ? (
                  <div className={s.showImg}>
                    <Image src={img} alt="" fill sizes="(min-width: 768px) 360px, 100vw" />
                  </div>
                ) : null}
                <div className={s.showBody}>
                  <h3 className={s.showName}>{it.texts.name}</h3>
                  {it.texts.venue ? <p className={s.showVenue}>{it.texts.venue}</p> : null}
                  {it.texts.description ? <p className={s.showDesc}>{it.texts.description}</p> : null}
                  <ul className={s.showTimes} aria-label={l.shows}>
                    {times.map((tm) => (
                      <li key={tm}>{tm}</li>
                    ))}
                  </ul>
                </div>
              </li>
            );
          })}
        </ul>
      </div>
    </section>
  );
}

export function Faq({ items, locale }: { items: ContentItem[]; locale: Locale }) {
  if (items.length === 0) return null;
  return (
    <section className={s.cmsSection} id="faq" aria-labelledby="faq-title">
      <div className="nn-container">
        <div className={s.sectionHead}>
          <h2 id="faq-title" className={s.h2}>
            {label(locale).faq}
          </h2>
        </div>
        <div className={s.faqList}>
          {items.map((it) => (
            <details key={it.id} className={s.faqItem}>
              <summary>{it.texts.question}</summary>
              <p>{it.texts.answer}</p>
            </details>
          ))}
        </div>
      </div>
    </section>
  );
}
