import { cookies } from 'next/headers';
import { canScan, decodeStaff } from '@/lib/session/jwt';
import Scanner from './Scanner';

export default function GatePage() {
  const claims = decodeStaff(cookies().get('nn_staff')?.value);
  return (
    <main className="page">
      <div className="page-h">
        <h1>
          สแกนบัตรเข้าชม
          <small>Gate scanner</small>
        </h1>
      </div>
      {canScan(claims?.roles) ? (
        <Scanner />
      ) : (
        <div className="notice" role="status">
          <b>บัญชีนี้ไม่มีสิทธิ์สแกนที่ประตู</b>
          <p className="hint">ต้องมีบทบาท gate หรือ admin · This account needs the gate or admin role to scan tickets.</p>
        </div>
      )}
    </main>
  );
}
