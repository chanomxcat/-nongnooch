import Approvals from './Approvals';
import { me } from '@/lib/session/me';

export default function ApprovalsPage() {
  const m = me();
  return (
    <main className="page">
      <Approvals me={m.sub} roles={m.roles} />
    </main>
  );
}
