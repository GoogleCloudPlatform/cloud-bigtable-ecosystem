'use client';

export default function LoginPage() {
  return (
    <div className="login-card">
      <img
        src="/cymbal.png"
        alt="Cymbal Logo"
        width="180"
        style={{ marginBottom: '2rem' }}
      />
      <h1>Personal Health Concierge</h1>
      <p>Log in to access your secure, Bigtable-powered AI health companion.</p>

      <a href="http://127.0.0.1:5000/auth/login" className="google-btn">
        Sign in with Google
      </a>
    </div>
  );
}
