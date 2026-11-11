import { normalizeInvite } from '@/lib/invite';

/** nn_invite is set by middleware from /r/{code} or ?invite= (readable by page JS on purpose). */
export function readInviteCookie(): string | null {
  if (typeof document === 'undefined') return null;
  const m = document.cookie.match(/(?:^|;\s*)nn_invite=([^;]+)/);
  return m ? normalizeInvite(decodeURIComponent(m[1])) : null;
}

export function clearInviteCookie() {
  document.cookie = 'nn_invite=; Max-Age=0; path=/; SameSite=Lax';
}
