import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { login as apiLogin } from "./api";

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
    <div className="login-container">
      <h2>Login to SimpleChat</h2>
      <form onSubmit={handleLogin}>
        <div>
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="Enter username (e.g., abhishek)"
            disabled={loading}
          />
        </div>
        {error && <p style={{ color: "red" }}>{error}</p>}
        <button type="submit" disabled={loading}>
          {loading ? "Logging in..." : "Login"}
        </button>
      </form>
    </div>
  );
}
