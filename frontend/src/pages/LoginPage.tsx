import { useEffect, useState, type FormEvent } from 'react';
import { Navigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { fetchAuthProviders, login, type AuthProviders } from '../api/client';

interface FormProps {
  email: string;
  onEmailChange: (email: string) => void;
  onSuccess: () => void;
}

function LoginForm({ email, onEmailChange, onSuccess }: FormProps) {
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    setSubmitting(true);
    try {
      await login(email, password);
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form className="auth-form" onSubmit={handleSubmit}>
      {error && <div className="auth-form__error">{error}</div>}
      <div className="auth-form__field">
        <label htmlFor="login-email">Email</label>
        <input
          id="login-email"
          type="email"
          value={email}
          onChange={(e) => onEmailChange(e.target.value)}
          required
          autoComplete="email"
          placeholder="you@example.com"
        />
      </div>
      <div className="auth-form__field">
        <label htmlFor="login-password">Password</label>
        <input
          id="login-password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          autoComplete="current-password"
          placeholder="Enter your password"
        />
      </div>
      <button type="submit" className="auth-form__button" disabled={submitting}>
        {submitting ? 'Signing in...' : 'Sign In'}
      </button>
    </form>
  );
}

export function LoginPage() {
  const { user, loading, checkAuth } = useAuth();
  const [email, setEmail] = useState('');
  const [providers, setProviders] = useState<AuthProviders | null>(null);
  const [searchParams] = useSearchParams();
  const oauthError = searchParams.get('error');

  useEffect(() => {
    let cancelled = false;
    fetchAuthProviders()
      .then((result) => {
        if (!cancelled) setProviders(result);
      })
      .catch(() => {
        if (!cancelled) setProviders({ github: false, zitadel: false });
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (loading) {
    return (
      <div className="loading-container">
        <div className="spinner" />
      </div>
    );
  }

  if (user) {
    return <Navigate to="/todos" replace />;
  }

  const handleSuccess = () => {
    checkAuth();
  };

  return (
    <div className="login-page">
      <div className="login-card">
        <h1>TodoDemo</h1>
        <p className="login-card__subtitle">
          A simple todo app to keep you organized.
        </p>

        {oauthError === 'zitadel' && (
          <div className="auth-form__error" role="alert">
            ZITADEL sign-in did not complete. Please try again.
          </div>
        )}

        {oauthError === 'email_not_verified' && (
          <div className="auth-form__error" role="alert">
            <p>
              Your email is not verified yet. We tried to send you a verification email —
              check your inbox (and spam), verify the address, then sign in again.
            </p>
            {providers?.zitadel_verify_url && (
              <a href={providers.zitadel_verify_url}>Verify your email in ZITADEL</a>
            )}
          </div>
        )}

        <LoginForm email={email} onEmailChange={setEmail} onSuccess={handleSuccess} />

        {providers?.zitadel_register_url && (
          <p className="login-card__register">
            New to TodoDemo? <a href={providers.zitadel_register_url}>Create an account</a>
          </p>
        )}

        {(providers?.github !== false || providers?.zitadel) && (
          <div className="auth-separator">
            <span>or</span>
          </div>
        )}

        {providers?.github !== false && (
          <a href="/auth/github/login" className="login-card__button login-card__button--github">
            Login with GitHub
          </a>
        )}

        {providers?.zitadel && (
          <a href="/auth/zitadel/login" className="login-card__button login-card__button--zitadel">
            Login with ZITADEL
          </a>
        )}
      </div>
    </div>
  );
}
