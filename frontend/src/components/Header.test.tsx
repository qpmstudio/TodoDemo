import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Header } from './Header';
import { AuthContext } from '../context/AuthContext';
import { MemoryRouter } from 'react-router-dom';

// Helper to wrap component with mock auth
function renderWithAuth(user: any, logout = vi.fn()) {
  const mockContext = {
    user,
    loading: false,
    error: null,
    logout,
    checkAuth: vi.fn(),
  };

  // We need to override the context. Let's test Header with the real provider pattern
  // For unit testing, we'll use a wrapper approach
  return render(
    <MemoryRouter>
      <AuthContext.Provider value={mockContext}>
        <Header />
      </AuthContext.Provider>
    </MemoryRouter>
  );
}

describe('Header', () => {
  it('renders logo', () => {
    renderWithAuth({
      id: '1',
      github_login: 'test',
      github_avatar_url: 'https://example.com/avatar.png',
      display_name: 'Test User',
      created_at: '',
      updated_at: '',
    });
    expect(screen.getByText('TodoDemo')).toBeInTheDocument();
  });

  it('shows user info when authenticated', () => {
    renderWithAuth({
      id: '1',
      github_login: 'test',
      github_avatar_url: 'https://example.com/avatar.png',
      display_name: 'Test User',
      created_at: '',
      updated_at: '',
    });
    expect(screen.getByText('Test User')).toBeInTheDocument();
    expect(screen.getByText('Logout')).toBeInTheDocument();
  });

  it('calls logout on button click', async () => {
    const logoutMock = vi.fn();
    renderWithAuth({
      id: '1',
      github_login: 'test',
      github_avatar_url: 'https://example.com/avatar.png',
      display_name: 'Test User',
      created_at: '',
      updated_at: '',
    }, logoutMock);
    const user = userEvent.setup();
    await user.click(screen.getByText('Logout'));
    expect(logoutMock).toHaveBeenCalled();
  });
});
