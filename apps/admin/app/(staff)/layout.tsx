import { cookies } from 'next/headers';
import { redirect } from 'next/navigation';
import { decodeStaff, isFresh } from '@/lib/session/jwt';
import Nav from '@/components/Nav';

export const dynamic = 'force-dynamic';

export default function StaffLayout({ children }: { children: React.ReactNode }) {
  const claims = decodeStaff(cookies().get('nn_staff')?.value);
  if (!claims || !isFresh(claims) || (claims.typ && claims.typ !== 'staff')) redirect('/login');
  return (
    <>
      <Nav name={claims.name ?? 'Staff'} roles={claims.roles ?? []} />
      {children}
    </>
  );
}
