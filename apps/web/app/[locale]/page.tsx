import { Hero } from '@/components/landing/Hero';
import { Faq, PreviewBanner, Shows } from '@/components/landing/Cms';
import { Gallery, MemberCta, News, Products, Promo, Services } from '@/components/landing/Sections';
import { getContent } from '@/lib/content';
import { getT } from '@/lib/i18n';
import { isLocale } from '@/lib/locale';
import { notFound } from 'next/navigation';

export const dynamic = 'force-dynamic';

export default async function Landing({ params, searchParams }: { params: { locale: string }; searchParams: { preview?: string } }) {
  if (!isLocale(params.locale)) notFound();
  const locale = params.locale;
  const t = getT(locale);
  const preview = typeof searchParams.preview === 'string' && searchParams.preview ? searchParams.preview : undefined;
  const [hero, shows, faq] = await Promise.all([
    getContent('hero_slide', locale, preview),
    getContent('show', locale, preview),
    getContent('faq', locale, preview),
  ]);
  const isPreview = !!(hero?.preview || shows?.preview || faq?.preview);
  return (
    <>
      {isPreview ? <PreviewBanner locale={locale} /> : null}
      <Hero slides={hero?.items.length ? hero.items : null} />
      <Promo t={t} locale={locale} />
      <Services t={t} locale={locale} />
      <Shows items={shows?.items ?? []} locale={locale} />
      <Gallery t={t} locale={locale} />
      <News t={t} locale={locale} />
      <Products t={t} locale={locale} />
      <Faq items={faq?.items ?? []} locale={locale} />
      <MemberCta t={t} locale={locale} />
    </>
  );
}
