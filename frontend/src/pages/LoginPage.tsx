import { Navigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

export function LoginPage() {
  const { user, loading } = useAuth();

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

  return (
    <div className="login-page">
      <div className="login-card">
        <h1>TodoDemo</h1>
        <p className="login-card__subtitle">
          A simple todo app to keep you organized.
        </p>
        <a href="/auth/github/login" className="login-card__button">
          Login with GitHub
        </a>
      </div>
    </div>
  );
}
