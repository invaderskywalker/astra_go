/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/no-unused-vars */
import { useEffect, useRef, useState } from "react";
import MarkdownPreview from "@uiw/react-markdown-preview";
import { v4 as uuidv4 } from 'uuid';
import './styles/chat.css';
import RenderJsonTree from "./RenderJsonTree";
import { FaTrash } from "react-icons/fa";
import {
  fetchChatSessions,
  fetchMessagesForSession,
  deleteChatSession
} from "./api";
import RenderYamlView from "./components/RenderYamlView";

function isJsonString(str: string): boolean {
  if (typeof str !== "string") return false;
  try {
    const parsed = JSON.parse(str);
    return typeof parsed === "object" && parsed !== null;
  } catch {
    return false;
  }
}

function parseMaybeJson(input: any): any {
  if (typeof input !== "string") return input;
  try {
    const parsed = JSON.parse(input);
    if (typeof parsed === "object" && parsed !== null) {
      for (const key in parsed) {
        if (typeof parsed[key] === "string" && isJsonString(parsed[key])) {
          parsed[key] = parseMaybeJson(parsed[key]);
        }
      }
    }
    return parsed;
  } catch {
    return input;
  }
}

interface Message {
  id: string;
  user: "me" | "agent";
  text: string;
  timestamp: string;
  type?: string;
}

interface IntermediateMessage {
  text: string;
  timestamp: string;
}

interface ChatSessionSummary {
  session_id: string;
  last_message: string;
  last_message_role: string;
  last_activity: string;
}

interface ChatProps {
  token: string;
  userId: number;
  handleLogout: () => void;
}

