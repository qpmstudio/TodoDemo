import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TodoFilter } from './TodoFilter';

describe('TodoFilter', () => {
  it('renders all three filter tabs', () => {
    render(<TodoFilter current="all" onChange={vi.fn()} />);
    expect(screen.getByText('All')).toBeInTheDocument();
    expect(screen.getByText('Active')).toBeInTheDocument();
    expect(screen.getByText('Completed')).toBeInTheDocument();
  });

  it('highlights the current filter', () => {
    render(<TodoFilter current="completed" onChange={vi.fn()} />);
    const tab = screen.getByText('Completed');
    expect(tab.className).toContain('todo-filter__tab--active');
  });

  it('calls onChange when a tab is clicked', async () => {
    const onChange = vi.fn();
    render(<TodoFilter current="all" onChange={onChange} />);
    const user = userEvent.setup();

    await user.click(screen.getByText('Active'));
    expect(onChange).toHaveBeenCalledWith('active');
  });
});
