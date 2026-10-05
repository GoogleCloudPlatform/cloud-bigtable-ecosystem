'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';

export default function LoginPage() {
  const router = useRouter();
  const [checkingAuth, setCheckingAuth] = useState(true);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const userParam = params.get('user')?.trim();
    if (userParam) {
      router.replace(`/chat?user=${encodeURIComponent(userParam)}`);
      return;
    }

    fetch('/api/user', { credentials: 'include' })
      .then((res) => {
        if (res.ok) {
          router.replace('/chat');
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

      <a href="/auth/login" className="google-btn">
        Sign in with Google
      </a>
    </div>
  );
}