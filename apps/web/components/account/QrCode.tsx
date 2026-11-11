'use client';

import QRCode from 'qrcode';
import { useEffect, useState } from 'react';

/** Renders the ticket's NN1.<payload>.<sig> string as-is (contracts section 6): no parsing, no verification. */
export function QrCode({ value, label, className }: { value: string; label: string; className?: string }) {
  const [svg, setSvg] = useState<string>('');
  useEffect(() => {
    let live = true;
    QRCode.toString(value, { type: 'svg', errorCorrectionLevel: 'M', margin: 0, color: { dark: '#00271C', light: '#FFFFFF' } }).then(
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
