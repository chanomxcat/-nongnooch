'use client';

import QRCode from 'qrcode';
import { useEffect, useState } from 'react';

/** Renders the ticket's NN1.<payload>.<sig> string as-is (contracts section 6): no parsing, no verification. */
/** Design-token colour read at runtime (qrcode needs a concrete hex); fallback only before CSS loads. */
function tokenColor(name: string, fallback: string): string {
  const v = typeof document !== 'undefined' ? getComputedStyle(document.documentElement).getPropertyValue(name).trim() : '';
  return /^#[0-9a-fA-F]{6}$/.test(v) ? v : fallback;
}

export function QrCode({ value, label, className }: { value: string; label: string; className?: string }) {
  const [svg, setSvg] = useState<string>('');
  useEffect(() => {
    let live = true;
    QRCode.toString(value, { type: 'svg', errorCorrectionLevel: 'M', margin: 0, color: { dark: tokenColor('--color-green-950', '#00271C'), light: tokenColor('--color-white', '#FFFFFF') } }).then(
      (out) => live && setSvg(out),
      () => live && setSvg(''),
    );
    return () => {
      live = false;
    };
  }, [value]);
  // The SVG markup comes from the qrcode library (no user HTML), so injecting it is safe.
  return <div className={className} role="img" aria-label={label} data-qr={value} dangerouslySetInnerHTML={{ __html: svg }} />;
}
