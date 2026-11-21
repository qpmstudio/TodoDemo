import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProtectedRoute } from './ProtectedRoute';
import { AuthContext } from '../context/AuthContext';

function renderWithAuth(user: any, loading = false) {
  const mockAuth = {
    user,
    loading,
    error: null,
    logout: vi.fn(),
    checkAuth: vi.fn(),
  };
  return render(
    <MemoryRouter>
      <AuthContext.Provider value={mockAuth}>
        <ProtectedRoute>
          <div>Protected Content</div>
        </ProtectedRoute>
      </AuthContext.Provider>
    </MemoryRouter>
  );
}

describe('ProtectedRoute', () => {
  it('renders children when authenticated', () => {
    renderWithAuth({ id: '1', github_login: 'test', github_avatar_url: '', display_name: 'Test', created_at: '', updated_at: '' });
    expect(screen.getByText('Protected Content')).toBeInTheDocument();
  });

  it('shows loading spinner', () => {
    renderWithAuth(null, true);
    const spinner = document.querySelector('.spinner');
    expect(spinner).toBeInTheDocument();
  });

  it('redirects to / when not authenticated', () => {
    renderWithAuth(null, false);
    // Navigate should have been triggered
    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
  });
});
