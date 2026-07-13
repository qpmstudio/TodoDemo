import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TodoForm } from './TodoForm';

describe('TodoForm', () => {
  it('renders input and button', () => {
    render(<TodoForm onSubmit={vi.fn()} />);
    expect(screen.getByPlaceholderText('What needs to be done?')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add' })).toBeInTheDocument();
  });

  it('button is disabled when input is empty', () => {
    render(<TodoForm onSubmit={vi.fn()} />);
    const button = screen.getByRole('button', { name: 'Add' });
    expect(button).toBeDisabled();
  });

  it('calls onSubmit with trimmed values', async () => {
    const onSubmit = vi.fn().mockResolvedValue(true);
    render(<TodoForm onSubmit={onSubmit} />);
    const user = userEvent.setup();

    await user.type(screen.getByPlaceholderText('What needs to be done?'), '  My Todo  ');
    await user.type(screen.getByPlaceholderText('Description (optional)'), '  A note  ');
    await user.click(screen.getByRole('button', { name: 'Add' }));

    expect(onSubmit).toHaveBeenCalledWith('My Todo', 'A note');
  });

  it('clears input on successful submit', async () => {
    const onSubmit = vi.fn().mockResolvedValue(true);
    render(<TodoForm onSubmit={onSubmit} />);
    const user = userEvent.setup();

    const input = screen.getByPlaceholderText('What needs to be done?');
    await user.type(input, 'New todo');
    await user.click(screen.getByRole('button', { name: 'Add' }));

    expect(input).toHaveValue('');
  });

  it('shows error on failed submit', async () => {
    const onSubmit = vi.fn().mockResolvedValue(false);
    render(<TodoForm onSubmit={onSubmit} />);
    const user = userEvent.setup();

    await user.type(screen.getByPlaceholderText('What needs to be done?'), 'New todo');
    await user.click(screen.getByRole('button', { name: 'Add' }));

    expect(screen.getByText('Failed to create todo. Please try again.')).toBeInTheDocument();
  });

  it('does not submit empty title', async () => {
    const onSubmit = vi.fn();
    render(<TodoForm onSubmit={onSubmit} />);
    const user = userEvent.setup();

    await user.type(screen.getByPlaceholderText('What needs to be done?'), '   ');
    await user.click(screen.getByRole('button', { name: 'Add' }));

    expect(onSubmit).not.toHaveBeenCalled();
  });
});
