const API_BASE = '/api/v1';

interface ApiResponse<T> {
  data: T | null;
  error: { code: string; message: string } | null;
}

interface ListResponse<T> {
  data: T[];
  meta: { page: number; per_page: number; total: number };
  error: null;
}

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${url}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });

  if (res.status === 204) {
    return undefined as T;
  }

  const json = await res.json();

  if (!res.ok) {
    const msg = json?.error?.message || `Request failed with status ${res.status}`;
    throw new Error(msg);
  }

  return json;
}

export interface Todo {
  id: string;
  title: string;
  description: string;
  completed: boolean;
  completed_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface TodoListMeta {
  page: number;
  per_page: number;
  total: number;
}

export interface User {
  id: string;
  github_login: string;
  github_avatar_url: string;
  display_name: string;
  created_at: string;
  updated_at: string;
}

export async function fetchMe(): Promise<User> {
  const res = await request<ApiResponse<User>>('/auth/me');
  return res.data!;
}

export async function logout(): Promise<void> {
  await request('/auth/logout', { method: 'POST' });
}

export async function fetchTodos(
  page = 1,
  perPage = 20,
  completed?: boolean
): Promise<{ todos: Todo[]; meta: TodoListMeta }> {
  const params = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  if (completed !== undefined) {
    params.set('completed', String(completed));
  }
  const res = await request<ListResponse<Todo>>(`/todos?${params}`);
  return { todos: res.data, meta: res.meta };
}

export async function createTodo(title: string, description: string): Promise<Todo> {
  const res = await request<ApiResponse<Todo>>('/todos', {
    method: 'POST',
    body: JSON.stringify({ title, description }),
  });
  return res.data!;
}

export async function updateTodo(
  id: string,
  updates: { title?: string; description?: string; completed?: boolean }
): Promise<Todo> {
  const res = await request<ApiResponse<Todo>>(`/todos/${id}`, {
    method: 'PUT',
    body: JSON.stringify(updates),
  });
  return res.data!;
}

export async function deleteTodo(id: string): Promise<void> {
  await request(`/todos/${id}`, { method: 'DELETE' });
}
