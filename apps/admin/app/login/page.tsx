import LoginForm from './LoginForm';

export default function LoginPage({ searchParams }: { searchParams: { next?: string } }) {
  return (
    <main className="login">
      <div className="login-card">
        <img src="/logo.svg" alt="Nongnooch" />
        <h1>
          เข้าสู่ระบบเจ้าหน้าที่
          <small>Staff sign in</small>
        </h1>
        <LoginForm next={searchParams.next} />
      </div>
    </main>
  );
}
