export const locales = ['th', 'en', 'zh', 'ja', 'ko', 'de', 'fr'] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = 'en';

export const isLocale = (v: string | undefined | null): v is Locale => !!v && (locales as readonly string[]).includes(v);

/** HTML lang / hreflang (contracts §2: zh → zh-Hans). */
export const htmlLang = (l: Locale): string => (l === 'zh' ? 'zh-Hans' : l);

/** Language names written in their own language (locale switcher). */
export const localeNames: Record<Locale, string> = {
  th: 'ไทย',
  en: 'English',
  zh: '简体中文',
  ja: '日本語',
  ko: '한국어',
  de: 'Deutsch',
  fr: 'Français',
};

/** Best match from an Accept-Language header, falling back to en. */
export function negotiate(accept: string | null | undefined): Locale {
  for (const part of (accept ?? '').split(',')) {
    const tag = part.split(';')[0].trim().toLowerCase().split('-')[0];
    if (isLocale(tag)) return tag;
  }
  return defaultLocale;
}
