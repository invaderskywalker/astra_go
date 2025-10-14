// ChatPanel displays chat messages and handles message input for the agent chat UI.
import React from "react";
import MarkdownPreview from "@uiw/react-markdown-preview";

interface Message {
  id: string;
  user: "me" | "agent";
  text: string;
  timestamp: string;
  type?: string;
}

interface ChatPanelProps {
  messages: Message[];
  input: string;
  setInput: (val: string) => void;
  sendMessage: () => void;
  handleKeyDown: (e: React.KeyboardEvent<HTMLTextAreaElement>) => void;
  messagesEndRef: React.RefObject<HTMLDivElement | null>;  // ✅ fix
  textareaRef: React.RefObject<HTMLTextAreaElement | null>; // ✅ same fix for textarea
  isConnected: boolean;
  handleConnect: () => void;
  isLoadingMessages: boolean;
}

const ChatPanel: React.FC<ChatPanelProps> = ({
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
}) => (
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
        messages.map((m, i) => (
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
        onChange={e => setInput(e.target.value)}
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

export default ChatPanel;
