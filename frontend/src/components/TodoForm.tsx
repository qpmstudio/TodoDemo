import { useState, type FormEvent } from 'react';

interface TodoFormProps {
  onSubmit: (title: string, description: string) => Promise<boolean>;
}

export function TodoForm({ onSubmit }: TodoFormProps) {
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    const trimmed = title.trim();
    if (!trimmed) return;

    setSubmitting(true);
    setError(null);

    const success = await onSubmit(trimmed, description.trim());
    if (success) {
      setTitle('');
      setDescription('');
    } else {
      setError('Failed to create todo. Please try again.');
    }
    setSubmitting(false);
  };

  return (
    <form className="todo-form" onSubmit={handleSubmit}>
      <div className="todo-form__row">
        <input
          type="text"
          className="todo-form__input"
          placeholder="What needs to be done?"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          disabled={submitting}
          maxLength={500}
          autoFocus
        />
        <button
          type="submit"
          className="todo-form__button"
          disabled={submitting || !title.trim()}
        >
          {submitting ? 'Adding...' : 'Add'}
        </button>
      </div>
      <input
        type="text"
        className="todo-form__description"
        placeholder="Description (optional)"
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        disabled={submitting}
      />
      {error && <p className="todo-form__error">{error}</p>}
    </form>
  );
}
