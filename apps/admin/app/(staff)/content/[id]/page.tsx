import Editor from './Editor';
import { me } from '@/lib/session/me';

export default function EditorPage({ params }: { params: { id: string } }) {
  const m = me();
  return (
    <main className="page">
      <Editor id={params.id} roles={m.roles} />
    </main>
  );
}
