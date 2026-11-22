import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { LoginPage } from './LoginPage';
import { AuthContext } from '../context/AuthContext';

// Mock the API client
vi.mock('../api/client', () => ({
  fetchAuthProviders: vi.fn().mockResolvedValue({ github: true, zitadel: false }),
  login: vi.fn(),
  fetchMe: vi.fn(),
  logout: vi.fn(),
  fetchTodos: vi.fn(),
  createTodo: vi.fn(),
  updateTodo: vi.fn(),
  deleteTodo: vi.fn(),
}));

import { fetchAuthProviders, login } from '../api/client';

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

  it('does not render a self-service register form', () => {
    renderWithAuth(null, false);

    expect(screen.queryByRole('button', { name: 'Register' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Create Account' })).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Confirm Password')).not.toBeInTheDocument();
  });

  it('shows GitHub login button', () => {
    renderWithAuth(null, false);

    const githubLink = screen.getByText('Login with GitHub');
    expect(githubLink).toBeInTheDocument();
    expect(githubLink).toHaveAttribute('href', '/auth/github/login');
  });

  it('shows ZITADEL login button when the provider is enabled', async () => {
    vi.mocked(fetchAuthProviders).mockResolvedValueOnce({ github: true, zitadel: true });
    renderWithAuth(null, false);

    const zitadelLink = await screen.findByText('Login with ZITADEL');
    expect(zitadelLink).toHaveAttribute('href', '/auth/zitadel/login');
  });

  it('hides ZITADEL login button when the provider is disabled', async () => {
    renderWithAuth(null, false);

    await waitFor(() => expect(fetchAuthProviders).toHaveBeenCalled());
    expect(screen.queryByText('Login with ZITADEL')).not.toBeInTheDocument();
  });

  it('links account creation to the ZITADEL register page', async () => {
    vi.mocked(fetchAuthProviders).mockResolvedValueOnce({
      github: true,
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

  it('shows loading spinner when loading', () => {
    renderWithAuth(null, true);

    const spinner = document.querySelector('.spinner');
    expect(spinner).toBeInTheDocument();
  });

  it('redirects to /todos when authenticated', () => {
    const authedUser = {
      id: '1',
      github_login: '',
      github_avatar_url: '',
      display_name: 'Test',
      created_at: '',
      updated_at: '',
    };
    renderWithAuth(authedUser, false);

    // Should redirect, so login card should not be visible
    expect(screen.queryByText('TodoDemo')).not.toBeInTheDocument();
  });

  it('shows login form fields', () => {
    renderWithAuth(null, false);

    expect(screen.getByLabelText('Email')).toBeInTheDocument();
    expect(screen.getByLabelText('Password')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sign In' })).toBeInTheDocument();
  });
});

describe('LoginForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('calls login API on submit', async () => {
    const mockUser = { id: '1', github_login: '', github_avatar_url: '', display_name: 'test', created_at: '', updated_at: '' };
    vi.mocked(login).mockResolvedValueOnce(mockUser);

    const user = userEvent.setup();
    renderWithAuth(null, false);

    await user.type(screen.getByLabelText('Email'), 'test@example.com');
    await user.type(screen.getByLabelText('Password'), 'password123');
    await user.click(screen.getByRole('button', { name: 'Sign In' }));

    await waitFor(() => {
      expect(login).toHaveBeenCalledWith('test@example.com', 'password123');
    });
  });

  it('shows error message on login failure', async () => {
    vi.mocked(login).mockRejectedValueOnce(new Error('Invalid email or password'));

    const user = userEvent.setup();
    renderWithAuth(null, false);

    await user.type(screen.getByLabelText('Email'), 'test@example.com');
    await user.type(screen.getByLabelText('Password'), 'wrong');
    await user.click(screen.getByRole('button', { name: 'Sign In' }));

    await waitFor(() => {
      expect(screen.getByText('Invalid email or password')).toBeInTheDocument();
    });
  });

  it('disables button while submitting', async () => {
    vi.mocked(login).mockImplementationOnce(() => new Promise(() => {})); // never resolves

    const user = userEvent.setup();
    renderWithAuth(null, false);

    await user.type(screen.getByLabelText('Email'), 'test@example.com');
    await user.type(screen.getByLabelText('Password'), 'password123');
    await user.click(screen.getByRole('button', { name: 'Sign In' }));

    await waitFor(() => {
      const button = screen.getByRole('button', { name: 'Signing in...' });
      expect(button).toBeDisabled();
    });
  });
});
