/* eslint-disable @typescript-eslint/no-explicit-any */
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { login as apiLogin } from "../api";
import "../styles/login.css";

interface LoginProps {
  onLogin: (token: string, userId: number) => void;
}

export default function Login({ onLogin }: LoginProps) {
  const [username, setUsername] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError("");
    try {
      const data = await apiLogin(username);
      if (data.token && data.user_id) {
        onLogin(data.token, data.user_id);
        navigate("/chat");
      } else {
        setError("Invalid response from server");
      }
    } catch (err: any) {
      setError(err?.message || "Failed to login. Please try again.");
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-outer-bg">
      <div className="login-card">
        <h2 className="login-title">Welcome to SimpleChat</h2>
        <form className="login-form" onSubmit={handleLogin} autoComplete="off">
          <label htmlFor="username" className="login-label">Username</label>
          <input
            id="username"
            className="login-input"
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="Enter your username (e.g., abhishek)"
            disabled={loading}
            autoFocus
            autoComplete="username"
          />
          {error && <div className="login-error" role="alert">{error}</div>}
          <button className="login-btn" type="submit" disabled={loading || !username.trim()}>
            {loading ? <span className="login-loader"></span> : "Login"}
          </button>
        </form>
      </div>
      <footer className="login-footer">Â© {new Date().getFullYear()} SimpleChat &mdash; AI Demo App</footer>
    </div>
  );
}
