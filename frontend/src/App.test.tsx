import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import App from './App';

vi.mock('./api/client', () => ({
  fetchMe: vi.fn().mockRejectedValue(new Error('unauthorized')),
  fetchAuthProviders: vi.fn().mockResolvedValue({ github: true, zitadel: true }),
  login: vi.fn(),
  register: vi.fn(),
  logout: vi.fn(),
  fetchTodos: vi.fn(),
  createTodo: vi.fn(),
  updateTodo: vi.fn(),
  deleteTodo: vi.fn(),
}));

describe('App routes', () => {
  beforeEach(() => {
    window.history.pushState({}, '', '/');
  });

  afterEach(() => {
    window.history.pushState({}, '', '/');
  });

  it('renders the login page at /login', async () => {
    window.history.pushState({}, '', '/login');
    render(<App />);

    expect(await screen.findByRole('button', { name: 'Login' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Register' })).toBeInTheDocument();
  });

  it('shows the ZITADEL error banner at /login?error=zitadel', async () => {
    window.history.pushState({}, '', '/login?error=zitadel');
    render(<App />);

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'ZITADEL sign-in did not complete. Please try again.'
    );
  });

  it('still renders NotFound for unknown routes', async () => {
    window.history.pushState({}, '', '/nope');
    render(<App />);

    expect(await screen.findByText('Page not found')).toBeInTheDocument();
  });
});
