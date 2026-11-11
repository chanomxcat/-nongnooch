import { gatewayUrl } from './session/upstream';

/** Public content API item (docs/demo-cms-app-contract.md); texts are already resolved to the requested lang. */
export type ContentItem = { id: string; type: string; attrs: Record<string, unknown>; texts: Record<string, string> };
export type ContentType = 'hero_slide' | 'show' | 'faq';

/**
 * Published items of a type, or the type list with one draft swapped in when `preview` is a valid token.
 * Returns null when content is unavailable (callers keep their built-in fallback).
 */
export async function getContent(type: ContentType, lang: string, preview?: string): Promise<{ items: ContentItem[]; preview: boolean } | null> {
  const path = preview
    ? `/api/content/public/preview/${encodeURIComponent(preview)}?lang=${lang}`
    : `/api/content/public/items?type=${type}&lang=${lang}`;
  try {
    // ponytail: no cache for the demo so an approval shows on the next refresh; add revalidate tags when a CDN purge exists.
    const r = await fetch(`${gatewayUrl()}${path}`, { cache: 'no-store', signal: AbortSignal.timeout(2500) });
    if (!r.ok) return preview ? getContent(type, lang) : null;
    const j = (await r.json()) as { items?: ContentItem[]; preview?: boolean };
    const items = (j.items ?? []).filter((i) => i.type === type);
    // A preview token belongs to one type; the other types render their published list.
    if (preview && items.length === 0) return getContent(type, lang);
    return { items, preview: !!j.preview };
  } catch {
    return null;
  }
}

export const str = (v: unknown) => (typeof v === 'string' ? v : '');
