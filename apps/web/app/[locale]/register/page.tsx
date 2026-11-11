import { Suspense } from 'react';
import { AuthFlow } from '@/components/auth/AuthFlow';

export const metadata = { robots: { index: false } };

export default function RegisterPage() {
  return (
    <Suspense>
      <AuthFlow mode="register" />
    </Suspense>
  );
}
