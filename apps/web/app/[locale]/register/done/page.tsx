import { Suspense } from 'react';
import { Welcome } from '@/components/auth/Welcome';

export const metadata = { robots: { index: false } };

export default function DonePage() {
  return (
    <Suspense>
      <Welcome />
    </Suspense>
  );
}
