import { useState, useEffect, type JSX } from "react";
import { BrowserRouter as Router, Routes, Route, Link, Navigate, useLocation } from 'react-router-dom';
import Chat from "./Chat";
import Login from "./Login";
import Home from "./Home";
import './App.css';
import './styles/markdown.css';

function App() {
  const [token, setToken] = useState<string | null>(null);
  const [userId, setUserId] = useState<number | null>(null);

  useEffect(() => {
    const storedToken = localStorage.getItem("token");
    const storedUserId = localStorage.getItem("userId");
    if (storedToken && storedUserId) {
      setToken(storedToken);
      setUserId(parseInt(storedUserId, 10));
    }
  }, []);

  const handleLogin = (newToken: string, newUserId: number) => {
    setToken(newToken);
    setUserId(newUserId);
    localStorage.setItem("token", newToken);
    localStorage.setItem("userId", newUserId.toString());
  };

  const handleLogout = () => {
    setToken(null);
    setUserId(null);
    localStorage.removeItem("token");
    localStorage.removeItem("userId");
  };

  // Navigation bar component
  function NavBar() {
    return (
      <nav style={{ 
        background: '#282c34', 
        padding: '1rem', 
        display: 'flex', 
        flexDirection: 'row',
        justifyContent: 'space-between',
        alignItems: 'center'
      }}>
        <div>
          <Link to="/" style={{ color: 'white', marginRight: '1rem', textDecoration: 'none', fontWeight: 'bold' }}>Home</Link>
          <Link to="/chat" style={{ color: 'white', marginRight: '1rem', textDecoration: 'none', fontWeight: 'bold' }}>Chat</Link>
        </div>
        {token && userId && (
          <button style={{ marginLeft: '1rem' }} onClick={handleLogout}>Logout</button>
        )}
      </nav>
    );
  }

  // Protected Route wrapper for Chat
  function ProtectedRoute({ children }: { children: JSX.Element }) {
    const location = useLocation();
    if (!token || !userId) {
      return <Navigate to="/login" state={{ from: location }} replace />;
    }
    return children;
  }

  return (
    <Router>
      <NavBar />
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/login" element={<Login onLogin={handleLogin} />} />
        <Route path="/chat" element={
          <ProtectedRoute>
            <Chat token={token!} userId={userId!} handleLogout={handleLogout} />
          </ProtectedRoute>
        } />
        <Route path="*" element={<Navigate to="/" />} />
      </Routes>
    </Router>
  );
}

export default App;
