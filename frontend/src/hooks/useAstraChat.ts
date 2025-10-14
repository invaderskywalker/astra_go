/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/no-unused-vars */
// Custom React Hook for managing chat threads, messages, session, and WebSocket for Astra Chat UI
import { useEffect, useRef, useState } from "react";
import { v4 as uuidv4 } from 'uuid';
import { fetchChatSessions, fetchMessagesForSession, deleteChatSession } from "../api";
import { isJsonString, parseMaybeJson, cleanContent, getCurrentTime, scrollToBottom } from "../utils/chatUtils";

export interface Message {
  id: string;
  user: "me" | "agent";
  text: string;
  timestamp: string;
  type?: string;
}

export interface IntermediateMessage {
  text: string;
  timestamp: string;
}

export interface ChatSessionSummary {
  session_id: string;
  last_message: string;
  last_message_role: string;
  last_activity: string;
}

interface UseAstraChatParams {
  token: string;
  userId: number;
}

export function useAstraChat({ token, userId }: UseAstraChatParams) {
  // State
  const [threads, setThreads] = useState<ChatSessionSummary[]>([]);
  const [isLoadingThreads, setIsLoadingThreads] = useState(false);
  const [messages, setMessages] = useState<Message[]>([]);
  const [isLoadingMessages, setIsLoadingMessages] = useState(false);
  const [intermediateMessages, setIntermediateMessages] = useState<IntermediateMessage[]>([]);
  const [input, setInput] = useState("");
  const [sessionId, setSessionId] = useState<string>("");
  const [isConnected, setIsConnected] = useState(false);

  // Refs
  const ws = useRef<WebSocket | null>(null);
  const reconnectAttempts = useRef(0);
  const reconnectInterval = useRef(1000);
  const messageBuffer = useRef<string[]>([]);
  const bufferTimeout = useRef<ReturnType<typeof setTimeout> | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Thread Operations
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
      setMessages(
        data.filter((m: any) => m.role !== "full_plan").map((m: any) => ({
          id: m.id,
          user: m.role === "user_query" ? "me" : "agent",
          text: cleanContent(m.content),
          timestamp: new Date(m.timestamp).toLocaleTimeString([], {
            hour: "2-digit",
            minute: "2-digit",
          })
        }))
      );
    } catch {
      setMessages([]);
    } finally {
      setIsLoadingMessages(false);
    }
  };

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

  // WebSocket Operations
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

  // Message Operations
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

  // UI/Effects
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

  useEffect(() => {
    return () => {
      ws.current?.close();
      setIsConnected(false);
      if (bufferTimeout.current) clearTimeout(bufferTimeout.current);
    };
  }, [token, userId]);

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
      textareaRef.current.style.height = `${textareaRef.current.scrollHeight}px`;
    }
  }, [input]);

  useEffect(() => {
    scrollToBottom(messagesEndRef);
  }, [messages, intermediateMessages]);

  // Exposed API
  return {
    threads,
    isLoadingThreads,
    messages,
    isLoadingMessages,
    intermediateMessages,
    input, setInput,
    sendMessage,
    sessionId, setSessionId,
    handleDeleteSession,
    handleSelectSession,
    isConnected,
    connectWebSocket,
    messagesEndRef,
    textareaRef
  };
}
