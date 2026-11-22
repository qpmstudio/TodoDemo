import { useEffect, useState } from 'react';
import { Navigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { fetchAuthProviders, type AuthProviders } from '../api/client';

export function LoginPage() {
  const { user, loading } = useAuth();
  const [providers, setProviders] = useState<AuthProviders | null>(null);
  const [searchParams] = useSearchParams();
  const oauthError = searchParams.get('error');
  const verificationResent = searchParams.get('resent');

  useEffect(() => {
    let cancelled = false;
    fetchAuthProviders()
      .then((result) => {
        if (!cancelled) setProviders(result);
      })
      .catch(() => {
        if (!cancelled) setProviders({ zitadel: false });
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
              {verificationResent === '1'
                ? 'Your email is not verified yet. We sent a new verification email — ' +
                  'check your inbox (and spam), verify the address, then sign in again.'
                : verificationResent === '0'
                  ? 'Your email is not verified yet and we could not send a new verification ' +
                    'email automatically. Request one from ZITADEL below, then sign in again.'
                  : 'Your email is not verified yet. Please complete the verification email, ' +
                    'then sign in again.'}
            </p>
            {providers?.zitadel_verify_url && (
              <a href={providers.zitadel_verify_url}>Verify your email in ZITADEL</a>
            )}
          </div>
        )}

        {providers?.zitadel ? (
          <a href="/auth/zitadel/login" className="login-card__button login-card__button--zitadel">
            Login with ZITADEL
          </a>
        ) : (
          <p className="login-card__hint">Sign-in is temporarily unavailable.</p>
        )}

        {providers?.zitadel_register_url && (
          <p className="login-card__register">
            New to TodoDemo? <a href={providers.zitadel_register_url}>Create an account</a>
          </p>
        )}
      </div>
    </div>
  );
}
