'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { timeHHmm } from '@/lib/format';
import { createDebouncer, mapScan, type ScanApi, type ScanView } from '@/lib/scan';

const SHOW_MS = 3000;
const GATES = ['MAIN-1', 'MAIN-2', 'ANNEX-1'];
const BEEP: Record<string, [number, number]> = { ok: [880, 0.12], warn: [440, 0.25], used: [300, 0.3], bad: [220, 0.35] };

const SYSTEM_ERROR: ScanView = { tone: 'bad', title: 'ระบบขัดข้อง', en: 'SYSTEM ERROR', lines: ['ลองสแกนใหม่อีกครั้ง'] };

export default function Scanner() {
  const [gateId, setGateId] = useState('MAIN-1');
  const [manual, setManual] = useState('');
  const [view, setView] = useState<ScanView | null>(null);
  const [camState, setCamState] = useState<'idle' | 'on' | 'error'>('idle');
  const [camMsg, setCamMsg] = useState('');
  const videoRef = useRef<HTMLVideoElement>(null);
  const stopRef = useRef<(() => void) | null>(null);
  const hideRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const audioRef = useRef<AudioContext | null>(null);
  const gateRef = useRef(gateId);
  const seen = useRef(createDebouncer(SHOW_MS));
  const busy = useRef(false);

  useEffect(() => {
    try {
      const g = localStorage.getItem('nn_gate');
      if (g) setGateId(g);
    } catch {}
  }, []);
  useEffect(() => {
    gateRef.current = gateId;
    try {
      localStorage.setItem('nn_gate', gateId);
    } catch {}
  }, [gateId]);

  const beep = useCallback((tone: string) => {
    try {
      const Ctx = window.AudioContext ?? (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
      const ctx = (audioRef.current ??= new Ctx());
      const [hz, dur] = BEEP[tone] ?? BEEP.bad;
      const o = ctx.createOscillator();
      const g = ctx.createGain();
      o.frequency.value = hz;
      g.gain.value = 0.08;
      o.connect(g).connect(ctx.destination);
      o.start();
      o.stop(ctx.currentTime + dur);
    } catch {}
  }, []);

  const show = useCallback(
    (v: ScanView) => {
      setView(v);
      beep(v.tone);
      if (hideRef.current) clearTimeout(hideRef.current);
      hideRef.current = setTimeout(() => setView(null), SHOW_MS);
    },
    [beep],
  );

  const submit = useCallback(
    async (raw: string, explicit = false) => {
      const qr = raw.trim();
      if (!qr || busy.current) return;
      if (explicit) seen.current.forget(qr);
      if (!seen.current.accept(qr)) return;
      busy.current = true;
      // Only a definitive verdict keeps the presentation marked; errors allow an immediate retry.
      let definitive = false;
      try {
        const r = await fetch('/bff/ticket/gate/scans', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'nn-web' },
          body: JSON.stringify({ qr, gateId: gateRef.current }),
        });
        if (r.status === 401) {
          window.location.assign('/login?next=/gate');
          return;
        }
        if (r.status === 403) {
          show({ tone: 'bad', title: 'ไม่มีสิทธิ์', en: 'NOT ALLOWED', lines: ['บัญชีนี้สแกนไม่ได้'] });
          return;
        }
        if (r.status === 400) {
          definitive = true;
          show(mapScan({ result: 'invalid' }, timeHHmm));
          return;
        }
        if (!r.ok) {
          show(SYSTEM_ERROR);
          return;
        }
        definitive = true;
        show(mapScan((await r.json()) as ScanApi, timeHHmm));
      } catch {
        show(SYSTEM_ERROR);
      } finally {
        // Brief cooldown so a code still in view does not hammer a failing server; manual entry bypasses it.
        if (!definitive) setTimeout(() => seen.current.forget(qr), 1000);
        busy.current = false;
      }
    },
    [show],
  );

  const submitRef = useRef(submit);
  submitRef.current = submit;

  const startCam = useCallback(async () => {
    if (stopRef.current || !videoRef.current) return;
    setCamMsg('');
    // getUserMedia can hang without a device or while a permission prompt is open; hint at the paste field.
    const slow = setTimeout(() => {
      if (!stopRef.current) setCamMsg('ยังไม่ได้ภาพจากกล้อง ใช้ช่องวางรหัส QR แทนได้ · No camera feed yet, you can use the paste field');
    }, 4000);
    try {
      if (!navigator.mediaDevices?.getUserMedia) throw new Error('no-camera-api');
      const { BrowserQRCodeReader } = await import('@zxing/browser');
      const reader = new BrowserQRCodeReader(undefined, { delayBetweenScanAttempts: 150 });
      const controls = await reader.decodeFromConstraints({ video: { facingMode: { ideal: 'environment' } } }, videoRef.current, (result) => {
        if (result) submitRef.current(result.getText());
      });
      stopRef.current = () => controls.stop();
      clearTimeout(slow);
      setCamState('on');
    } catch (e) {
      clearTimeout(slow);
      setCamState('error');
      const name = (e as Error)?.name;
      setCamMsg(
        name === 'NotAllowedError'
          ? 'ไม่ได้รับอนุญาตให้ใช้กล้อง · Camera permission denied'
          : 'ไม่พบกล้อง หรือเปิดกล้องไม่ได้ (ใช้ช่องวางรหัส QR ด้านขวาได้) · No camera available, use the paste field',
      );
    }
  }, []);

  const stopCam = useCallback(() => {
    stopRef.current?.();
    stopRef.current = null;
    setCamState('idle');
  }, []);

  useEffect(() => {
    startCam();
    return () => {
      stopRef.current?.();
      stopRef.current = null;
      if (hideRef.current) clearTimeout(hideRef.current);
    };
  }, [startCam]);

  return (
    <>
      <div className="gate">
        <div>
          <div className="cam">
            <video ref={videoRef} muted playsInline aria-label="กล้องสแกน QR" style={{ display: camState === 'on' ? 'block' : 'none' }} />
            {camState === 'on' && <div className="frame" aria-hidden />}
            {camState !== 'on' && (
              <div className="cam-off">
                <p>{camMsg || 'กำลังเปิดกล้อง… · Starting camera…'}</p>
              </div>
            )}
          </div>
          <div className="row" style={{ marginTop: 'var(--spacing-12)' }}>
            {camState === 'on' ? (
              <button className="btn btn-ghost" onClick={stopCam}>
                ปิดกล้อง
              </button>
            ) : (
              <button className="btn btn-gold" onClick={startCam}>
                เปิดกล้อง / Start camera
              </button>
            )}
          </div>
        </div>

        <div className="panel">
          <h2>
            ประตู / Gate
          </h2>
          <label className="field">
            รหัสประตู · Gate ID
            <input className="in" list="gates" value={gateId} onChange={(e) => setGateId(e.target.value.trim().toUpperCase())} maxLength={32} />
            <datalist id="gates">
              {GATES.map((g) => (
                <option key={g} value={g} />
              ))}
            </datalist>
          </label>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              submit(manual, true);
              setManual('');
            }}
            style={{ display: 'contents' }}
          >
            <label className="field">
              วางหรือพิมพ์รหัส QR · Paste QR string
              <div className="row">
                <input className="in" value={manual} onChange={(e) => setManual(e.target.value)} placeholder="NN1.…" autoComplete="off" spellCheck={false} />
                <button className="btn" type="submit" disabled={!manual.trim() || !gateId}>
                  ตรวจ
                </button>
              </div>
            </label>
          </form>
          <p className="hint">สำหรับทดสอบบนโน้ตบุ๊กที่ไม่มีกล้อง · For laptops without a camera.</p>
        </div>
      </div>

      {view && (
        <div className={`result ${view.tone}`} role="alert" onClick={() => {
            seen.current.reset();
            setView(null);
          }}>
          <div className="t">{view.title}</div>
          <div className="e">{view.en}</div>
          {view.lines.map((l) => (
            <div className="l" key={l}>
              {l}
            </div>
          ))}
          <div className="x">แตะเพื่อสแกนถัดไป · tap for next scan</div>
        </div>
      )}
    </>
  );
}
