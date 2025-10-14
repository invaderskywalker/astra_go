/* eslint-disable @typescript-eslint/no-unused-vars */
// ThreadsPanel handles display and interaction for chat threads, now with expandable/minimizable sidebar.
import React, { useState } from "react";
import { FaTrash, FaBars, FaChevronLeft } from "react-icons/fa";

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
}) => {
  const [expanded, setExpanded] = useState(true);
  // Optionally, persist expanded/minimized state via local storage if desired

  return (
    <div
      className={`threads-panel${expanded ? " expanded" : " minimized"}`}
    >
      <div
        className="threads-header"
      >
        {expanded ? "Chat Threads" : null}
        <button
          aria-label={expanded ? "Minimize threads panel" : "Expand threads panel"}
          title={expanded ? "Minimize" : "Expand"}
          onClick={() => setExpanded(x => !x)}
        >
          {expanded ? <FaChevronLeft size={18} /> : <FaBars size={22} />}
        </button>
      </div>
      {expanded ? (
        <>
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
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                    }}
                  >
                    <span className="thread-title" style={{ fontSize: 14 }}>
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
          <button
            className="neon-btn thread-new-btn"
            onClick={() => onSelectSession("")}
            style={{
              margin: 12,
              minHeight: 36,
              fontSize: 16,
              fontWeight: 700,
              width: "auto"
            }}
          >
            + New Chat
          </button>
        </>
      ) : (
        // Slim view â only show icons for main actions (add new chat)
        <div style={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "flex-start",
          paddingTop: 16,
          flex: 1,
          width: "100%"
        }}>
          <button
            className="neon-btn thread-new-btn"
            onClick={() => onSelectSession("")}
            style={{
              minWidth: 0,
              minHeight: 40,
              width: 36,
              height: 36,
              padding: 0,
              fontSize: 24,
              borderRadius: 18,
              display: "flex",
              justifyContent: "center",
              alignItems: "center"
            }}
            title="New Chat"
          >
            +
          </button>
        </div>
      )}
    </div>
  );
};

export default ThreadsPanel;
