type FilterValue = 'all' | 'active' | 'completed';

interface TodoFilterProps {
  current: FilterValue;
  onChange: (filter: FilterValue) => void;
}

export function TodoFilter({ current, onChange }: TodoFilterProps) {
  const filters: { key: FilterValue; label: string }[] = [
    { key: 'all', label: 'All' },
    { key: 'active', label: 'Active' },
    { key: 'completed', label: 'Completed' },
  ];

  return (
    <div className="todo-filter">
      {filters.map((f) => (
        <button
          key={f.key}
          className={`todo-filter__tab ${current === f.key ? 'todo-filter__tab--active' : ''}`}
          onClick={() => onChange(f.key)}
        >
          {f.label}
        </button>
      ))}
    </div>
  );
}
