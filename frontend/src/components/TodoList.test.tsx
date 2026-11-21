import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TodoList } from './TodoList';
import type { Todo } from '../api/client';

const todos: Todo[] = [
  { id: '1', title: 'First', description: '', completed: false, completed_at: null, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' },
  { id: '2', title: 'Second', description: '', completed: true, completed_at: '2026-01-02T00:00:00Z', created_at: '2026-01-02T00:00:00Z', updated_at: '2026-01-02T00:00:00Z' },
];

describe('TodoList', () => {
  it('renders list of todos', () => {
    render(
      <TodoList todos={todos} onToggle={vi.fn()} onEdit={vi.fn()} onDelete={vi.fn()} />
    );
    expect(screen.getByText('First')).toBeInTheDocument();
    expect(screen.getByText('Second')).toBeInTheDocument();
  });

  it('renders empty state', () => {
    render(
      <TodoList todos={[]} onToggle={vi.fn()} onEdit={vi.fn()} onDelete={vi.fn()} />
    );
    expect(screen.getByText(/No todos yet/)).toBeInTheDocument();
  });
});
