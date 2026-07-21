import { useState, type FormEvent } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { login, register } from '../api/client';

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

function RegisterForm({ email, onEmailChange, onSuccess }: FormProps) {
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');

    if (password !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }

    setSubmitting(true);
    try {
      await register(email, password);
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form className="auth-form" onSubmit={handleSubmit}>
      {error && <div className="auth-form__error">{error}</div>}
      <div className="auth-form__field">
        <label htmlFor="register-email">Email</label>
        <input
          id="register-email"
          type="email"
          value={email}
          onChange={(e) => onEmailChange(e.target.value)}
          required
          autoComplete="email"
          placeholder="you@example.com"
        />
      </div>
      <div className="auth-form__field">
        <label htmlFor="register-password">Password</label>
        <input
          id="register-password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          autoComplete="new-password"
          placeholder="At least 8 characters"
          minLength={8}
        />
      </div>
      <div className="auth-form__field">
        <label htmlFor="register-confirm">Confirm Password</label>
        <input
          id="register-confirm"
          type="password"
          value={confirmPassword}
          onChange={(e) => setConfirmPassword(e.target.value)}
          required
          autoComplete="new-password"
          placeholder="Re-enter your password"
        />
      </div>
      <button type="submit" className="auth-form__button" disabled={submitting}>
        {submitting ? 'Creating account...' : 'Create Account'}
      </button>
    </form>
  );
}

export function LoginPage() {
  const { user, loading, checkAuth } = useAuth();
  const [tab, setTab] = useState<'login' | 'register'>('login');
  const [email, setEmail] = useState('');

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

        <div className="auth-tabs">
          <button
            className={`auth-tabs__tab${tab === 'login' ? ' auth-tabs__tab--active' : ''}`}
            onClick={() => setTab('login')}
            type="button"
          >
            Login
          </button>
          <button
            className={`auth-tabs__tab${tab === 'register' ? ' auth-tabs__tab--active' : ''}`}
            onClick={() => setTab('register')}
            type="button"
          >
            Register
          </button>
        </div>

        {tab === 'login' ? (
          <LoginForm email={email} onEmailChange={setEmail} onSuccess={handleSuccess} />
        ) : (
          <RegisterForm email={email} onEmailChange={setEmail} onSuccess={handleSuccess} />
        )}

        <div className="auth-separator">
          <span>or</span>
        </div>

        <a href="/auth/github/login" className="login-card__button login-card__button--github">
          Login with GitHub
        </a>
      </div>
    </div>
  );
}
