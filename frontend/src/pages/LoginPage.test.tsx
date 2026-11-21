import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { LoginPage } from './LoginPage';
import { AuthContext } from '../context/AuthContext';

// Mock the API client
vi.mock('../api/client', () => ({
  login: vi.fn(),
  register: vi.fn(),
  fetchMe: vi.fn(),
  logout: vi.fn(),
  fetchTodos: vi.fn(),
  createTodo: vi.fn(),
  updateTodo: vi.fn(),
  deleteTodo: vi.fn(),
}));

import { login, register } from '../api/client';

function renderWithAuth(user: any = null, loading = false) {
  const mockAuth = {
    user,
    loading,
    error: null,
    logout: vi.fn(),
    checkAuth: vi.fn(),
  };
  return render(
    <MemoryRouter initialEntries={['/']}>
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

  it('renders both tabs', () => {
    renderWithAuth(null, false);

    expect(screen.getByRole('button', { name: 'Login' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Register' })).toBeInTheDocument();
  });

  it('shows GitHub login button', () => {
    renderWithAuth(null, false);

    const githubLink = screen.getByText('Login with GitHub');
    expect(githubLink).toBeInTheDocument();
    expect(githubLink).toHaveAttribute('href', '/auth/github/login');
  });

  it('shows login tab active by default', () => {
    renderWithAuth(null, false);

    const loginTab = screen.getByRole('button', { name: 'Login' });
    expect(loginTab.className).toContain('auth-tabs__tab--active');
  });

  it('switches to register tab and preserves email', async () => {
    const user = userEvent.setup();
    renderWithAuth(null, false);

    // Type email in login form
    const emailInput = screen.getByLabelText('Email');
    await user.type(emailInput, 'test@example.com');

    // Switch to register tab
    const registerTab = screen.getByRole('button', { name: 'Register' });
    await user.click(registerTab);

    // Email should be preserved
    await waitFor(() => {
      const emailInputs = screen.getAllByDisplayValue('test@example.com');
      expect(emailInputs.length).toBeGreaterThan(0);
    });
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

    // Use getAllByRole since tab and submit button both reference these names
    expect(screen.getByLabelText('Email')).toBeInTheDocument();
    expect(screen.getByLabelText('Password')).toBeInTheDocument();
    const buttons = screen.getAllByRole('button', { name: 'Sign In' });
    expect(buttons.length).toBeGreaterThan(0);
  });

  it('shows register form with confirm password when tab is clicked', async () => {
    const user = userEvent.setup();
    renderWithAuth(null, false);

    const registerTab = screen.getByRole('button', { name: 'Register' });
    await user.click(registerTab);

    expect(screen.getByLabelText('Confirm Password')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create Account' })).toBeInTheDocument();
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

describe('RegisterForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('calls register API on submit', async () => {
    const mockUser = { id: '1', github_login: '', github_avatar_url: '', display_name: 'test', created_at: '', updated_at: '' };
    vi.mocked(register).mockResolvedValueOnce(mockUser);

    const user = userEvent.setup();
    renderWithAuth(null, false);

    // Switch to register tab
    await user.click(screen.getByRole('button', { name: 'Register' }));

    await user.type(screen.getByLabelText('Email'), 'test@example.com');
    await user.type(screen.getByLabelText('Password'), 'password123');
    await user.type(screen.getByLabelText('Confirm Password'), 'password123');
    await user.click(screen.getByRole('button', { name: 'Create Account' }));

    await waitFor(() => {
      expect(register).toHaveBeenCalledWith('test@example.com', 'password123');
    });
  });

  it('shows error when passwords do not match', async () => {
    const user = userEvent.setup();
    renderWithAuth(null, false);

    // Switch to register tab
    await user.click(screen.getByRole('button', { name: 'Register' }));

    await user.type(screen.getByLabelText('Email'), 'test@example.com');
    await user.type(screen.getByLabelText('Password'), 'password123');
    await user.type(screen.getByLabelText('Confirm Password'), 'different');
    await user.click(screen.getByRole('button', { name: 'Create Account' }));

    await waitFor(() => {
      expect(screen.getByText('Passwords do not match')).toBeInTheDocument();
    });

    // register should not have been called
    expect(register).not.toHaveBeenCalled();
  });

  it('shows error message on registration failure', async () => {
    vi.mocked(register).mockRejectedValueOnce(new Error('Email already registered'));

    const user = userEvent.setup();
    renderWithAuth(null, false);

    // Switch to register tab
    await user.click(screen.getByRole('button', { name: 'Register' }));

    await user.type(screen.getByLabelText('Email'), 'test@example.com');
    await user.type(screen.getByLabelText('Password'), 'password123');
    await user.type(screen.getByLabelText('Confirm Password'), 'password123');
    await user.click(screen.getByRole('button', { name: 'Create Account' }));

    await waitFor(() => {
      expect(screen.getByText('Email already registered')).toBeInTheDocument();
    });
  });

  it('disables button while submitting', async () => {
    vi.mocked(register).mockImplementationOnce(() => new Promise(() => {})); // never resolves

    const user = userEvent.setup();
    renderWithAuth(null, false);

    // Switch to register tab
    await user.click(screen.getByRole('button', { name: 'Register' }));

    await user.type(screen.getByLabelText('Email'), 'test@example.com');
    await user.type(screen.getByLabelText('Password'), 'password123');
    await user.type(screen.getByLabelText('Confirm Password'), 'password123');
    await user.click(screen.getByRole('button', { name: 'Create Account' }));

    await waitFor(() => {
      const button = screen.getByRole('button', { name: 'Creating account...' });
      expect(button).toBeDisabled();
    });
  });
});
