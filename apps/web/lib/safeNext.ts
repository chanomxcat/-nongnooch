import type { Locale } from './locale';

/** Only same-site paths under the current locale; anything else goes to /<locale>. */
export function safeNext(next: string | null | undefined, locale: Locale): string {
  if (!next || !next.startsWith(`/${locale}`) || next.startsWith('//') || next.includes('\\')) return `/${locale}`;
  const rest = next.slice(locale.length + 1);
  if (rest !== '' && !rest.startsWith('/') && !rest.startsWith('?')) return `/${locale}`;
  return next;
}
