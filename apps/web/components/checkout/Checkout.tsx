'use client';

import Image from 'next/image';
import { useRouter, useSearchParams } from 'next/navigation';
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { ErrorBox } from '@/components/auth/ErrorBox';
import { COUNTRIES, useCountryName } from '@/components/auth/ProfileForm';
import { useErrorText, useI18n, useSession } from '@/components/providers/Providers';
import { Icon } from '@/components/ui/Icon';
import { ThaiFlag } from '@/components/ui/ThaiFlag';
import { ApiError, commerce, member, newKey, type Availability, type Hold, type Me, type Product } from '@/lib/api/client';
import { addDays, dateParts, formatTHB, formatVisitDate, todayInBangkok } from '@/lib/format';
import { mmss, remainingMs, serverOffset } from './clock';
import {
  audienceFor,
  buildOrderItems,
  clearPersisted,
  findProduct,
  holderErrors,
  MAX_TICKETS,
  persist,
  restore,
  slots,
  summarize,
  syncHolders,
  ticketCount,
  unitPrice,
  type AgeGroup,
  type CheckoutState,
  type HolderInput,
  type Pass,
} from './state';
import s from './checkout.module.css';

type Phase = 'select' | 'holders' | 'review';
const BOOK_AHEAD_DAYS = 60;
const STRIP = 7;

