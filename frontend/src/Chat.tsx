import { useEffect, useRef, useState } from "react";
import MarkdownPreview from "@uiw/react-markdown-preview";
import { v4 as uuidv4 } from 'uuid';
import './styles/chat.css'

interface Message {
  user: "me" | "agent";
  text: string;
  timestamp: string;
  type?: string;
}

interface IntermediateMessage {
  text: string;
  timestamp: string;
}

interface ChatProps {
  token: string;
  userId: number;
  handleLogout: () => void;
}

export default function Chat({ token, userId }: ChatProps) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [intermediateMessages, setIntermediateMessages] = useState<IntermediateMessage[]>([]);
  const [input, setInput] = useState("");
  const [sessionId, setSessionId] = useState(uuidv4());
  const [isConnected, setIsConnected] = useState(false);
  const ws = useRef<WebSocket | null>(null);
  const reconnectAttempts = useRef(0);
  const reconnectInterval = useRef(1000);
  const messageBuffer = useRef<string[]>([]);
  const bufferTimeout = useRef<ReturnType<typeof setTimeout> | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null); // For auto-scrolling
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const getCurrentTime = () => {
    const now = new Date();
    return now.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  };

  // Auto-scroll to the latest message
  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages, intermediateMessages]);

  const connectWebSocket = () => {
    ws.current = new WebSocket("ws://localhost:8000/agents/ws");

    ws.current.onopen = () => {
      console.log("WebSocket connected");
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
      } else {
        console.error("WebSocket not in OPEN state");
      }
    };

    ws.current.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        const { type, payload } = msg;

        if (type === "session_created") {
          setSessionId(payload.session_id);
          return; // No need to display this as a message
        } else if (type === "response_chunk") {
          const chunk = typeof payload === "object" && payload.chunk ? payload.chunk : JSON.stringify(payload);
          messageBuffer.current.push(chunk);

          // Update UI immediately with the new chunk
          setMessages((prev) => {
            const lastMessage = prev[prev.length - 1];
            const currentTime = getCurrentTime();
            if (lastMessage?.user === "agent" && lastMessage.type === "response_chunk") {
              // Append to the last agent message
              return [
                ...prev.slice(0, -1),
                { ...lastMessage, text: messageBuffer.current.join("") },
              ];
            } else {
              // Create a new agent message
              return [
                ...prev,
                { user: "agent", text: chunk, timestamp: currentTime, type: "response_chunk" },
              ];
            }
          });

          // Clear any existing timeout
          if (bufferTimeout.current) {
            clearTimeout(bufferTimeout.current);
          }

          // Set timeout to finalize the message (optional cleanup)
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
            { user: "agent", text: `Error: ${errorMessage}`, timestamp: getCurrentTime(), type: "error" },
          ]);
        } else if (type === "intermediate" || type === "completed") {
          const messageText = typeof payload === "object" ? JSON.stringify(payload) : payload;
          setIntermediateMessages((prev) => [
            ...prev,
            { text: type === "intermediate" ? `Progress: ${messageText}` : `Completed: ${messageText}`, timestamp: getCurrentTime() },
          ]);
          // Auto-clear after 5 seconds
          // setTimeout(() => {
          //   setIntermediateMessages((prev) => prev.slice(1));
          // }, 5000);
        } else {
          setMessages((prev) => [
            ...prev,
            { user: "agent", text: JSON.stringify(msg), timestamp: getCurrentTime(), type: "unknown" },
          ]);
        }
      } catch (error) {
        console.error("Failed to parse WebSocket message:", error);
        setMessages((prev) => [
          ...prev,
          { user: "agent", text: `Error: Invalid message format - ${event.data}`, timestamp: getCurrentTime(), type: "error" },
        ]);
      }
    };

    ws.current.onclose = (event) => {
      console.log("WebSocket closed", event);
      setIsConnected(false);
      if (bufferTimeout.current) {
        clearTimeout(bufferTimeout.current);
      }
    };

    ws.current.onerror = (error) => {
      console.error("WebSocket error", error);
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
      setMessages((prev) => [
        ...prev,
        { user: "agent", text: "Error: Not connected to server", timestamp: getCurrentTime(), type: "error" },
      ]);
      return;
    }

    setMessages((prev) => [...prev, { user: "me", text: input, timestamp: getCurrentTime() }]);
    ws.current.send(
      JSON.stringify({
        agent_name: "astra",
        query: input,
        session_id: sessionId,
        user_id: userId,
      })
    );
    setInput("");
  };

  const handleConnect = () => {
    if (!isConnected) {
      connectWebSocket();
      setSessionId(uuidv4())
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  // Optional: Auto-resize textarea based on content
  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
      textareaRef.current.style.height = `${textareaRef.current.scrollHeight}px`;
    }
  }, [input]);

  return (
    <div className="chat-container">
      <header className="chat-header">
        {/* <div className="chat-header-title">
          <i className="fas fa-comment-alt"></i> SimpleChat
        </div> */}
        <div className="chat-header-options">
          <span className="signal-light" style={{ backgroundColor: isConnected ? "green" : "red" }}></span>
          <button className="connect-btn" onClick={handleConnect} disabled={isConnected}>
            {isConnected ? "Connected" : "Connect"}
          </button>
          {/* <button className="logout-btn" onClick={handleLogout}>
            Logout
          </button> */}
          <span>
            <i className="fas fa-cog"></i>
          </span>
        </div>
      </header>
      {/* <div className="intermediate-messages">
        {intermediateMessages.map((m, i) => (
          <div key={i} className="intermediate-message">
            <span className="intermediate-text">{m.text}</span>
            <span className="intermediate-time">{m.timestamp}</span>
          </div>
        ))}
      </div> */}
      <div className="messages">
        {messages.map((m, i) => (
          <div key={i} className={`message ${m.user === "me" ? "me" : "agent"}`}>
            <div className="msg-img"></div>
            <div className="msg-bubble">
              <div className="msg-info">
                <div className="msg-info-name">{m.user === "me" ? "You" : "Bot"}</div>
                <div className="msg-info-time">{m.timestamp}</div>
              </div>
              <div className="msg-text">
                {m.user === "agent" && m.type === "response_chunk" ? (
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
                ) : (
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
                )}
              </div>
            </div>
          </div>
        ))}
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
        <button onClick={sendMessage} disabled={!isConnected}>
          Send
        </button>
      </div>
    </div>
  );
}