/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/no-unused-vars */
// Chat page: orchestrates Astra Chat panels with state from custom hook
import React from "react";
import ThreadsPanel from "../components/ThreadsPanel";
import ChatPanel from "../components/ChatPanel";
import ResizableThoughtPanel from "../components/ResizableThoughtPanel";
import ThoughtProcessPanel from "../components/ThoughtProcessPanel";
import { useAstraChat } from "../hooks/useAstraChat";
import "../styles/chat.css";

interface ChatProps {
  token: string;
  userId: number;
  handleLogout: () => void;
}

const Chat: React.FC<ChatProps> = ({ token, userId, handleLogout }) => {
  const {
    threads,
    isLoadingThreads,
    messages,
    isLoadingMessages,
    intermediateMessages,
    input, setInput,
    sendMessage,
    sessionId,
    handleDeleteSession,
    handleSelectSession,
    isConnected,
    connectWebSocket,
    messagesEndRef,
    textareaRef
  } = useAstraChat({ token, userId });

  // Keyboard shortcut for enter-to-send
  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

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
        handleConnect={connectWebSocket}
        isLoadingMessages={isLoadingMessages}
      />
      {/* <ResizableThoughtPanel> */}
        <ThoughtProcessPanel thoughts={intermediateMessages} />
      {/* </ResizableThoughtPanel> */}
    </div>
  );
};

export default Chat;