export function Checkout() {
  const { t, locale } = useI18n();
  const errText = useErrorText();
  const session = useSession();
  const router = useRouter();
  const params = useSearchParams();
  const today = useMemo(() => todayInBangkok(), []);

  const [state, setState] = useState<CheckoutState>(() => ({
    pass: 'day',
    segment: locale === 'th' ? 'thai' : 'foreign',
    visitDate: null,
    qty: { adult: 1, child: 0 },
    holders: [],
    idempotencyKey: newKey(),
  }));
  const [phase, setPhase] = useState<Phase>('select');
  const [catalog, setCatalog] = useState<Product[] | null>(null);
  const [loadError, setLoadError] = useState(false);
  const [avail, setAvail] = useState<Availability>({});
  const [me, setMe] = useState<Me | null>(null);
  const [hold, setHold] = useState<Hold | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [stripStart, setStripStart] = useState(today);
  const restored = useRef(false);
  const topRef = useRef<HTMLDivElement>(null);

  const isMember = session.status === 'authed' && !!session.memberId;
  const ctx = useMemo(() => ({ thaiVerified: !!me?.thaiVerified }), [me]);

  // Restore after login redirect (client only, avoids hydration mismatch).
  useEffect(() => {
    if (restored.current) return;
    restored.current = true;
    const saved = restore();
    if (saved) {
      setState({ ...saved, idempotencyKey: saved.idempotencyKey || newKey() });
      if (saved.visitDate) setStripStart(saved.visitDate < today ? today : saved.visitDate);
    }
  }, [today]);

  useEffect(() => {
    if (!isMember) {
      setMe(null);
      return;
    }
    member.me().then(setMe, () => setMe(null));
  }, [isMember]);

  // Resume at the holders step after signing in.
  useEffect(() => {
    if (params.get('step') === 'holders' && isMember && me && phase === 'select' && ticketCount(state) > 0 && state.visitDate) {
      setState((st) => ({ ...st, holders: syncHolders(st, true) }));
      setPhase('holders');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isMember, me]);

  const visitDateForPrices = state.visitDate ?? today;
  useEffect(() => {
    let live = true;
    setLoadError(false);
    commerce.products(visitDateForPrices, locale).then(
      (p) => live && setCatalog(p),
      () => live && setLoadError(true),
    );
    return () => {
      live = false;
    };
  }, [visitDateForPrices, locale]);

  useEffect(() => {
    commerce.availability(today, addDays(today, BOOK_AHEAD_DAYS - 1)).then(setAvail, () => setAvail({}));
  }, [today]);

  /** Any change after a hold invalidates it: new idempotency key, back to selection. */
  const update = useCallback((patch: Partial<CheckoutState>) => {
    setState((st) => {
      const next = { ...st, ...patch, idempotencyKey: newKey() };
      persist(next);
      return next;
    });
    setHold(null);
    setError(null);
  }, []);

  const passes: Pass[] = useMemo(() => {
    const kinds = new Set((catalog ?? []).map((p) => p.kind));
    return (['day', 'annual'] as Pass[]).filter((k) => !catalog || kinds.has(k));
  }, [catalog]);

  const ages: AgeGroup[] = useMemo(
    () => (['adult', 'child'] as AgeGroup[]).filter((a) => !catalog || !!findProduct(catalog, state.pass, a)),
    [catalog, state.pass],
  );

  const count = ticketCount(state);
  const perHolder = phase !== 'select';
  const summary = summarize(state, catalog ?? [], ctx, perHolder);
  const soldOut = (d: string) => d in avail && avail[d] <= 0;

  function scrollTop() {
    topRef.current?.scrollIntoView({ behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' });
  }

  function proceed() {
    setError(null);
    if (!state.visitDate) {
      setError('checkout.dateTitle');
      return;
    }
    if (count === 0) return;
    persist(state);
    const resume = `/${locale}/checkout?step=holders`;
    if (session.status !== 'authed') {
      router.push(`/${locale}/login?next=${encodeURIComponent(resume)}`);
      return;
    }
    if (!session.memberId) {
      router.push(`/${locale}/register/profile?next=${encodeURIComponent(resume)}`);
      return;
    }
    if (state.segment === 'thai' && me && !me.thaiVerified) {
      setError('checkout.thaiNeedsVerified');
      return;
    }
    setState((st) => ({ ...st, holders: syncHolders(st, true) }));
    setPhase('holders');
    scrollTop();
  }

  async function reserve() {
    const errs = holderErrors(state);
    if (errs.some(Boolean) || !catalog || !state.visitDate) {
      setError(errs.find(Boolean) ?? 'errors.bad_request');
      return;
    }
    setBusy(true);
    setError(null);
    persist(state);
    try {
      const h = await commerce.createOrder(state.idempotencyKey, state.visitDate, buildOrderItems(state, catalog, ctx), locale);
      setHold(h);
      setPhase('review');
      scrollTop();
    } catch (err) {
      const code = (err as ApiError).code;
      if (code === 'invalid_token' || code === 'refresh_invalid') {
        router.push(`/${locale}/login?next=${encodeURIComponent(`/${locale}/checkout?step=holders`)}`);
        return;
      }
      if (code === 'sold_out') {
        setPhase('select');
        commerce.availability(today, addDays(today, BOOK_AHEAD_DAYS - 1)).then(setAvail, () => null);
      }
      setError(`errors.${code}`);
    } finally {
      setBusy(false);
    }
  }

  function startOver() {
    clearPersisted();
    setHold(null);
    setPhase('select');
    update({ holders: [] });
  }

  const errorText = error ? (error.startsWith('errors.') ? errText(error.slice(7)) : t(error)) : null;
  const segmentLabel = state.segment === 'thai' ? t('checkout.priceThai') : t('checkout.priceForeign');

  return (
    <div className={s.page} ref={topRef}>
      <div className={`nn-container ${s.layout}`}>
        <div className={s.main}>
          <header className={s.pageHead}>
            <Steps phase={phase} />
            <h1 className={s.pageTitle}>{t('checkout.title')}</h1>
            <p className={s.pageLead}>{t('checkout.lead')}</p>
          </header>

          {loadError && <ErrorBox text={t('checkout.loadError')} />}

          {phase === 'select' && (
            <>
              {/* 1 · Choose your pass */}
              <StepCard n={1} title={t('checkout.passTitle')} sub={t('checkout.passSub')}>
                <div className={s.passGrid} role="radiogroup" aria-label={t('checkout.passTitle')}>
                  {passes.map((p) => (
                    <PassCard key={p} pass={p} selected={state.pass === p} onSelect={() => update({ pass: p })} />
                  ))}
                </div>
              </StepCard>

              {/* 2 · Who's visiting? */}
              <StepCard n={2} title={t('checkout.whoTitle')} sub={t('checkout.whoSub')}>
                <div className={s.segmented} role="radiogroup" aria-label={t('checkout.whoTitle')}>
                  {(['thai', 'foreign'] as const).map((seg) => (
                    <button
                      key={seg}
                      type="button"
                      role="radio"
                      aria-checked={state.segment === seg}
                      className={s.segment}
                      onClick={() => update({ segment: seg })}
                    >
                      {seg === 'thai' ? <ThaiFlag size={22} /> : <Icon name="globe" size={18} />}
                      {t(`checkout.${seg}`)}
                      {state.segment === seg && <Icon name="check" size={16} className={s.segCheck} />}
                    </button>
                  ))}
                </div>
                {state.segment === 'thai' && (!isMember || (me && !me.thaiVerified)) && (
                  <p className={s.notice}>
                    <Icon name="info" size={18} />
                    {isMember ? t('checkout.thaiNeedsVerified') : t('checkout.thaiNeedsLogin')}
                  </p>
                )}
              </StepCard>

              {/* 3 · Select visit date */}
              <StepCard n={3} title={t('checkout.dateTitle')} sub={state.pass === 'annual' ? t('checkout.annualDateSub') : t('checkout.dateSub')}>
                <div className={s.dateRow}>
                  <button
                    type="button"
                    className={s.dateArrow}
                    onClick={() => setStripStart((d) => (addDays(d, -STRIP) < today ? today : addDays(d, -STRIP)))}
                    disabled={stripStart <= today}
                    aria-label={t('checkout.prevDates')}
                  >
                    <Icon name="chevL" size={18} />
                  </button>
                  <ul className={s.dates} role="radiogroup" aria-label={t('checkout.dateTitle')}>
                    {Array.from({ length: STRIP }, (_, i) => addDays(stripStart, i))
                      .filter((d) => d < addDays(today, BOOK_AHEAD_DAYS))
                      .map((d) => {
                        const p = dateParts(d, locale);
                        const full = soldOut(d);
                        const sel = state.visitDate === d;
                        return (
                          <li key={d}>
                            <button
                              type="button"
                              role="radio"
                              aria-checked={sel}
                              aria-disabled={full}
                              disabled={full}
                              aria-label={`${formatVisitDate(d, locale)}${full ? ` · ${t('checkout.soldOut')}` : ''}`}
                              className={`${s.date} ${sel ? s.dateOn : ''}`}
                              onClick={() => update({ visitDate: d })}
                              data-date={d}
                            >
                              <span className={s.dateDow}>{p.weekday}</span>
                              <span className={s.dateDay}>{p.day}</span>
                              <span className={s.dateMon}>{full ? t('checkout.soldOut') : p.month}</span>
                            </button>
                          </li>
                        );
                      })}
                  </ul>
                  <button
                    type="button"
                    className={s.dateArrow}
                    onClick={() => setStripStart((d) => addDays(d, STRIP))}
                    disabled={addDays(stripStart, STRIP) >= addDays(today, BOOK_AHEAD_DAYS)}
                    aria-label={t('checkout.nextDates')}
                  >
                    <Icon name="chevR" size={18} />
                  </button>
                </div>
                <label className={s.another}>
                  <Icon name="cal" size={16} />
                  <span>{t('checkout.chooseAnother')}</span>
                  <input
                    type="date"
                    min={today}
                    max={addDays(today, BOOK_AHEAD_DAYS - 1)}
                    value={state.visitDate ?? ''}
                    onChange={(e) => {
                      const d = e.target.value;
                      if (!d || soldOut(d)) return;
                      update({ visitDate: d });
                      setStripStart(d);
                    }}
                  />
                </label>
              </StepCard>

              {/* 4 · Number of tickets */}
              <StepCard
                n={4}
                title={t('checkout.qtyTitle')}
                sub={t('checkout.qtySub', { segment: state.segment === 'thai' ? t('checkout.thai') : t('checkout.foreign'), pass: t(`checkout.${state.pass}.name`) })}
              >
                <div className={s.qtyList}>
                  {ages.map((age) => {
                    const product = catalog ? findProduct(catalog, state.pass, age) : undefined;
                    const price = unitPrice(product, state.segment);
                    const name = t(`checkout.${age}.name`);
                    return (
                      <div key={age} className={s.qtyRow}>
                        <span className={`${s.qtyIcon} ${age === 'child' ? s.qtyIconChild : ''}`} aria-hidden="true">
                          <Icon name={age} size={26} />
                        </span>
                        <div className={s.qtyText}>
                          <span className={s.qtyName}>{name}</span>
                          <span className={s.qtyHint}>{t(`checkout.${age}.hint`)}</span>
                        </div>
                        <div className={s.qtyPrice}>
                          <strong>{catalog ? formatTHB(price, locale) : '–'}</strong>
                          <span>{t('checkout.perPerson')}</span>
                        </div>
                        <Stepper
                          value={state.qty[age]}
                          min={0}
                          max={MAX_TICKETS - (count - state.qty[age])}
                          labelDec={t('checkout.decrease', { name })}
                          labelInc={t('checkout.increase', { name })}
                          onChange={(v) => update({ qty: { ...state.qty, [age]: v } })}
                        />
                      </div>
                    );
                  })}
                  <div className={s.free}>
                    <Icon name="baby" size={22} />
                    <span>{t('checkout.freeNote')}</span>
                    <span className={s.freeTag}>{t('checkout.free')}</span>
                  </div>
                  {count >= MAX_TICKETS && <p className={s.qtyHint}>{t('checkout.maxTickets')}</p>}
                </div>
              </StepCard>

              <aside className={s.note} aria-labelledby="note-title">
                <h2 id="note-title">
                  <Icon name="info" size={18} />
                  {t('checkout.noteTitle')}
                </h2>
                <p>* {t('checkout.note1')}</p>
                <p>* {t('checkout.note2')}</p>
                <p>* {t('checkout.note3')}</p>
              </aside>
            </>
          )}

          {phase === 'holders' && (
            <HoldersStep
              state={state}
              me={me}
              ctx={ctx}
              catalog={catalog ?? []}
              onChange={(holders) => update({ holders })}
              onBack={() => {
                setPhase('select');
                scrollTop();
              }}
              onReserve={reserve}
              busy={busy}
              error={errorText}
            />
          )}

          {phase === 'review' && hold && (
            <ReviewStep hold={hold} onExpired={startOver} onBack={() => setPhase('holders')} />
          )}
        </div>

        <aside className={s.summary} aria-labelledby="summary-title">
          <div className={s.summaryCard}>
            <h2 id="summary-title" className={s.summaryTitle}>
              {t('checkout.summary')}
            </h2>
            <div className={s.summaryPass}>
              <Image src={state.pass === 'day' ? '/images/pass-day.webp' : '/images/pass-annual.webp'} alt="" width={72} height={72} />
              <div>
                <strong>{t(`checkout.${state.pass}.name`)}</strong>
                <span>{perHolder ? '' : segmentLabel}</span>
              </div>
            </div>
            <dl className={s.summaryRows}>
              <div>
                <dt>
                  <Icon name="cal" size={16} />
                  {state.pass === 'annual' ? t('checkout.startDate') : t('checkout.visitDate')}
                </dt>
                <dd>{state.visitDate ? formatVisitDate(state.visitDate, locale, { dateStyle: 'medium' }) : '–'}</dd>
              </div>
              {summary.lines.map((l) => (
                <div key={`${l.age}-${l.audience}`}>
                  <dt>
                    {l.qty} × {t(`checkout.${l.age}.name`)}
                    {perHolder && <small> · {l.audience === 'thai' ? t('checkout.priceThai') : t('checkout.priceForeign')}</small>}
                  </dt>
                  <dd>{formatTHB(l.qty * l.unitSatang, locale)}</dd>
                </div>
              ))}
            </dl>
            {count === 0 && <p className={s.summaryEmpty}>{t('checkout.summaryEmpty')}</p>}
            <div className={s.total}>
              <span>{t('checkout.total')}</span>
              <strong>{formatTHB(hold?.totalSatang ?? summary.totalSatang, locale)}</strong>
            </div>
            {phase === 'select' && (
              <>
                {errorText && <ErrorBox text={errorText} />}
                <button type="button" className={`nn-btn nn-btn--gold nn-btn--block ${s.glow}`} disabled={count === 0 || !catalog} onClick={proceed}>
                  {t('checkout.continue')}
                  <Icon name="arrowR" size={20} />
                </button>
              </>
            )}
            <p className={s.secure}>
              <Icon name="lock" size={14} />
              {t('checkout.secureNote')}
            </p>
          </div>
        </aside>
      </div>
    </div>
  );
}

function Steps({ phase }: { phase: Phase }) {
  const { t } = useI18n();
  const items: [Phase, string][] = [
    ['select', t('checkout.qtyTitle')],
    ['holders', t('checkout.holdersTitle')],
    ['review', t('checkout.reviewTitle')],
  ];
  const idx = items.findIndex(([p]) => p === phase);
  return (
    <ol className={s.steps}>
      {items.map(([p, label], i) => (
        <li key={p} className={i <= idx ? s.stepOn : ''} aria-current={i === idx ? 'step' : undefined}>
          <span>{i + 1}</span>
          <span className={s.stepLabel}>{label}</span>
        </li>
      ))}
    </ol>
  );
}

function StepCard({ n, title, sub, children }: { n: number; title: string; sub: string; children: ReactNode }) {
  return (
    <section className={s.stepCard} aria-labelledby={`step-${n}`}>
      <header className={s.stepHead}>
        <span className={s.stepNum} aria-hidden="true">
          {n}
        </span>
        <div>
          <h2 id={`step-${n}`} className={s.stepTitle}>
            {title}
          </h2>
          <p className={s.stepSub}>{sub}</p>
        </div>
      </header>
      {children}
    </section>
  );
}

function PassCard({ pass, selected, onSelect }: { pass: Pass; selected: boolean; onSelect: () => void }) {
  const { t } = useI18n();
  return (
    <button type="button" role="radio" aria-checked={selected} className={`${s.pass} ${selected ? s.passOn : ''}`} onClick={onSelect}>
      <span className={s.passImg}>
        <Image src={pass === 'day' ? '/images/pass-day.webp' : '/images/pass-annual.webp'} alt="" fill sizes="(min-width: 768px) 300px, 90vw" />
        {selected ? (
          <span className={s.passBadge}>
            <Icon name="check" size={14} />
            {t('checkout.selected')}
          </span>
        ) : pass === 'annual' ? (
          <span className={`${s.passBadge} ${s.passBadgeLight}`}>
            <Icon name="star" size={14} />
            {t('checkout.bestValue')}
          </span>
        ) : null}
        <span className={s.passIcon} aria-hidden="true">
          <Icon name={pass === 'day' ? 'ticket' : 'infinity'} size={20} />
        </span>
      </span>
      <span className={s.passBody}>
        <span className={s.passName}>{t(`checkout.${pass}.name`)}</span>
        <span className={s.passTag}>{t(`checkout.${pass}.tag`)}</span>
        <span className={s.passFeatures}>
          {(['f1', 'f2', 'f3'] as const).map((f) => (
            <span key={f}>
              <Icon name="check" size={14} />
              {t(`checkout.${pass}.${f}`)}
            </span>
          ))}
        </span>
      </span>
    </button>
  );
}

function Stepper({
  value,
  min,
  max,
  labelDec,
  labelInc,
  onChange,
}: {
  value: number;
  min: number;
  max: number;
  labelDec: string;
  labelInc: string;
  onChange: (v: number) => void;
}) {
  return (
    <div className={s.stepper}>
      <button type="button" onClick={() => onChange(Math.max(min, value - 1))} disabled={value <= min} aria-label={labelDec}>
        <Icon name="minus" size={16} />
      </button>
      <output aria-live="polite">{value}</output>
      <button type="button" className={s.plus} onClick={() => onChange(Math.min(max, value + 1))} disabled={value >= max} aria-label={labelInc}>
        <Icon name="plus" size={16} />
      </button>
    </div>
  );
}

function HoldersStep({
  state,
  me,
  ctx,
  catalog,
  onChange,
  onBack,
  onReserve,
  busy,
  error,
}: {
  state: CheckoutState;
  me: Me | null;
  ctx: { thaiVerified: boolean };
  catalog: Product[];
  onChange: (h: HolderInput[]) => void;
  onBack: () => void;
  onReserve: () => void;
  busy: boolean;
  error: string | null;
}) {
  const { t, locale } = useI18n();
  const countryName = useCountryName();
  const errs = holderErrors(state);
  const [touched, setTouched] = useState(false);
  const meIndex = state.holders.findIndex((h) => h.kind === 'me');
  const set = (i: number, h: HolderInput) => onChange(state.holders.map((x, j) => (j === i ? h : x)));

  return (
    <section className={s.stepCard} aria-labelledby="holders-title">
      <header className={s.stepHead}>
        <span className={s.stepNum} aria-hidden="true">
          <Icon name="user" size={16} />
        </span>
        <div>
          <h2 id="holders-title" className={s.stepTitle}>
            {t('checkout.holdersTitle')}
          </h2>
          <p className={s.stepSub}>{t('checkout.holdersLead')}</p>
        </div>
      </header>
      {state.segment === 'thai' && (
        <p className={s.notice}>
          <Icon name="info" size={18} />
          {t('checkout.thaiPerHolder')}
        </p>
      )}
      <ol className={s.holderList}>
        {slots(state).map((age, i) => {
          const h = state.holders[i] ?? { kind: 'other', name: '', nationality: '', idLast4: '' };
          const aud = audienceFor(state, h, ctx);
          const price = unitPrice(findProduct(catalog, state.pass, age), aud);
          const canMe = !!me && (meIndex === -1 || meIndex === i);
          const err = touched ? errs[i] : null;
          return (
            <li key={i} className={s.holder}>
              <div className={s.holderHead}>
                <strong>
                  {t('checkout.ticketN', { n: i + 1 })} · {t(`checkout.${age}.name`)}
                </strong>
                <span className={s.holderPrice}>
                  {formatTHB(price, locale)}
                  <small>{aud === 'thai' ? t('checkout.priceThai') : t('checkout.priceForeign')}</small>
                </span>
              </div>
              <div className={s.holderChoice} role="radiogroup" aria-label={t('checkout.ticketN', { n: i + 1 })}>
                <button type="button" role="radio" aria-checked={h.kind === 'me'} disabled={!canMe} className={s.choice} onClick={() => set(i, { kind: 'me' })}>
                  <Icon name="user" size={16} />
                  {t('checkout.me')}
                </button>
                <button
                  type="button"
                  role="radio"
                  aria-checked={h.kind === 'other'}
                  className={s.choice}
                  onClick={() => h.kind !== 'other' && set(i, { kind: 'other', name: '', nationality: '', idLast4: '' })}
                >
                  <Icon name="adult" size={16} />
                  {t('checkout.someoneElse')}
                </button>
              </div>
              {h.kind === 'me' && me ? (
                <p className={s.meLine}>
                  <Icon name="check" size={16} />
                  {t('checkout.meLabel', { name: me.name, last4: me.idLast4 })}
                </p>
              ) : h.kind === 'other' ? (
                <div className={s.holderFields}>
                  <label className={s.hf}>
                    <span>{t('checkout.holderName')}</span>
                    <input value={h.name} autoComplete="off" onChange={(e) => set(i, { ...h, name: e.target.value })} />
                  </label>
                  <label className={s.hf}>
                    <span>{t('checkout.holderNationality')}</span>
                    <select value={h.nationality} onChange={(e) => set(i, { ...h, nationality: e.target.value })}>
                      <option value="" />
                      {COUNTRIES.map((c) => (
                        <option key={c} value={c}>
                          {countryName(c)}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className={s.hf}>
                    <span>{t('checkout.holderId')}</span>
                    <input
                      value={h.idLast4}
                      inputMode="text"
                      maxLength={4}
                      placeholder="1234"
                      autoComplete="off"
                      onChange={(e) => set(i, { ...h, idLast4: e.target.value.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 4) })}
                    />
                  </label>
                </div>
              ) : null}
              {err && (
                <p className={s.holderErr} role="alert">
                  <Icon name="alert" size={14} />
                  {t(err)}
                </p>
              )}
            </li>
          );
        })}
      </ol>
      {error && <ErrorBox text={error} />}
      <div className={s.bar}>
        <button type="button" className="nn-btn nn-btn--outline-dark" onClick={onBack}>
          <Icon name="arrowL" size={20} />
          {t('checkout.back')}
        </button>
        <button
          type="button"
          className={`nn-btn nn-btn--gold ${s.glow}`}
          disabled={busy}
          onClick={() => {
            setTouched(true);
            if (!errs.some(Boolean)) onReserve();
          }}
        >
          {busy ? t('checkout.reserving') : t('checkout.reserve')}
          {!busy && <Icon name="arrowR" size={20} />}
        </button>
      </div>
    </section>
  );
}

function ReviewStep({ hold, onExpired, onBack }: { hold: Hold; onExpired: () => void; onBack: () => void }) {
  const { t, locale } = useI18n();
  const errText = useErrorText();
  const offset = useMemo(() => serverOffset(hold.serverDate, Date.now()), [hold]);
  const [left, setLeft] = useState(() => remainingMs(hold.holdExpiresAt, offset, Date.now()));
  const [method, setMethod] = useState<'card' | 'promptpay'>('card');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const keys = useRef<Record<string, string>>({});
  const hasExpiry = !!hold.holdExpiresAt;
  const expired = hasExpiry && left <= 0;

  useEffect(() => {
    if (!hasExpiry) return;
    const id = setInterval(() => setLeft(remainingMs(hold.holdExpiresAt, offset, Date.now())), 1000);
    return () => clearInterval(id);
  }, [hold, offset, hasExpiry]);

  async function pay() {
    setBusy(true);
    setError(null);
    // One idempotency key per method choice: double clicks reuse it, so mockext creates one payment.
    keys.current[method] ??= newKey();
    try {
      const url = await commerce.pay(hold.orderId, method, `${window.location.origin}/${locale}/checkout/result/${hold.orderId}`, keys.current[method]);
      if (!url) throw new ApiError(500, 'unknown');
      clearPersisted();
      window.location.assign(url);
    } catch (err) {
      setError((err as ApiError).code);
      setBusy(false);
    }
  }

  return (
    <section className={s.stepCard} aria-labelledby="review-title">
      <header className={s.stepHead}>
        <span className={s.stepNum} aria-hidden="true">
          <Icon name="card" size={16} />
        </span>
        <div>
          <h2 id="review-title" className={s.stepTitle}>
            {t('checkout.reviewTitle')}
          </h2>
          {hold.orderNo && <p className={s.stepSub}>{t('checkout.orderNo', { no: hold.orderNo })}</p>}
        </div>
      </header>

      {hasExpiry && (
        <p className={`${s.timer} ${left < 5 * 60_000 ? s.timerLow : ''}`} role="timer">
          <Icon name="clock" size={18} />
          {expired ? t('checkout.holdExpired') : t('checkout.holdTimer', { time: mmss(left) })}
        </p>
      )}

      <fieldset className={s.methods}>
        <legend className={s.stepSub}>{t('checkout.payWith')}</legend>
        {(['card', 'promptpay'] as const).map((m) => (
          <label key={m} className={`${s.method} ${method === m ? s.methodOn : ''}`}>
            <input type="radio" name="method" value={m} checked={method === m} onChange={() => setMethod(m)} />
            <Icon name={m === 'card' ? 'card' : 'qr'} size={22} />
            <span>{t(`checkout.${m}`)}</span>
            {m === 'card' && <span className={s.brands}>VISA · Mastercard · JCB</span>}
          </label>
        ))}
      </fieldset>

      {error && <ErrorBox text={errText(error)} />}
      <div className={s.bar}>
        {expired ? (
          <button type="button" className="nn-btn nn-btn--dark" onClick={onExpired}>
            {t('checkout.startOver')}
          </button>
        ) : (
          <button type="button" className="nn-btn nn-btn--outline-dark" onClick={onBack} disabled={busy}>
            <Icon name="arrowL" size={20} />
            {t('checkout.back')}
          </button>
        )}
        <button type="button" className={`nn-btn nn-btn--gold ${s.glow}`} disabled={busy || expired} onClick={pay}>
          <Icon name="lock" size={18} />
          {busy ? t('checkout.redirecting') : t('checkout.pay', { amount: formatTHB(hold.totalSatang, locale) })}
        </button>
      </div>
      <p className={s.secure}>
        <Icon name="shield" size={14} />
        {t('checkout.secureNote')}
      </p>
    </section>
  );
}
