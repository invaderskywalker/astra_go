// Centralized API utility for Astra frontend
export const API_BASE = "http://localhost:8000";

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
export async function fetchNotes(token: string, userId: number) {
  const resp = await fetch(`${API_BASE}/notes/user/${userId}`, {
    headers: { Authorization: token }
  });
  if (!resp.ok) throw new Error(await resp.text());
  return await resp.json();
}

export async function createNote(token: string, userId: number, title: string, content: string) {
  const resp = await fetch(`${API_BASE}/notes/`, {
    method: "POST",
    headers: {
      'Content-Type': 'application/json',
      Authorization: token
    },
    body: JSON.stringify({ user_id: userId, title, content })
  });
  if (!resp.ok) throw new Error(await resp.text());
  return await resp.json();
}

export async function updateNote(token: string, noteId: string, title: string, content: string) {
  const resp = await fetch(`${API_BASE}/notes/${noteId}`, {
    method: "PUT",
    headers: {
      'Content-Type': 'application/json',
      Authorization: token
    },
    body: JSON.stringify({ title, content })
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
