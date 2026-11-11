'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import { Icon } from '@/components/ui/Icon';
import { ThaiFlag } from '@/components/ui/ThaiFlag';
import { useErrorText, useI18n, useSession } from '@/components/providers/Providers';
import { ApiError, auth, member } from '@/lib/api/client';
import { normalizeInvite } from '@/lib/invite';
import { safeNext } from '@/lib/safeNext';
import { formatThaiId, isValidThaiId, normalizePassport, normalizeThaiId } from '@/lib/thaiId';
import { ErrorBox } from './ErrorBox';
import { clearInviteCookie, readInviteCookie } from './inviteCookie';
import s from '@/components/account/account.module.css';

export const COUNTRIES = ['TH', 'CN', 'JP', 'KR', 'IN', 'MY', 'SG', 'VN', 'ID', 'PH', 'LA', 'MM', 'KH', 'TW', 'HK', 'RU', 'DE', 'FR', 'GB', 'IT', 'ES', 'NL', 'SE', 'US', 'CA', 'AU', 'NZ', 'AE', 'SA', 'IL'];

export function useCountryName() {
  const { locale } = useI18n();
  return useMemo(() => {
    let dn: Intl.DisplayNames | null = null;
    try {
      dn = new Intl.DisplayNames([locale === 'zh' ? 'zh-Hans' : locale], { type: 'region' });
    } catch {
      dn = null;
    }
    return (code: string) => dn?.of(code) ?? code;
  }, [locale]);
}

