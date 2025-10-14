/* eslint-disable @typescript-eslint/no-explicit-any */
import React, { useEffect, useState } from 'react';
import '../styles/notes.css';
import {
  fetchNotes as apiFetchNotes,
  createNote as apiCreateNote,
  updateNote as apiUpdateNote,
  deleteNote as apiDeleteNote
} from "../api";

interface Note {
  id: string;
  user_id: number;
  title: string;
  content: string;
  created_at: string;
  updated_at: string;
}

interface NotesListProps {
  token: string;
  userId: number;
}

const NotesList: React.FC<NotesListProps> = ({ token, userId }) => {
  const [notes, setNotes] = useState<Note[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [newTitle, setNewTitle] = useState('');
  const [newContent, setNewContent] = useState('');
  const [creating, setCreating] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editTitle, setEditTitle] = useState('');
  const [editContent, setEditContent] = useState('');
  const [updating, setUpdating] = useState(false);

  const fetchNotes = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await apiFetchNotes(token, userId);
      setNotes(data || []);
    } catch (e: any) {
      setError(e.message || 'Failed to fetch notes');
    } finally {
      setIsLoading(false);
    }
  };
  useEffect(() => { fetchNotes(); }, [userId]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newContent.trim()) return;
    setCreating(true);
    try {
      await apiCreateNote(token, userId, newTitle, newContent);
      setNewTitle('');
      setNewContent('');
      fetchNotes();
    } catch (e: any) {
      setError(e.message || 'Failed to create note');
    } finally {
      setCreating(false);
    }
  };
  const startEdit = (note: Note) => {
    setEditingId(note.id);
    setEditTitle(note.title);
    setEditContent(note.content);
  };
  const cancelEdit = () => { setEditingId(null); setEditTitle(''); setEditContent(''); };
  const handleUpdate = async (id: string) => {
    setUpdating(true);
    try {
      await apiUpdateNote(token, id, editTitle, editContent);
      fetchNotes();
      cancelEdit();
    } catch (e: any) {
      setError(e.message || 'Failed to update note');
    } finally {
      setUpdating(false);
    }
  };
  const handleDelete = async (id: string) => {
    if (!window.confirm('Are you sure you want to delete this note?')) return;
    try {
      await apiDeleteNote(token, id);
      fetchNotes();
    } catch (e: any) {
      setError(e.message || 'Failed to delete note');
    }
  };

  return (
    <div className="notes-list-container">
      <div className="notes-list-header">
        <h2>Your Notes</h2>
      </div>
      <form onSubmit={handleCreate} className="notes-form">
        <input
          className="notes-input title-input"
          type="text"
          placeholder="Title (optional)"
          value={newTitle}
          onChange={e => setNewTitle(e.target.value)}
          disabled={creating}
        />
        <input
          className="notes-input content-input"
          type="text"
          placeholder="Note content"
          value={newContent}
          onChange={e => setNewContent(e.target.value)}
          required
          disabled={creating}
        />
        <button className="notes-button" type="submit" disabled={creating}>
          {creating ? 'Adding...' : 'Add Note'}
        </button>
      </form>
      {isLoading && <div className="notes-list-loading">Loadingâ¦</div>}
      {error && <div className="notes-list-error">{error}</div>}
      <div className="notes-list-items">
        {notes.length === 0 && !isLoading && !error && (
          <div className="notes-list-empty">No notes found.</div>
        )}
        {notes.map(note => (
          <div key={note.id} className={`notes-item${editingId === note.id ? ' editing' : ''}`}>
            {editingId === note.id ? (
              <form onSubmit={e => { e.preventDefault(); handleUpdate(note.id); }} className="notes-edit-form">
                <input
                  className="notes-input title-input"
                  type="text"
                  value={editTitle}
                  onChange={e => setEditTitle(e.target.value)}
                  placeholder="Title (optional)"
                  disabled={updating}
                />
                <input
                  className="notes-input content-input"
                  type="text"
                  value={editContent}
                  onChange={e => setEditContent(e.target.value)}
                  required
                  disabled={updating}
                />
                <button className="notes-button" type="submit" disabled={updating}>Save</button>
                <button className="notes-cancel-button" type="button" onClick={cancelEdit} disabled={updating}>Cancel</button>
              </form>
            ) : (
              <>
                <div className="notes-item-title">{note.title || '(Untitled)'}</div>
                <div className="notes-item-content">{note.content}</div>
                <div className="notes-item-meta">
                  Created: {new Date(note.created_at).toLocaleString()}
                  {' | '}Last updated: {new Date(note.updated_at).toLocaleString()}
                </div>
                <div className="notes-item-actions">
                  <button className="notes-button edit" onClick={() => startEdit(note)} disabled={creating || updating}>Edit</button>
                  <button className="notes-button delete" onClick={() => handleDelete(note.id)} disabled={creating || updating}>Delete</button>
                </div>
              </>
            )}
          </div>
        ))}
      </div>
    </div>
  );
};

export default NotesList;
