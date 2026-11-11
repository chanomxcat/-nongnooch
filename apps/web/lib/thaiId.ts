/** Strip dashes / spaces; must leave exactly 13 digits. */
export function normalizeThaiId(input: string): string | null {
  const d = input.replace(/[\s-]/g, '');
  return /^\d{13}$/.test(d) ? d : null;
}

/** Thai national ID check digit: (11 − Σ d[i]×(13−i) mod 11) mod 10 === d[12]. */
export function isValidThaiId(input: string): boolean {
  const d = normalizeThaiId(input);
  if (!d) return false;
  let sum = 0;
  for (let i = 0; i < 12; i++) sum += Number(d[i]) * (13 - i);
  return (11 - (sum % 11)) % 10 === Number(d[12]);
}

/** Display format x-xxxx-xxxxx-xx-x while typing. */
export function formatThaiId(input: string): string {
  const d = input.replace(/\D/g, '').slice(0, 13);
  const parts = [d.slice(0, 1), d.slice(1, 5), d.slice(5, 10), d.slice(10, 12), d.slice(12, 13)];
  return parts.filter(Boolean).join('-');
}

export function normalizePassport(input: string): string | null {
  const p = input.toUpperCase().replace(/\s/g, '');
  return /^[A-Z0-9]{6,9}$/.test(p) ? p : null;
}

export const last4 = (id: string) => id.replace(/[\s-]/g, '').slice(-4);
