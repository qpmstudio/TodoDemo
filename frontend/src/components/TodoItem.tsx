import { useState, useRef, useEffect } from 'react';
import type { Todo } from '../api/client';

interface TodoItemProps {
  todo: Todo;
  onToggle: (id: string, completed: boolean) => void;
  onEdit: (id: string, title: string, description: string) => void;
  onDelete: (id: string) => void;
}

export function TodoItem({ todo, onToggle, onEdit, onDelete }: TodoItemProps) {
  const [editing, setEditing] = useState(false);
  const [editTitle, setEditTitle] = useState(todo.title);
  const [editDesc, setEditDesc] = useState(todo.description);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (editing && inputRef.current) {
      inputRef.current.focus();
    }
  }, [editing]);

  const handleSave = () => {
    const trimmed = editTitle.trim();
    if (trimmed) {
      onEdit(todo.id, trimmed, editDesc.trim());
    }
    setEditing(false);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') handleSave();
    if (e.key === 'Escape') {
      setEditTitle(todo.title);
      setEditDesc(todo.description);
      setEditing(false);
    }
  };

  return (
    <li className={`todo-item ${todo.completed ? 'todo-item--completed' : ''}`}>
      {editing ? (
        <div className="todo-item__edit">
          <input
            ref={inputRef}
            type="text"
            className="todo-item__edit-input"
            value={editTitle}
            onChange={(e) => setEditTitle(e.target.value)}
            onKeyDown={handleKeyDown}
            onBlur={handleSave}
            maxLength={500}
          />
          <input
            type="text"
            className="todo-item__edit-desc"
            value={editDesc}
            onChange={(e) => setEditDesc(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Description (optional)"
          />
        </div>
      ) : (
        <>
          <input
            type="checkbox"
            className="todo-item__checkbox"
            checked={todo.completed}
            onChange={() => onToggle(todo.id, !todo.completed)}
          />
          <div
            className="todo-item__content"
            onDoubleClick={() => setEditing(true)}
          >
            <span className="todo-item__title">{todo.title}</span>
            {todo.description && (
              <span className="todo-item__description">{todo.description}</span>
            )}
            <span className="todo-item__timestamp">
              {todo.completed && todo.completed_at
                ? `Completed: ${new Date(todo.completed_at).toLocaleDateString()}`
                : `Created: ${new Date(todo.created_at).toLocaleDateString()}`}
            </span>
          </div>
          <button
            className="todo-item__delete"
            onClick={() => onDelete(todo.id)}
            title="Delete todo"
          >
            ✕
          </button>
        </>
      )}
    </li>
  );
}
