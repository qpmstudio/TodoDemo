import React, { createContext, useContext, useReducer, useCallback } from 'react';
import {
  fetchTodos,
  createTodo as apiCreateTodo,
  updateTodo as apiUpdateTodo,
  deleteTodo as apiDeleteTodo,
} from '../api/client';
import type { Todo } from '../api/client';

interface TodoState {
  todos: Todo[];
  meta: { page: number; per_page: number; total: number };
  loading: boolean;
  error: string | null;
  filter: 'all' | 'active' | 'completed';
}

type TodoAction =
  | { type: 'FETCH_START' }
  | { type: 'FETCH_SUCCESS'; todos: Todo[]; meta: TodoState['meta'] }
  | { type: 'FETCH_ERROR'; error: string }
  | { type: 'ADD_TODO'; todo: Todo }
  | { type: 'UPDATE_TODO'; todo: Todo }
  | { type: 'REMOVE_TODO'; id: string }
  | { type: 'ROLLBACK_TODO'; todo: Todo }
  | { type: 'SET_FILTER'; filter: TodoState['filter'] };

function todoReducer(state: TodoState, action: TodoAction): TodoState {
  switch (action.type) {
    case 'FETCH_START':
      return { ...state, loading: true, error: null };
    case 'FETCH_SUCCESS':
      return { ...state, loading: false, todos: action.todos, meta: action.meta };
    case 'FETCH_ERROR':
      return { ...state, loading: false, error: action.error };
    case 'ADD_TODO':
      return { ...state, todos: [action.todo, ...state.todos], meta: { ...state.meta, total: state.meta.total + 1 } };
    case 'UPDATE_TODO':
      return {
        ...state,
        todos: state.todos.map((t) => (t.id === action.todo.id ? action.todo : t)),
      };
    case 'ROLLBACK_TODO':
      return {
        ...state,
        todos: state.todos.map((t) => (t.id === action.todo.id ? action.todo : t)),
      };
    case 'REMOVE_TODO':
      return {
        ...state,
        todos: state.todos.filter((t) => t.id !== action.id),
        meta: { ...state.meta, total: state.meta.total - 1 },
      };
    case 'SET_FILTER':
      return { ...state, filter: action.filter };
    default:
      return state;
  }
}

const initialState: TodoState = {
  todos: [],
  meta: { page: 1, per_page: 20, total: 0 },
  loading: false,
  error: null,
  filter: 'all',
};

interface TodoContextValue extends TodoState {
  loadTodos: (page?: number, completed?: boolean) => Promise<void>;
  addTodo: (title: string, description: string) => Promise<boolean>;
  toggleTodo: (id: string, completed: boolean) => Promise<void>;
  editTodo: (id: string, title: string, description: string) => Promise<void>;
  removeTodo: (id: string) => Promise<void>;
  setFilter: (filter: TodoState['filter']) => void;
}

const TodoContext = createContext<TodoContextValue | null>(null);

export function TodoProvider({ children }: { children: React.ReactNode }) {
  const [state, dispatch] = useReducer(todoReducer, initialState);

  const loadTodos = useCallback(async (page = 1, completed?: boolean) => {
    dispatch({ type: 'FETCH_START' });
    try {
      const filterVal = completed !== undefined ? completed : undefined;
      const { todos, meta } = await fetchTodos(page, state.meta.per_page, filterVal);
      dispatch({ type: 'FETCH_SUCCESS', todos, meta });
    } catch (err: any) {
      dispatch({ type: 'FETCH_ERROR', error: err.message || 'Failed to load todos' });
    }
  }, [state.meta.per_page]);

  const addTodo = useCallback(async (title: string, description: string): Promise<boolean> => {
    try {
      const todo = await apiCreateTodo(title, description);
      dispatch({ type: 'ADD_TODO', todo });
      return true;
    } catch {
      return false;
    }
  }, []);

  const toggleTodo = useCallback(async (id: string, completed: boolean) => {
    const original = state.todos.find((t) => t.id === id);
    if (!original) return;

    // Optimistic update
    const optimistic = { ...original, completed, completed_at: completed ? new Date().toISOString() : null };
    dispatch({ type: 'UPDATE_TODO', todo: optimistic });

    try {
      const updated = await apiUpdateTodo(id, { completed });
      dispatch({ type: 'UPDATE_TODO', todo: updated });
    } catch {
      // Rollback
      dispatch({ type: 'ROLLBACK_TODO', todo: original });
    }
  }, [state.todos]);

  const editTodo = useCallback(async (id: string, title: string, description: string) => {
    try {
      const updated = await apiUpdateTodo(id, { title, description });
      dispatch({ type: 'UPDATE_TODO', todo: updated });
    } catch {
      // silently fail, leave inline edit
    }
  }, []);

  const removeTodo = useCallback(async (id: string) => {
    const original = state.todos.find((t) => t.id === id);
    if (!original) return;

    // Optimistic removal
    dispatch({ type: 'REMOVE_TODO', id });

    try {
      await apiDeleteTodo(id);
    } catch {
      // Rollback
      dispatch({ type: 'ADD_TODO', todo: original });
    }
  }, [state.todos]);

  const setFilter = useCallback((filter: TodoState['filter']) => {
    dispatch({ type: 'SET_FILTER', filter });
  }, []);

  return (
    <TodoContext.Provider value={{ ...state, loadTodos, addTodo, toggleTodo, editTodo, removeTodo, setFilter }}>
      {children}
    </TodoContext.Provider>
  );
}

export function useTodos() {
  const ctx = useContext(TodoContext);
  if (!ctx) throw new Error('useTodos must be used within TodoProvider');
  return ctx;
}
