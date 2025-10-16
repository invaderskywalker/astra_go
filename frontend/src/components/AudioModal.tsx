/* eslint-disable @typescript-eslint/no-explicit-any */
import React, { useState, useRef, useEffect, useCallback } from "react";
import MicIcon from "@mui/icons-material/Mic";
import CloseIcon from "@mui/icons-material/Close";
import "../styles/AudioModal.css";

interface AudioModalProps {
  open: boolean;
  onClose: () => void;
  setInput: (val: string) => void;
  onVoiceSend: (finalText: string) => void;
  isSpeaking?: boolean;
}

const AudioModal: React.FC<AudioModalProps> = ({
  open,
  onClose,
  setInput,
  onVoiceSend,
  isSpeaking = false,
}) => {
  const [isRecording, _setIsRecording] = useState(false);
  const [sttSupported, setSttSupported] = useState(true);
  const [errorMsg, setErrorMsg] = useState("");
  const [interimTranscript, setInterimTranscript] = useState("");
  const [countdown, _setCountdown] = useState<number | null>(null);

  // ---- REFS ----
  const recognitionRef = useRef<any>(null);
  const silenceTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const countdownIntervalRef = useRef<NodeJS.Timeout | null>(null);
  const silenceCheckIntervalRef = useRef<NodeJS.Timeout | null>(null);
  const finalTextRef = useRef("");
  const lastSpeechTimeRef = useRef<number>(0);
  const isRecordingRef = useRef(false);
  const countdownRef = useRef<number | null>(null);

  // Wrapped state setters
  const setIsRecording = (val: boolean) => {
    console.log(`[${new Date().toISOString()}] 🎙️ setIsRecording:`, val);
    isRecordingRef.current = val;
    _setIsRecording(val);
  };
  const setCountdown = (val: number | null) => {
    countdownRef.current = val;
    _setCountdown(val);
  };

  const log = (...args: any[]) =>
    console.log(`[${new Date().toISOString()}]`, ...args);

  // ---- SPEECH RECOGNITION ----
  const getSpeechRecognition = useCallback(() => {
    return (
      (window as any).SpeechRecognition ||
      (window as any).webkitSpeechRecognition ||
      null
    );
  }, []);

  useEffect(() => {
    const SR = getSpeechRecognition();
    if (!SR) setSttSupported(false);
  }, [getSpeechRecognition]);

  const stopRecording = useCallback(() => {
    log("🛑 stopRecording() called");
    if (recognitionRef.current) {
      recognitionRef.current.stop();
      recognitionRef.current = null;
    }
    if (silenceCheckIntervalRef.current)
      clearInterval(silenceCheckIntervalRef.current);
    if (countdownIntervalRef.current)
      clearInterval(countdownIntervalRef.current);
    setIsRecording(false);
    setInterimTranscript("");
    setCountdown(null);
  }, []);

  const startCountdownAndSend = useCallback(() => {
    let timeLeft = 3;
    setCountdown(timeLeft);
    log("🕒 Starting countdown to send...");

    countdownIntervalRef.current = setInterval(() => {
      timeLeft -= 1;
      setCountdown(timeLeft);
      log(`⏳ Countdown: ${timeLeft}`);

      if (timeLeft <= 0) {
        clearInterval(countdownIntervalRef.current!);
        countdownIntervalRef.current = null;

        const textToSend = finalTextRef.current.trim();
        log("🚀 Countdown finished — preparing to send:", textToSend);

        if (textToSend) {
          log("📤 Triggering onVoiceSend() with:", textToSend);
          onVoiceSend(textToSend);
        } else {
          log("⚠️ No text to send, skipping sendMessage()");
        }

        finalTextRef.current = "";
        onClose();
      }
    }, 1000);
  }, [onVoiceSend, onClose]);

  const startRecording = useCallback(() => {
    log("🎤 startRecording() called");
    setErrorMsg("");
    setInterimTranscript("");
    finalTextRef.current = "";
    lastSpeechTimeRef.current = Date.now();

    const SR = getSpeechRecognition();
    if (!SR) {
      setErrorMsg("Speech Recognition API not supported in this browser.");
      setSttSupported(false);
      return;
    }

    recognitionRef.current = new SR();
    recognitionRef.current.lang = "en-US";
    recognitionRef.current.interimResults = true;
    recognitionRef.current.maxAlternatives = 1;

    recognitionRef.current.onresult = (event: any) => {
      let finalText = "";
      let heardSomething = false;

      for (let i = event.resultIndex; i < event.results.length; i++) {
        const transcript = event.results[i][0]?.transcript || "";
        if (event.results[i].isFinal) {
          finalText += transcript;
        } else if (transcript.trim()) {
          setInterimTranscript(transcript);
          heardSomething = true;
        }
      }

      if (heardSomething || finalText.trim()) {
        lastSpeechTimeRef.current = Date.now();
        log("🎧 Heard voice input");
        if (countdownIntervalRef.current) {
          log("🔁 Cancelling active countdown due to new speech");
          clearInterval(countdownIntervalRef.current);
          countdownIntervalRef.current = null;
          setCountdown(null);
        }
      }

      if (finalText.trim()) {
        finalTextRef.current += " " + finalText.trim();
        log("📝 Final text accumulated:", finalTextRef.current);
        setInput((prev) => (prev ? prev + " " : "") + finalText.trim());
        setInterimTranscript("");
      }
    };

    recognitionRef.current.onerror = (event: any) => {
      setErrorMsg(event.error ? `Audio Error: ${event.error}` : "Unknown error");
      log("❌ Speech recognition error:", event.error);
      stopRecording();
    };

    recognitionRef.current.start();
    setIsRecording(true);

    silenceCheckIntervalRef.current = setInterval(() => {
      const now = Date.now();
      const elapsed = now - lastSpeechTimeRef.current;
      const silenceThreshold = 5000;

      if (isRecordingRef.current && elapsed > silenceThreshold && !countdownRef.current) {
        log(`🤫 Detected silence for ${elapsed}ms. Starting countdown...`);
        stopRecording();
        if (finalTextRef.current.trim()) {
          startCountdownAndSend();
        } else {
          log("⚠️ No recognized text at silence detection");
        }
      }
    }, 1000);
  }, [getSpeechRecognition, stopRecording, setInput, onVoiceSend, onClose, startCountdownAndSend]);

  useEffect(() => {
    if (!open && isRecordingRef.current) stopRecording();
    return () => {
      if (silenceTimeoutRef.current) clearTimeout(silenceTimeoutRef.current);
      if (silenceCheckIntervalRef.current)
        clearInterval(silenceCheckIntervalRef.current);
      if (countdownIntervalRef.current)
        clearInterval(countdownIntervalRef.current);
    };
  }, [open, stopRecording]);

  if (!open) return null;

  return (
    <div className="audio-container" onClick={onClose}>
      <button
        className="close-btn"
        onClick={() => {
          log("❎ Close button clicked");
          stopRecording();
          onClose();
        }}
      >
        <CloseIcon />
      </button>

      <div className="audio-center" onClick={(e) => e.stopPropagation()}>
        <div className={`ripple-wrapper ${isRecording ? "recording" : ""} ${isSpeaking ? "speaking" : ""}`}>
          <div className="ripple"></div>
          <div className="ripple"></div>
          <div className="ripple"></div>

          <MicIcon
            className={`mic-icon ${isRecording ? "recording" : ""}`}
            onClick={() => {
              log(isRecording ? "🎙️ Stopping recording" : "🎙️ Starting recording");
              isRecording ? stopRecording() : startRecording();
            }}
          />
        </div>

        <p className="status-text">
          {errorMsg
            ? errorMsg
            : countdown !== null
            ? `Sending in ${countdown}...`
            : isRecording
            ? "Listening..."
            : isSpeaking
            ? "Speaking..."
            : sttSupported
            ? "Tap to start"
            : "Not supported"}
        </p>

        {isRecording && interimTranscript && <p className="interim-transcript">{interimTranscript}</p>}
      </div>
    </div>
  );
};

export default AudioModal;
