import { Suspense } from 'react';
import { ProfileForm } from '@/components/auth/ProfileForm';

export const metadata = { robots: { index: false } };

export default function ProfilePage() {
  return (
    <Suspense>
      <ProfileForm />
    </Suspense>
  );
}
