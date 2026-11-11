'use client';

import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Icon } from '@/components/ui/Icon';
import { useErrorText, useI18n, useSession } from '@/components/providers/Providers';
import { WELCOME_POINTS } from '@/lib/points';
import { ApiError, auth } from '@/lib/api/client';
import { safeNext } from '@/lib/safeNext';
import { readInviteCookie } from './inviteCookie';
import { ErrorBox } from './ErrorBox';
import { Turnstile } from './Turnstile';
import s from '@/components/account/account.module.css';

type Tab = 'phone' | 'email';

export function AuthFlow({ mode }: { mode: 'login' | 'register' }) {
  const { t, locale } = useI18n();
  const errText = useErrorText();
  const session = useSession();
  const router = useRouter();
  const params = useSearchParams();
  const next = safeNext(params.get('next'), locale);

  const [step, setStep] = useState<'contact' | 'otp'>('contact');
  const [tab, setTab] = useState<Tab>('phone');
  const [contact, setContact] = useState('');
  const [token, setToken] = useState<string | null>(null);
  const [requestId, setRequestId] = useState('');
  const [otp, setOtp] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [resendIn, setResendIn] = useState(0);
  const [invite, setInvite] = useState<string | null>(null);
  const otpRef = useRef<HTMLInputElement>(null);

  useEffect(() => setInvite(readInviteCookie()), []);
  useEffect(() => {
    if (resendIn <= 0) return;
    const id = setTimeout(() => setResendIn((n) => n - 1), 1000);
    return () => clearTimeout(id);
  }, [resendIn]);
  useEffect(() => {
    if (step === 'otp') otpRef.current?.focus();
  }, [step]);

  async function requestOtp(e?: FormEvent) {
    e?.preventDefault();
    if (!token) {
      setError('turnstile_failed');
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const value = contact.trim();
      const r = await auth.requestOtp({ ...(tab === 'phone' ? { phone: value } : { email: value }), lang: locale, turnstileToken: token });
      setRequestId(r.requestId);
      setOtp('');
      setStep('otp');
      setResendIn(60);
    } catch (err) {
      const ae = err as ApiError;
      setError(ae.code);
      if (ae.retryAfter) setResendIn(ae.retryAfter);
    } finally {
      setBusy(false);
    }
  }

  async function verify(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const r = await auth.verifyOtp(requestId, otp);
      await session.reload();
      if (r.memberId) router.push(next);
      else router.push(`/${locale}/register/profile?next=${encodeURIComponent(next)}`);
    } catch (err) {
      setError((err as ApiError).code);
      setBusy(false);
    }
  }

  const mockSms = process.env.NEXT_PUBLIC_DEMO_TOOLS === 'true';
  const title = mode === 'register' ? t('auth.registerTitle') : t('auth.loginTitle');
  const lead = mode === 'register' ? t('auth.registerLead', { points: WELCOME_POINTS }) : t('auth.loginLead');

  return (
    <section className={s.page}>
      <div className={`nn-container ${s.narrow}`}>
        <div className={s.head}>
          <h1 className={s.title}>{step === 'otp' ? t('auth.otpTitle') : title}</h1>
          <p className={s.lead}>{step === 'otp' ? t('auth.otpLead', { contact: contact.trim() }) : lead}</p>
        </div>

        <div className={s.card}>
          {invite && (
            <span className={s.badge}>
              <Icon name="gift" size={18} />
              {t('auth.inviteBadge', { code: invite })}
            </span>
          )}

          {step === 'contact' ? (
            <form onSubmit={requestOtp} className={s.stack} noValidate>
              <div className={s.tabs} role="tablist" aria-label={`${t('auth.phoneTab')} / ${t('auth.emailTab')}`}>
                {(['phone', 'email'] as const).map((k) => (
                  <button
                    key={k}
                    type="button"
                    role="tab"
                    aria-selected={tab === k}
                    className={s.tab}
                    onClick={() => {
                      setTab(k);
                      setContact('');
                      setError(null);
                    }}
                  >
                    <Icon name={k === 'phone' ? 'phone' : 'mail'} size={18} />
                    {t(k === 'phone' ? 'auth.phoneTab' : 'auth.emailTab')}
                  </button>
                ))}
              </div>
              <div className={s.field}>
                <label htmlFor="contact-input" className={s.label}>
                  {tab === 'phone' ? t('auth.phoneLabel') : t('auth.emailLabel')}
                </label>
                <input
                  id="contact-input"
                  name="contact"
                  className={s.input}
                  type={tab === 'phone' ? 'tel' : 'email'}
                  inputMode={tab === 'phone' ? 'tel' : 'email'}
                  autoComplete={tab === 'phone' ? 'tel' : 'email'}
                  placeholder={tab === 'phone' ? '081 234 5678' : 'you@example.com'}
                  value={contact}
                  onChange={(e) => setContact(e.target.value)}
                  aria-describedby="contact-hint"
                  aria-invalid={error === 'invalid_phone' || error === 'invalid_email'}
                  required
                />
                {tab === 'phone' && (
                  <span id="contact-hint" className={s.hint}>
                    {t('auth.phoneHint')}
                  </span>
                )}
              </div>
              <Turnstile onToken={setToken} />
              {error && <ErrorBox text={errText(error)} />}
              <button type="submit" className="nn-btn nn-btn--dark nn-btn--block" disabled={busy || !contact.trim() || !token}>
                {busy ? t('auth.sending') : t('auth.sendOtp')}
              </button>
            </form>
          ) : (
            <form onSubmit={verify} className={s.stack} noValidate>
              <div className={s.field}>
                <label htmlFor="otp" className={s.label}>
                  {t('auth.otpLabel')}
                </label>
                <input
                  id="otp"
                  ref={otpRef}
                  className={`${s.input} ${s.otp}`}
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  pattern="[0-9]*"
                  maxLength={6}
                  value={otp}
                  onChange={(e) => setOtp(e.target.value.replace(/\D/g, '').slice(0, 6))}
                  aria-invalid={!!error}
                />
              </div>
              {mockSms && (
                <p className={s.info}>
                  <Icon name="info" size={18} />
                  <span>
                    {t('auth.demoHint')}{' '}
                    <a href="http://localhost:8090" target="_blank" rel="noopener noreferrer" style={{ textDecoration: 'underline' }}>
                      localhost:8090
                    </a>
                  </span>
                </p>
              )}
              {error && <ErrorBox text={errText(error)} />}
              <button type="submit" className="nn-btn nn-btn--dark nn-btn--block" disabled={busy || otp.length !== 6}>
                {busy ? t('auth.verifying') : t('auth.verify')}
              </button>
              <div className={s.split}>
                <button type="button" className={s.linkBtn} onClick={() => setStep('contact')}>
                  {t('auth.change')}
                </button>
                <button type="button" className={s.linkBtn} disabled={resendIn > 0 || busy} onClick={() => requestOtp()}>
                  {resendIn > 0 ? t('auth.resendIn', { s: resendIn }) : t('auth.resend')}
                </button>
              </div>
            </form>
          )}
        </div>

        <p className={s.foot}>
          <Link
            className={s.linkBtn}
            href={`/${locale}/${mode === 'register' ? 'login' : 'register'}${params.get('next') ? `?next=${encodeURIComponent(next)}` : ''}`}
          >
            {mode === 'register' ? t('auth.toLogin') : t('auth.toRegister')}
          </Link>
        </p>
        <p className={s.secure}>
          <Icon name="shield" size={16} />
          {t('auth.secure')}
        </p>
      </div>
    </section>
  );
}