function ThreadsPanel({
  threads,
  activeSessionId,
  onSelectSession,
  onDeleteSession,
  isLoading
}: {
  threads: ChatSessionSummary[];
  activeSessionId: string;
  onSelectSession: (session_id: string) => void;
  onDeleteSession: (session_id: string) => void;
  isLoading: boolean;
}) {
  return (
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
                onClick={(e) => {
                  if ((e.target as HTMLElement).classList.contains('thread-delete-btn')) return;
                  onSelectSession(thread.session_id);
                }}
                style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}
              >
                <span className="thread-title">
                  {thread.last_message ? thread.session_id.slice(0, 20) : "(no message yet)"}
                </span>
                {/* <span className="thread-meta">
                  {new Date(thread.last_activity).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                </span> */}
                <button
                  className="thread-delete-btn"
                  title="Delete thread"
                  onClick={(e) => {
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
}

function ThoughtProcessPanel({ thoughts }: { thoughts: IntermediateMessage[] }) {
  return (
    <div className="thought-panel">
      <div className="thought-header">Astra's Thought Process</div>
      <div className="thought-messages">
        {thoughts.length === 0 ? (
          <div className="thought-empty">
            Astra's reasoning/steps will appear here as you chat
          </div>
        ) : (
          thoughts.map((m, i) => {
            const jsonMatch = m.text.match(/{[\s\S]*}$/);
            const maybeJson = jsonMatch ? jsonMatch[0] : m.text;
            const isJson = isJsonString(maybeJson);
            const parsedData = isJson ? parseMaybeJson(maybeJson) : maybeJson;
            return (
              <div key={i} className="thought-message">
                <span className="thought-text">
                  {isJson && parsedData ? (
                    <RenderYamlView data={parsedData} />
                  ) : (
                    m.text
                  )}
                </span>
                <span className="thought-time">{m.timestamp}</span>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}

function ChatPanel({
  messages,
  input,
  setInput,
  sendMessage,
  handleKeyDown,
  messagesEndRef,
  textareaRef,
  isConnected,
  handleConnect,
  isLoadingMessages
}: any) {
  return (
    <div className="main-chat-panel">
      <header className="chat-header">
        <div className="chat-header-title">
          Astra Chat
        </div>
        <div className="chat-header-options">
          <span className="signal-light" style={{ backgroundColor: isConnected ? "green" : "red" }}></span>
          <button className="connect-btn" onClick={handleConnect} disabled={isConnected}>
            {isConnected ? "Connected" : "Connect"}
          </button>
        </div>
      </header>
      <div className="messages">
        {isLoadingMessages ? (
          <div className="messages-loading">Loading messages...</div>
        ) : (
          messages.map((m: Message, i: number) => (
            <div key={m.id || i} className={`message ${m.user === "me" ? "me" : "agent"}`}>
              <div className="msg-img"></div>
              <div className="msg-bubble">
                <div className="msg-info">
                  <div className="msg-info-name">{m.user === "me" ? "You" : "Astra"}</div>
                  <div className="msg-info-time">{m.timestamp}</div>
                </div>
                <div className="msg-text">
                  <MarkdownPreview
                    source={m.text}
                    className="markdown-preview"
                    style={{
                      padding: 0,
                      background: "transparent",
                      color: "#333333",
                      fontSize: "14px",
                      fontWeight: "400",
                      fontFamily: "Poppins",
                    }}
                  />
                </div>
              </div>
            </div>
          ))
        )}
        <div ref={messagesEndRef} />
      </div>
      <div className="input-container">
        <textarea
          ref={textareaRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Enter your message..."
          disabled={!isConnected}
          rows={1}
          style={{
            resize: "none",
            overflow: "hidden",
            minWidth: "84%",
            minHeight: "100px",
            maxHeight: "150px",
            borderRadius: '16px',
            background: 'white',
            padding: '16px'
          }}
        />
        <button onClick={sendMessage} disabled={!isConnected || !input.trim()}>
          Send
        </button>
      </div>
    </div>
  );
}

export default function Chat({ token, userId, handleLogout }: ChatProps) {
  const [threads, setThreads] = useState<ChatSessionSummary[]>([]);
  const [isLoadingThreads, setIsLoadingThreads] = useState(false);

  const handleDeleteSession = async (sid: string) => {
    if (!sid) return;
    const confirmed = window.confirm('Are you sure you want to delete this chat thread? This action cannot be undone.');
    if (!confirmed) return;
    const ok = await deleteChatSession(sid, token);
    if (ok) {
      setThreads(prev => prev.filter(th => th.session_id !== sid));
      if (sessionId === sid) {
        setSessionId("");
        setMessages([]);
      }
    } else {
      window.alert('Failed to delete chat thread.');
    }
  };
  const [messages, setMessages] = useState<Message[]>([]);
  const [isLoadingMessages, setIsLoadingMessages] = useState(false);
  const [intermediateMessages, setIntermediateMessages] = useState<IntermediateMessage[]>([]);
  const [input, setInput] = useState("");
  const [sessionId, setSessionId] = useState<string>("");
  const [isConnected, setIsConnected] = useState(false);
  const ws = useRef<WebSocket | null>(null);
  const reconnectAttempts = useRef(0);
  const reconnectInterval = useRef(1000);
  const messageBuffer = useRef<string[]>([]);
  const bufferTimeout = useRef<ReturnType<typeof setTimeout> | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const getCurrentTime = () => {
    const now = new Date();
    return now.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  };

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(() => { scrollToBottom(); }, [messages, intermediateMessages]);

  const fetchThreads = async () => {
    setIsLoadingThreads(true);
    try {
      const data = await fetchChatSessions(token);
      setThreads(Array.isArray(data) ? data : []);
    } catch {
      setThreads([]);
    } finally {
      setIsLoadingThreads(false);
    }
  };

  const fetchMessagesForCurrentSession = async (sid: string) => {
    setIsLoadingMessages(true);
    try {
      const data = await fetchMessagesForSession(sid, token);
      function cleanContent(raw: string | undefined): string {
        if (!raw) return "";
        let content = raw.trim();
        try {
          if (
            (content.startsWith('"') && content.endsWith('"')) ||
            (content.startsWith("'") && content.endsWith("'"))
          ) {
            if (content.startsWith("'") && content.endsWith("'")) {
              content = '"' + content.slice(1, -1).replace(/"/g, '\\"') + '"';
            }
            content = JSON.parse(content);
          }
        } catch (e) {
          //
        }
        content = content
          .replace(/\\n/g, "\n")
          .replace(/\\r/g, "\r")
          .replace(/\\t/g, "\t")
          .replace(/\\"/g, '"')
          .replace(/\\'/g, "'")
          .replace(/\\\\/g, "\\");
        return content.trim();
      }
      setMessages(
        data
          .filter((m: any) => m.role !== "full_plan")
          .map((m: any) => {
            const cleaned = cleanContent(m.content);
            return {
              id: m.id,
              user: m.role === "user_query" ? "me" : "agent",
              text: cleaned,
              timestamp: new Date(m.timestamp).toLocaleTimeString([], {
                hour: "2-digit",
                minute: "2-digit",
              }),
            };
          })
      );
    } catch {
      setMessages([]);
    } finally {
      setIsLoadingMessages(false);
    }
  };

  useEffect(() => {
    fetchThreads();
  }, [token]);

  useEffect(() => {
    if (!sessionId && threads.length > 0) {
      setSessionId(threads[0].session_id);
    }
    if (sessionId && !threads.some(th => th.session_id === sessionId)) {
      setSessionId("");
      setMessages([]);
    }
  }, [threads]);

  useEffect(() => {
    if (sessionId) {
      fetchMessagesForCurrentSession(sessionId);
    } else {
      setMessages([]);
    }
  }, [sessionId]);

  const handleSelectSession = (sid: string) => {
    if (!sid) {
      const newSid = uuidv4();
      setSessionId(newSid);
      setMessages([]);
    } else {
      setSessionId(sid);
    }
    setIntermediateMessages([]);
  };

  const connectWebSocket = () => {
    ws.current = new WebSocket("ws://localhost:8000/agents/ws");
    ws.current.onopen = () => {
      setIsConnected(true);
      reconnectAttempts.current = 0;
      reconnectInterval.current = 1000;
      if (ws.current?.readyState === WebSocket.OPEN) {
        ws.current.send(
          JSON.stringify({
            token,
            agent_name: "astra",
            query: "init",
            session_id: sessionId,
            user_id: userId,
          })
        );
      }
    };
    ws.current.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        const { type, payload } = msg;
        if (type === "session_created") {
          return;
        } else if (type === "response_chunk") {
          const chunk = typeof payload === "object" && payload.chunk ? payload.chunk : JSON.stringify(payload);
          messageBuffer.current.push(chunk);
          setMessages((prev) => {
            const lastMessage = prev[prev.length - 1];
            const currentTime = getCurrentTime();
            if (lastMessage?.user === "agent" && lastMessage.type === "response_chunk") {
              return [
                ...prev.slice(0, -1),
                { ...lastMessage, text: messageBuffer.current.join("") },
              ];
            } else {
              return [
                ...prev,
                { id: uuidv4(), user: "agent", text: chunk, timestamp: currentTime, type: "response_chunk" },
              ];
            }
          });
          if (bufferTimeout.current) clearTimeout(bufferTimeout.current);
          bufferTimeout.current = setTimeout(() => {
            const fullMessage = messageBuffer.current.join("");
            if (fullMessage.trim()) {
              setMessages((prev) => {
                const lastMessage = prev[prev.length - 1];
                if (lastMessage?.user === "agent" && lastMessage.type === "response_chunk") {
                  return [
                    ...prev.slice(0, -1),
                    { ...lastMessage, text: fullMessage },
                  ];
                }
                return prev;
              });
            }
            messageBuffer.current = [];
          }, 500);
        } else if (type === "error") {
          const errorMessage = typeof payload === "object" && payload.message ? payload.message : JSON.stringify(payload);
          setMessages((prev) => [
            ...prev,
            { id: uuidv4(), user: "agent", text: `Error: ${errorMessage}`, timestamp: getCurrentTime(), type: "error" },
          ]);
        } else if (type === "intermediate" || type === "completed") {
          const messageText = typeof payload === "object" ? JSON.stringify(payload) : payload;
          setIntermediateMessages((prev) => [
            ...prev,
            { text: type === "intermediate" ? `Progress: ${messageText}` : `Completed: ${messageText}`, timestamp: getCurrentTime() },
          ]);
        } else {
          setMessages((prev) => [
            ...prev,
            { id: uuidv4(), user: "agent", text: JSON.stringify(msg), timestamp: getCurrentTime(), type: "unknown" },
          ]);
        }
      } catch (error) {
        setMessages((prev) => [
          ...prev,
          { id: uuidv4(), user: "agent", text: `Error: Invalid message format - ${event.data}`, timestamp: getCurrentTime(), type: "error" },
        ]);
      }
    };
    ws.current.onclose = () => {
      setIsConnected(false);
      if (bufferTimeout.current) clearTimeout(bufferTimeout.current);
    };
    ws.current.onerror = () => {
      setIsConnected(false);
      ws.current?.close();
    };
  };

  useEffect(() => {
    return () => {
      ws.current?.close();
      setIsConnected(false);
      if (bufferTimeout.current) {
        clearTimeout(bufferTimeout.current);
      }
    };
  }, [token, userId]);

  const sendMessage = () => {
    if (!input.trim()) return;
    if (!ws.current || ws.current.readyState !== WebSocket.OPEN) {
      setMessages((prev) => [...prev, { id: uuidv4(), user: "agent", text: "Error: Not connected to server", timestamp: getCurrentTime(), type: "error" }]);
      return;
    }
    setMessages((prev) => [...prev, { id: uuidv4(), user: "me", text: input, timestamp: getCurrentTime() }]);
    ws.current.send(JSON.stringify({
      agent_name: "astra",
      query: input,
      session_id: sessionId,
      user_id: userId,
    }));
    setInput("");
    setTimeout(fetchThreads, 2000);
  };

  const handleConnect = () => {
    if (!isConnected) {
      connectWebSocket();
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
      textareaRef.current.style.height = `${textareaRef.current.scrollHeight}px`;
    }
  }, [input]);

  return (
    <div className="chat-3panel-root">
      <ThreadsPanel
        threads={threads}
        activeSessionId={sessionId}
        onSelectSession={handleSelectSession}
        onDeleteSession={handleDeleteSession}
        isLoading={isLoadingThreads}
      />
      <ChatPanel
        messages={messages}
        input={input}
        setInput={setInput}
        sendMessage={sendMessage}
        handleKeyDown={handleKeyDown}
        messagesEndRef={messagesEndRef}
        textareaRef={textareaRef}
        isConnected={isConnected}
        handleConnect={handleConnect}
        isLoadingMessages={isLoadingMessages}
      />
      <ThoughtProcessPanel thoughts={intermediateMessages} />
    </div>
  );
}
