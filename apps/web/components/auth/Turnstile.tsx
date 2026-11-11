'use client';

import { useState } from 'react';
import { Icon } from '@/components/ui/Icon';
import { useI18n } from '@/components/providers/Providers';
import s from '@/components/account/account.module.css';

/**
 * Turnstile widget. Demo mode (NEXT_PUBLIC_TURNSTILE_MODE=mock) shows a checkbox that yields the
 * token "mock-ok"; with NEXT_PUBLIC_DEMO_TOOLS=true a select can force "mock-fail" for testing.
 * The real Cloudflare widget is wired in Plan 13 Task 2 (not part of the demo slice).
 */
export function Turnstile({ onToken }: { onToken: (token: string | null) => void }) {
  const { t } = useI18n();
  const [checked, setChecked] = useState(false);
  const [mode, setMode] = useState<'mock-ok' | 'mock-fail'>('mock-ok');
  const demoTools = process.env.NEXT_PUBLIC_DEMO_TOOLS === 'true';
  return (
    <div className={s.turnstile}>
      <label className={s.check}>
        <input
          type="checkbox"
          checked={checked}
          onChange={(e) => {
            setChecked(e.target.checked);
            onToken(e.target.checked ? mode : null);
          }}
        />
        <span>{checked ? t('auth.turnstileDone') : t('auth.turnstile')}</span>
        {checked && <Icon name="check" size={16} />}
      </label>
      <span className={s.turnstileBrand}>
        Cloudflare
        <br />
        Turnstile
        {demoTools && (
          <select
            aria-label="Turnstile mock result"
            value={mode}
            onChange={(e) => {
              const m = e.target.value as typeof mode;
              setMode(m);
              if (checked) onToken(m);
            }}
            style={{ display: 'block', marginTop: 4, fontSize: 11 }}
          >
            <option value="mock-ok">mock-ok</option>
            <option value="mock-fail">mock-fail</option>
          </select>
        )}
      </span>
    </div>
  );
}
