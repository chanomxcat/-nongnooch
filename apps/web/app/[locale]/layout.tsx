import '@nn/tokens/tokens.css';
import '@fontsource/noto-sans-thai/400.css';
import '@fontsource/noto-sans-thai/500.css';
import '@fontsource/noto-sans-thai/600.css';
import '@fontsource/noto-sans-thai/700.css';
import '../globals.css';

import type { Metadata, Viewport } from 'next';
import { notFound } from 'next/navigation';
import type { ReactNode } from 'react';
import { Providers } from '@/components/providers/Providers';
import { Footer } from '@/components/shell/Footer';
import { Nav } from '@/components/shell/Nav';
import { getMessages, getT } from '@/lib/i18n';
import { htmlLang, isLocale, locales, type Locale } from '@/lib/locale';

export function generateStaticParams() {
  return locales.map((locale) => ({ locale }));
}

export const viewport: Viewport = { themeColor: '#00271C', width: 'device-width', initialScale: 1 };

export function generateMetadata({ params }: { params: { locale: string } }): Metadata {
  if (!isLocale(params.locale)) return {};
  const t = getT(params.locale);
  const languages: Record<string, string> = Object.fromEntries(locales.map((l) => [htmlLang(l), `/${l}`]));
  languages['x-default'] = '/en';
  return {
    metadataBase: new URL(process.env.SITE_URL ?? 'http://localhost:3000'),
    title: { default: t('meta.title'), template: `%s · ${t('meta.title')}` },
    description: t('meta.description'),
    alternates: { canonical: `/${params.locale}`, languages },
    icons: { icon: '/images/favicon.svg' },
  };
}

/** CJK pages load their own self-hosted Noto subset (DESIGN 2.2: per-locale fonts, no Google Fonts). */
const cjkFont: Partial<Record<Locale, string>> = { zh: '/fonts/sc/font.css', ja: '/fonts/jp/font.css', ko: '/fonts/kr/font.css' };

export default function LocaleLayout({ children, params }: { children: ReactNode; params: { locale: string } }) {
  if (!isLocale(params.locale)) notFound();
  const locale = params.locale;
  const t = getT(locale);
  const font = cjkFont[locale];
  return (
    <html lang={htmlLang(locale)}>
      <head>{font && <link rel="stylesheet" href={font} />}</head>
      <body>
        <a href="#main" className="nn-skip">
          {t('nav.skip')}
        </a>
        <Providers locale={locale} messages={getMessages(locale)}>
          <Nav />
          <main id="main">{children}</main>
          <Footer t={t} locale={locale} />
        </Providers>
      </body>
    </html>
  );
}
