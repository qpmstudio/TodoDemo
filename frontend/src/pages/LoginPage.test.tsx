import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { LoginPage } from './LoginPage';
import { AuthContext } from '../context/AuthContext';

// Mock the API client
vi.mock('../api/client', () => ({
  fetchAuthProviders: vi.fn().mockResolvedValue({ zitadel: false }),
  fetchMe: vi.fn(),
  logout: vi.fn(),
  fetchTodos: vi.fn(),
  createTodo: vi.fn(),
  updateTodo: vi.fn(),
  deleteTodo: vi.fn(),
}));

import { fetchAuthProviders } from '../api/client';

function renderWithAuth(user: any = null, loading = false, initialEntry = '/') {
  const mockAuth = {
    user,
    loading,
    error: null,
    logout: vi.fn(),
    checkAuth: vi.fn(),
  };
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <AuthContext.Provider value={mockAuth}>
        <LoginPage />
      </AuthContext.Provider>
    </MemoryRouter>
  );
}

describe('LoginPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('does not render any self-built sign-in form', () => {
    renderWithAuth(null, false);

    expect(screen.queryByLabelText('Email')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Password')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Sign In' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Register' })).not.toBeInTheDocument();
    expect(screen.queryByText('Login with GitHub')).not.toBeInTheDocument();
  });

  it('shows the ZITADEL login button when the provider is enabled', async () => {
    vi.mocked(fetchAuthProviders).mockResolvedValueOnce({ zitadel: true });
    renderWithAuth(null, false);

    const zitadelLink = await screen.findByText('Login with ZITADEL');
    expect(zitadelLink).toHaveAttribute('href', '/auth/zitadel/login');
  });

  it('hides the ZITADEL login button when the provider is disabled', async () => {
    renderWithAuth(null, false);

    await waitFor(() => expect(fetchAuthProviders).toHaveBeenCalled());
    expect(screen.queryByText('Login with ZITADEL')).not.toBeInTheDocument();
    expect(screen.getByText('Sign-in is temporarily unavailable.')).toBeInTheDocument();
  });

  it('links account creation to the ZITADEL register page', async () => {
    vi.mocked(fetchAuthProviders).mockResolvedValueOnce({
      zitadel: true,
      zitadel_register_url: 'https://auth.example.com/ui/v2/login/register',
    });
    renderWithAuth(null, false);

    const link = await screen.findByText('Create an account');
    expect(link).toHaveAttribute('href', 'https://auth.example.com/ui/v2/login/register');
  });

  it('hides the account creation link when ZITADEL is not configured', async () => {
    renderWithAuth(null, false);

    await waitFor(() => expect(fetchAuthProviders).toHaveBeenCalled());
    expect(screen.queryByText('Create an account')).not.toBeInTheDocument();
  });

  it('shows an error banner after a cancelled ZITADEL login', () => {
    renderWithAuth(null, false, '/?error=zitadel');

    expect(screen.getByRole('alert')).toHaveTextContent('ZITADEL sign-in did not complete');
  });

  it('shows an error banner when the email is not verified', () => {
    renderWithAuth(null, false, '/?error=email_not_verified');

    expect(screen.getByRole('alert')).toHaveTextContent('Your email is not verified yet');
  });

  it('tells the user a new verification email was sent', () => {
    renderWithAuth(null, false, '/?error=email_not_verified&resent=1');

    expect(screen.getByRole('alert')).toHaveTextContent('We sent a new verification email');
  });

  it('tells the user when the resend failed', () => {
    renderWithAuth(null, false, '/?error=email_not_verified&resent=0');

    expect(screen.getByRole('alert')).toHaveTextContent('we could not send a new verification email');
  });

  it('links the unverified-email banner to the ZITADEL verify page', async () => {
    vi.mocked(fetchAuthProviders).mockResolvedValueOnce({
      zitadel: true,
      zitadel_verify_url: 'https://auth.example.com/ui/v2/login',
    });
    renderWithAuth(null, false, '/?error=email_not_verified');

    const link = await screen.findByText('Verify your email in ZITADEL');
    expect(link).toHaveAttribute('href', 'https://auth.example.com/ui/v2/login');
  });

  it('shows loading spinner when loading', () => {
    renderWithAuth(null, true);

    const spinner = document.querySelector('.spinner');
    expect(spinner).toBeInTheDocument();
  });

  it('redirects to /todos when authenticated', () => {
    const authedUser = {
      id: '1',
      display_name: 'Test',
      avatar_url: '',
      created_at: '',
      updated_at: '',
    };
    renderWithAuth(authedUser, false);

    // Should redirect, so login card should not be visible
    expect(screen.queryByText('TodoDemo')).not.toBeInTheDocument();
  });
});
