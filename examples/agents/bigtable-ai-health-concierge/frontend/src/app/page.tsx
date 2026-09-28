'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';

export default function LoginPage() {
  const router = useRouter();
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [backendBase, setBackendBase] = useState('http://127.0.0.1:5000');

  useEffect(() => {
    const host = typeof window !== 'undefined' ? window.location.hostname : '127.0.0.1';
    const base = `http://${host}:5000`;
    setBackendBase(base);

    const params = new URLSearchParams(window.location.search);
    const userParam = params.get('user')?.trim();
    const userUrl = userParam
      ? `${base}/api/user?user=${encodeURIComponent(userParam)}`
      : `${base}/api/user`;

    fetch(userUrl, { credentials: 'include' })
      .then((res) => {
        if (res.ok) {
          router.push(userParam ? `/chat?user=${encodeURIComponent(userParam)}` : '/chat');
        } else {
          setCheckingAuth(false);
        }
      })
      .catch(() => {
        setCheckingAuth(false);
      });
  }, [router]);

  if (checkingAuth) {
    return <div style={{ padding: '2rem' }}>Checking authentication...</div>;
  }

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

      <a href={`${backendBase}/auth/login`} className="google-btn">
        Sign in with Google
      </a>
    </div>
  );
}