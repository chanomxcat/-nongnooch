import { Suspense } from 'react';
import ContentList from './ContentList';
import { me } from '@/lib/session/me';

export default function ContentPage() {
  return (
    <main className="page">
      <Suspense fallback={<div className="skel" />}>
        <ContentList roles={me().roles} />
      </Suspense>
    </main>
  );
}