export function ProfileForm() {
  const { t, locale } = useI18n();
  const errText = useErrorText();
  const session = useSession();
  const router = useRouter();
  const params = useSearchParams();
  const next = safeNext(params.get('next'), locale);
  const countryName = useCountryName();

  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');
  const [isThai, setIsThai] = useState(locale === 'th');
  const [nationality, setNationality] = useState('CN');
  const [thaiId, setThaiId] = useState('');
  const [passport, setPassport] = useState('');
  const [passportCountry, setPassportCountry] = useState('');
  const [invite, setInvite] = useState('');
  const [terms, setTerms] = useState(false);
  const [pdpa, setPdpa] = useState(false);
  const [marketing, setMarketing] = useState(false);
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const registered = useRef(false);

  useEffect(() => setInvite(readInviteCookie() ?? ''), []);
  // Not signed in → back to the OTP step; already a member → nothing to fill in.
  useEffect(() => {
    if (registered.current) return;
    if (session.status === 'anon') router.replace(`/${locale}/register?next=${encodeURIComponent(next)}`);
    if (session.status === 'authed' && session.memberId) router.replace(next);
  }, [session.status, session.memberId, router, locale, next]);

  const idOk = isThai ? isValidThaiId(thaiId) : !!normalizePassport(passport);
  const inviteOk = !invite.trim() || !!normalizeInvite(invite);
  const canSubmit = firstName.trim() && lastName.trim() && idOk && inviteOk && terms && pdpa && !busy;
  const idError = touched.id && !idOk ? (isThai ? t('profile.idInvalid') : t('profile.passportInvalid')) : null;

  async function submit(e: FormEvent) {
    e.preventDefault();
    setTouched({ id: true, first: true, last: true });
    if (!canSubmit) return;
    setBusy(true);
    setError(null);
    try {
      await member.register({
        firstName: firstName.trim(),
        lastName: lastName.trim(),
        nationality: isThai ? 'TH' : nationality,
        idNumber: isThai ? normalizeThaiId(thaiId)! : normalizePassport(passport)!,
        passportCountry: isThai ? undefined : passportCountry || nationality,
        inviteCode: normalizeInvite(invite) ?? undefined,
        marketing,
        lang: locale,
      });
      registered.current = true;
      // New token now carries mid (contracts section 3).
      await auth.refresh().catch(() => null);
      clearInviteCookie();
      setThaiId('');
      setPassport('');
      await session.reload();
      router.push(`/${locale}/register/done?next=${encodeURIComponent(next)}`);
    } catch (err) {
      const code = (err as ApiError).code;
      if (code === 'invalid_token' || code === 'refresh_invalid') {
        router.replace(`/${locale}/register?next=${encodeURIComponent(next)}`);
        return;
      }
      setError(code);
      setBusy(false);
    }
  }

  return (
    <section className={s.page}>
      <div className={`nn-container ${s.narrow}`}>
        <div className={s.head}>
          <h1 className={s.title}>{t('profile.title')}</h1>
          <p className={s.lead}>{t('profile.lead')}</p>
        </div>
        <form className={`${s.card}`} onSubmit={submit} noValidate>
          <div className={s.row2}>
            <TextField id="firstName" label={t('profile.firstName')} value={firstName} onChange={setFirstName} autoComplete="given-name" />
            <TextField id="lastName" label={t('profile.lastName')} value={lastName} onChange={setLastName} autoComplete="family-name" />
          </div>

          <fieldset className={s.field} style={{ border: 0, padding: 0, margin: 0 }}>
            <legend className={s.label} style={{ marginBottom: 6 }}>
              {t('profile.nationality')}
            </legend>
            <div className={s.tabs} role="radiogroup">
              {[true, false].map((v) => (
                <button
                  key={String(v)}
                  type="button"
                  role="radio"
                  aria-checked={isThai === v}
                  className={s.tab}
                  onClick={() => {
                    setIsThai(v);
                    setTouched((x) => ({ ...x, id: false }));
                  }}
                >
                  {v ? <ThaiFlag /> : <Icon name="globe" size={16} />}
                  {v ? t('profile.thai') : t('profile.foreign')}
                </button>
              ))}
            </div>
          </fieldset>

          {isThai ? (
            <div className={s.field}>
              <label htmlFor="thaiId" className={s.label}>
                {t('profile.thaiId')}
              </label>
              <input
                id="thaiId"
                className={s.input}
                inputMode="numeric"
                autoComplete="off"
                placeholder="x-xxxx-xxxxx-xx-x"
                value={formatThaiId(thaiId)}
                onChange={(e) => setThaiId(e.target.value.replace(/\D/g, '').slice(0, 13))}
                onBlur={() => setTouched((x) => ({ ...x, id: true }))}
                aria-invalid={!!idError}
                aria-describedby="thaiId-hint"
              />
              {idError ? (
                <span id="thaiId-hint" className={s.fieldError} aria-live="polite">
                  <Icon name="alert" size={14} />
                  {idError}
                </span>
              ) : (
                <span id="thaiId-hint" className={s.hint}>
                  {t('profile.thaiIdHint')}
                </span>
              )}
            </div>
          ) : (
            <>
              <div className={s.field}>
                <label htmlFor="nationality" className={s.label}>
                  {t('profile.nationality')}
                </label>
                <select id="nationality" className={s.select} value={nationality} onChange={(e) => setNationality(e.target.value)}>
                  {COUNTRIES.filter((c) => c !== 'TH').map((c) => (
                    <option key={c} value={c}>
                      {countryName(c)}
                    </option>
                  ))}
                </select>
              </div>
              <div className={s.row2}>
                <div className={s.field}>
                  <label htmlFor="passport" className={s.label}>
                    {t('profile.passport')}
                  </label>
                  <input
                    id="passport"
                    className={s.input}
                    autoComplete="off"
                    autoCapitalize="characters"
                    value={passport}
                    onChange={(e) => setPassport(e.target.value.toUpperCase())}
                    onBlur={() => setTouched((x) => ({ ...x, id: true }))}
                    aria-invalid={!!idError}
                    aria-describedby="passport-err"
                  />
                  {idError && (
                    <span id="passport-err" className={s.fieldError} aria-live="polite">
                      <Icon name="alert" size={14} />
                      {idError}
                    </span>
                  )}
                </div>
                <div className={s.field}>
                  <label htmlFor="passportCountry" className={s.label}>
                    {t('profile.passportCountry')}
                  </label>
                  <select id="passportCountry" className={s.select} value={passportCountry || nationality} onChange={(e) => setPassportCountry(e.target.value)}>
                    {COUNTRIES.map((c) => (
                      <option key={c} value={c}>
                        {countryName(c)}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            </>
          )}
          <p className={s.hint} style={{ display: 'flex', gap: 6, alignItems: 'center', marginTop: -8 }}>
            <Icon name="lock" size={14} />
            {t('profile.idPrivacy')}
          </p>

          <div className={s.field}>
            <label htmlFor="invite" className={s.label}>
              {t('profile.invite')}
            </label>
            <input
              id="invite"
              className={s.input}
              autoCapitalize="characters"
              value={invite}
              onChange={(e) => {
                setInvite(e.target.value.toUpperCase());
                if (error === 'invite_invalid') setError(null);
              }}
              aria-invalid={!inviteOk || error === 'invite_invalid'}
              aria-describedby="invite-hint"
            />
            {error === 'invite_invalid' ? (
              <span id="invite-hint" className={s.fieldError}>
                <Icon name="alert" size={14} />
                {errText('invite_invalid')}{' '}
                <button
                  type="button"
                  className={s.linkBtn}
                  onClick={() => {
                    setInvite('');
                    setError(null);
                  }}
                >
                  {t('profile.withoutInvite')}
                </button>
              </span>
            ) : (
              <span id="invite-hint" className={s.hint}>
                {t('profile.inviteHint')}
              </span>
            )}
          </div>

          <div className={s.consent}>
            <label>
              <input type="checkbox" checked={terms} onChange={(e) => setTerms(e.target.checked)} required />
              <span>{t('profile.terms')} *</span>
            </label>
            <label>
              <input type="checkbox" checked={pdpa} onChange={(e) => setPdpa(e.target.checked)} required />
              <span>{t('profile.pdpa')} *</span>
            </label>
            <label>
              <input type="checkbox" checked={marketing} onChange={(e) => setMarketing(e.target.checked)} />
              <span>{t('profile.marketing')}</span>
            </label>
          </div>

          {error && error !== 'invite_invalid' && <ErrorBox text={errText(error)} />}
          <button type="submit" className="nn-btn nn-btn--gold nn-btn--block" disabled={!canSubmit}>
            {busy ? t('profile.submitting') : t('profile.submit')}
          </button>
        </form>
      </div>
    </section>
  );
}

function TextField({
  id,
  label,
  value,
  onChange,
  autoComplete,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (v: string) => void;
  autoComplete?: string;
}) {
  return (
    <div className={s.field}>
      <label htmlFor={id} className={s.label}>
        {label}
      </label>
      <input id={id} className={s.input} value={value} onChange={(e) => onChange(e.target.value)} autoComplete={autoComplete} required />
    </div>
  );
}
