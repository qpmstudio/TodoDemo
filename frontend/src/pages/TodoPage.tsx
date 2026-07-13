import { useEffect, useCallback } from 'react';
import { Header } from '../components/Header';
import { TodoForm } from '../components/TodoForm';
import { TodoFilter } from '../components/TodoFilter';
import { TodoList } from '../components/TodoList';
import { useTodos } from '../context/TodoContext';

export function TodoPage() {
  const {
    todos,
    loading,
    error,
    filter,
    meta,
    loadTodos,
    addTodo,
    toggleTodo,
    editTodo,
    removeTodo,
    setFilter,
  } = useTodos();

  useEffect(() => {
    const completed = filter === 'active' ? false : filter === 'completed' ? true : undefined;
    loadTodos(1, completed);
  }, [filter, loadTodos]);

  const handleFilterChange = useCallback(
    (newFilter: 'all' | 'active' | 'completed') => {
      setFilter(newFilter);
    },
    [setFilter]
  );

  const handleAddTodo = useCallback(
    async (title: string, description: string) => {
      const success = await addTodo(title, description);
      return success;
    },
    [addTodo]
  );

  const handleToggle = useCallback(
    (id: string, completed: boolean) => {
      toggleTodo(id, completed);
    },
    [toggleTodo]
  );

  const handleEdit = useCallback(
    (id: string, title: string, description: string) => {
      editTodo(id, title, description);
    },
    [editTodo]
  );

  const handleDelete = useCallback(
    (id: string) => {
      removeTodo(id);
    },
    [removeTodo]
  );

  const handleRetry = useCallback(() => {
    const completed = filter === 'active' ? false : filter === 'completed' ? true : undefined;
    loadTodos(1, completed);
  }, [filter, loadTodos]);

  return (
    <div className="todo-page">
      <Header />
      <main className="todo-page__main">
        <TodoForm onSubmit={handleAddTodo} />
        <TodoFilter current={filter} onChange={handleFilterChange} />
        {loading ? (
          <div className="loading-container">
            <div className="spinner" />
          </div>
        ) : error ? (
          <div className="error-container">
            <p className="error-message">{error}</p>
            <button className="retry-button" onClick={handleRetry}>
              Retry
            </button>
          </div>
        ) : (
          <TodoList
            todos={todos}
            onToggle={handleToggle}
            onEdit={handleEdit}
            onDelete={handleDelete}
          />
        )}
        {meta.total > 0 && !loading && (
          <p className="todo-page__count">
            {meta.total} todo{meta.total !== 1 ? 's' : ''} total
          </p>
        )}
      </main>
    </div>
  );
}
