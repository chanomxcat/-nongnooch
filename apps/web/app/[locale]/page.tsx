import { Hero } from '@/components/landing/Hero';
import { Gallery, MemberCta, News, Products, Promo, Services } from '@/components/landing/Sections';
import { getT } from '@/lib/i18n';
import { isLocale } from '@/lib/locale';
import { notFound } from 'next/navigation';

export default function Landing({ params }: { params: { locale: string } }) {
  if (!isLocale(params.locale)) notFound();
  const locale = params.locale;
  const t = getT(locale);
  return (
    <>
      <Hero />
      <Promo t={t} locale={locale} />
      <Services t={t} locale={locale} />
      <Gallery t={t} locale={locale} />
      <News t={t} locale={locale} />
      <Products t={t} locale={locale} />
      <MemberCta t={t} locale={locale} />
    </>
  );
}
