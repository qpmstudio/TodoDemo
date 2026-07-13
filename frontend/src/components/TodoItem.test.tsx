import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TodoItem } from './TodoItem';
import type { Todo } from '../api/client';

const todo: Todo = {
  id: '1',
  title: 'Test Todo',
  description: 'Test description',
  completed: false,
  completed_at: null,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

describe('TodoItem', () => {
  it('renders title and description', () => {
    render(
      <TodoItem todo={todo} onToggle={vi.fn()} onEdit={vi.fn()} onDelete={vi.fn()} />
    );
    expect(screen.getByText('Test Todo')).toBeInTheDocument();
    expect(screen.getByText('Test description')).toBeInTheDocument();
  });

  it('calls onToggle when checkbox is clicked', async () => {
    const onToggle = vi.fn();
    render(
      <TodoItem todo={todo} onToggle={onToggle} onEdit={vi.fn()} onDelete={vi.fn()} />
    );
    const user = userEvent.setup();

    await user.click(screen.getByRole('checkbox'));
    expect(onToggle).toHaveBeenCalledWith('1', true);
  });

  it('calls onDelete when delete button is clicked', async () => {
    const onDelete = vi.fn();
    render(
      <TodoItem todo={todo} onToggle={vi.fn()} onEdit={vi.fn()} onDelete={onDelete} />
    );
    const user = userEvent.setup();

    await user.click(screen.getByTitle('Delete todo'));
    expect(onDelete).toHaveBeenCalledWith('1');
  });

  it('shows completed state', () => {
    const completedTodo = { ...todo, completed: true, completed_at: '2026-01-02T00:00:00Z' };
    render(
      <TodoItem todo={completedTodo} onToggle={vi.fn()} onEdit={vi.fn()} onDelete={vi.fn()} />
    );
    expect(screen.getByRole('checkbox')).toBeChecked();
  });

  it('enters edit mode on double click', async () => {
    render(
      <TodoItem todo={todo} onToggle={vi.fn()} onEdit={vi.fn()} onDelete={vi.fn()} />
    );
    const user = userEvent.setup();

    await user.dblClick(screen.getByText('Test Todo'));
    const input = screen.getByDisplayValue('Test Todo');
    expect(input).toBeInTheDocument();
  });
});
