// Centralized and type-safe API utility for Astra frontend
export const API_BASE = "http://localhost:8000";

// ========== User Profile ==========
export interface UserProfile {
  id: number;
  username: string;
  email: string;
  full_name?: string | null;
  image_url?: string | null;
}

export interface UpdateUserProfilePayload {
  username?: string;
  email?: string;
  full_name?: string;
  image_url?: string;
}

export async function getCurrentUserProfile(token: string): Promise<UserProfile> {
  const resp = await fetch(`${API_BASE}/users/me`, {
    headers: { Authorization: token }
  });
  if (!resp.ok) throw new Error(await resp.text());
  return await resp.json();
}

export async function updateCurrentUserProfile(token: string, data: UpdateUserProfilePayload): Promise<UserProfile> {
  const resp = await fetch(`${API_BASE}/users/me`, {
    method: "PUT",
    headers: {
      'Content-Type': 'application/json',
      Authorization: token
    },
    body: JSON.stringify(data)
  });
  if (!resp.ok) throw new Error(await resp.text());
  return await resp.json();
}

// ========== Auth ==========
export async function login(username: string): Promise<{ token: string; user_id: number }> {
  const resp = await fetch(`${API_BASE}/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username })
  });
  if (!resp.ok) throw new Error(await resp.text());
  return await resp.json();
}

// ========== Chat ==========
export async function fetchChatSessions(token: string) {
  const resp = await fetch(`${API_BASE}/chat/sessions`, {
    headers: { Authorization: token }
  });
  if (!resp.ok) throw new Error(await resp.text());
  return await resp.json();
}

export async function fetchMessagesForSession(sessionId: string, token: string) {
  const resp = await fetch(`${API_BASE}/chat/session/${sessionId}/messages`, {
    headers: { Authorization: token }
  });
  if (!resp.ok) throw new Error(await resp.text());
  return await resp.json();
}

export async function deleteChatSession(sessionId: string, token: string): Promise<boolean> {
  const resp = await fetch(`${API_BASE}/chat/session/${sessionId}`, {
    method: "DELETE",
    headers: { Authorization: token }
  });
  return resp.status === 204;
}

// ========== Learnings ==========
export async function fetchLearnings(token: string, userId: number, type?: string) {
  let url = `${API_BASE}/learning/fetch/${userId}`;
  if (type && type !== 'all') {
    url = `${API_BASE}/learning/fetch/${userId}/type/${type}`;
  }
  const resp = await fetch(url, {
    headers: { Authorization: token }
  });
  if (!resp.ok) throw new Error(await resp.text());
  return await resp.json();
}

// ========== Notes ==========
export interface NoteApiPayload {
  user_id?: number;
  title?: string;
  content?: string;
  favourite?: boolean;
}

export async function fetchNotes(token: string, userId: number) {
  const resp = await fetch(`${API_BASE}/notes/user/${userId}`, {
    headers: { Authorization: token }
  });
  if (!resp.ok) throw new Error(await resp.text());
  return await resp.json();
}

export async function createNote(token: string, userId: number, title: string, content: string, favourite: boolean = false) {
  const payload: NoteApiPayload = { user_id: userId, title, content, favourite };
  const resp = await fetch(`${API_BASE}/notes/`, {
    method: "POST",
    headers: {
      'Content-Type': 'application/json',
      Authorization: token
    },
    body: JSON.stringify(payload)
  });
  if (!resp.ok) throw new Error(await resp.text());
  return await resp.json();
}

/**
 * updateNote allows to update any of title, content, or favourite. At least one must be provided.
 */
export async function updateNote(
  token: string,
  noteId: string,
  title?: string,
  content?: string,
  favourite?: boolean
) {
  const updates: NoteApiPayload = {};
  if (typeof title === 'string') updates.title = title;
  if (typeof content === 'string') updates.content = content;
  if (typeof favourite === 'boolean') updates.favourite = favourite;
  if (!updates.title && !updates.content && typeof updates.favourite !== 'boolean') {
    throw new Error('At least one field (title, content, favourite) must be provided for update');
  }
  const resp = await fetch(`${API_BASE}/notes/${noteId}`, {
    method: "PUT",
    headers: {
      'Content-Type': 'application/json',
      Authorization: token
    },
    body: JSON.stringify(updates)
  });
  if (!resp.ok) throw new Error(await resp.text());
  return await resp.json();
}

export async function deleteNote(token: string, noteId: string) {
  const resp = await fetch(`${API_BASE}/notes/${noteId}`, {
    method: "DELETE",
    headers: { Authorization: token }
  });
  if (!resp.ok) throw new Error(await resp.text());
  return true;
}
