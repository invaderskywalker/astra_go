import React, { useState } from "react";
import MarkdownPreview from "@uiw/react-markdown-preview";
import DOMPurify from "dompurify";
import AudioModal from "./AudioModal";


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
  sendMessageDirect: (val: string) => void;
  handleKeyDown: (e: React.KeyboardEvent<HTMLTextAreaElement>) => void;
  messagesEndRef: React.RefObject<HTMLDivElement | null>;
  textareaRef: React.RefObject<HTMLTextAreaElement | null>;
  isConnected: boolean;
  handleConnect: () => void;
  isLoadingMessages: boolean;
}

const ChatPanel: React.FC<ChatPanelProps> = ({
  messages,
  input,
  setInput,
  sendMessage,
  sendMessageDirect,
  handleKeyDown,
  messagesEndRef,
  textareaRef,
  isConnected,
  handleConnect,
  isLoadingMessages,
}) => {
  const [showAudioModal, setShowAudioModal] = useState(false);

  const handleCloseAudioModal = () => {
    setShowAudioModal(false);
    setTimeout(() => textareaRef.current?.focus(), 50);
  };

  const handleSendMessage = () => sendMessage();

  return (
    <div className="main-chat-panel">
      <header className="chat-header">
        <div className="chat-header-title">Astra Chat</div>
        <div className="chat-header-options">
          <span
            className="signal-light"
            style={{ backgroundColor: isConnected ? "green" : "red" }}
          />
          <button
            className="connect-btn"
            onClick={handleConnect}
            disabled={isConnected}
          >
            {isConnected ? "Connected" : "Connect"}
          </button>
        </div>
      </header>

      <div className="messages">
        {isLoadingMessages ? (
          <div className="messages-loading">Loading messages...</div>
        ) : (
          messages.map((m) => (
            <div key={m.id} className={`message ${m.user}`}>
              <div className="msg-bubble">
                <div className="msg-info">
                  <div className="msg-info-name">{m.user === "me" ? "You" : "Astra"}</div>
                  <div className="msg-info-time">{m.timestamp}</div>
                </div>
                <div className="msg-text">
                  <MarkdownPreview
                    source={DOMPurify.sanitize(m.text)}
                    className="markdown-preview"
                    style={{
                      padding: 0,
                      background: "transparent",
                      color: "#333",
                      fontSize: "14px",
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
        <button
          onClick={() => setShowAudioModal(true)}
          className="mic-btn"
          aria-label="Open voice input"
        >
          🎤
        </button>
        <textarea
          ref={textareaRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Type or use the mic..."
          disabled={!isConnected}
          rows={1}
        />
        <button
          onClick={handleSendMessage}
          disabled={!isConnected || !input.trim()}
        >
          Send
        </button>
      </div>

      <AudioModal
        open={showAudioModal}
        onClose={handleCloseAudioModal}
        setInput={setInput}
        onVoiceSend={(finalText) => {
          console.log("[ChatPanel] 🗣 Received voice message:", finalText);
          console.log("[ChatPanel] 🚀 Now sending via sendMessage() with:", finalText);
          sendMessageDirect(finalText); // will now read updated input immediately
        }}
        isSpeaking={speechSynthesis.speaking}
      />


    </div>
  );
};

export default ChatPanel;
