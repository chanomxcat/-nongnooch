import { cookies } from 'next/headers';
import { decodeStaff } from './jwt';

/** The signed-in staff (layout already redirected when missing). */
export function me() {
  const c = decodeStaff(cookies().get('nn_staff')?.value);
  return { sub: c?.sub ?? '', roles: c?.roles ?? [] };
}
