// ThreadsPanel handles display and interaction for chat threads.
import React from "react";
import { FaTrash } from "react-icons/fa";

interface ChatSessionSummary {
  session_id: string;
  last_message: string;
  last_message_role: string;
  last_activity: string;
}

interface ThreadsPanelProps {
  threads: ChatSessionSummary[];
  activeSessionId: string;
  onSelectSession: (session_id: string) => void;
  onDeleteSession: (session_id: string) => void;
  isLoading: boolean;
}

const ThreadsPanel: React.FC<ThreadsPanelProps> = ({
  threads, activeSessionId, onSelectSession, onDeleteSession, isLoading
}) => (
  <div className="threads-panel">
    <div className="threads-header">Chat Threads</div>
    {isLoading ? (
      <div className="threads-loading">Loading...</div>
    ) : (
      <ul className="threads-list">
        {threads.length === 0 ? (
          <li className="thread-thread-empty">No chats yet.</li>
        ) : (
          threads.map((thread) => (
            <li
              key={thread.session_id}
              className={thread.session_id === activeSessionId ? "thread selected" : "thread"}
              onClick={e => {
                if ((e.target as HTMLElement).classList.contains('thread-delete-btn')) return;
                onSelectSession(thread.session_id);
              }}
              style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}
            >
              <span className="thread-title">
                {thread.last_message ? thread.session_id.slice(0, 20) : "(no message yet)"}
              </span>
              <button
                className="thread-delete-btn"
                title="Delete thread"
                onClick={e => {
                  e.stopPropagation();
                  onDeleteSession(thread.session_id);
                }}
                style={{ 
                  marginLeft: '0.75em',
                  border: 'none', 
                  background: 'transparent',
                  color: 'rgba(98, 98, 98, 1)',
                  cursor: 'pointer',
                  fontSize: '1em'
                }}
                aria-label="Delete thread"
              >
                <FaTrash />
              </button>
            </li>
          ))
        )}
      </ul>
    )}
    <button className="neon-btn thread-new-btn" onClick={() => onSelectSession("")}>+ New Chat</button>
  </div>
);

export default ThreadsPanel;
