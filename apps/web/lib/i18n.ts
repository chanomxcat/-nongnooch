import type { Locale } from './locale';
import en from '../messages/en.json';
import th from '../messages/th.json';
import zh from '../messages/zh.json';
import ja from '../messages/ja.json';
import ko from '../messages/ko.json';
import de from '../messages/de.json';
import fr from '../messages/fr.json';

export type Messages = typeof en;
type Tree = { [k: string]: string | Tree };

const raw: Record<Locale, Tree> = { en, th, zh, ja, ko, de, fr };

/** Deep-merge locale messages over EN so missing keys fall back to English. */
export function mergeMessages(base: Tree, over: Tree): Tree {
  const out: Tree = { ...base };
  for (const [k, v] of Object.entries(over)) {
    const b = base[k];
    out[k] = v && typeof v === 'object' && b && typeof b === 'object' ? mergeMessages(b, v) : v;
  }
  return out;
}

const cache = new Map<Locale, Messages>();
export function getMessages(locale: Locale): Messages {
  let m = cache.get(locale);
  if (!m) {
    m = mergeMessages(raw.en, raw[locale] ?? {}) as unknown as Messages;
    cache.set(locale, m);
  }
  return m;
}

export type Vars = Record<string, string | number>;
export type TFn = (key: string, vars?: Vars) => string;

/** Look up a dotted key; unknown keys return the key itself so gaps are visible, never a crash. */
export function translate(messages: unknown, key: string, vars?: Vars): string {
  let cur: unknown = messages;
  for (const part of key.split('.')) {
    if (cur && typeof cur === 'object' && part in (cur as Tree)) cur = (cur as Tree)[part];
    else return key;
  }
  if (typeof cur !== 'string') return key;
  return vars ? cur.replace(/\{(\w+)\}/g, (_, n) => (n in vars ? String(vars[n]) : `{${n}}`)) : cur;
}

export const getT = (locale: Locale): TFn => {
  const m = getMessages(locale);
  return (key, vars) => translate(m, key, vars);
};
