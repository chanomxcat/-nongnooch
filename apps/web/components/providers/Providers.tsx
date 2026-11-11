'use client';

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { translate, type Messages, type TFn } from '@/lib/i18n';
import type { Locale } from '@/lib/locale';
import { auth, type Session } from '@/lib/api/client';

type I18n = { locale: Locale; t: TFn };
const I18nCtx = createContext<I18n | null>(null);

type SessionState = { status: 'loading' | 'anon' | 'authed'; memberId: string | null; reload: () => Promise<Session | null> };
const SessionCtx = createContext<SessionState | null>(null);

export function Providers({ locale, messages, children }: { locale: Locale; messages: Messages; children: ReactNode }) {
  const i18n = useMemo<I18n>(() => ({ locale, t: (k, v) => translate(messages, k, v) }), [locale, messages]);

  const [session, setSession] = useState<{ status: SessionState['status']; memberId: string | null }>({ status: 'loading', memberId: null });
  const reload = useCallback(async () => {
    try {
      const s = await auth.session();
      setSession({ status: s.loggedIn ? 'authed' : 'anon', memberId: s.memberId });
      return s;
    } catch {
      setSession({ status: 'anon', memberId: null });
      return null;
    }
  }, []);
  useEffect(() => {
    void reload();
  }, [reload]);

  const sessionValue = useMemo(() => ({ ...session, reload }), [session, reload]);
  return (
    <I18nCtx.Provider value={i18n}>
      <SessionCtx.Provider value={sessionValue}>{children}</SessionCtx.Provider>
    </I18nCtx.Provider>
  );
}

export function useI18n(): I18n {
  const v = useContext(I18nCtx);
  if (!v) throw new Error('useI18n outside Providers');
  return v;
}

export function useSession(): SessionState {
  const v = useContext(SessionCtx);
  if (!v) throw new Error('useSession outside Providers');
  return v;
}

/** Translated text for an API error code (errors.<code>, falling back to errors.unknown). */
export function useErrorText() {
  const { t } = useI18n();
  return (code: string | null | undefined) => {
    if (!code) return '';
    const key = `errors.${code}`;
    const s = t(key);
    return s === key ? t('errors.unknown') : s;
  };
}
