import { useState, useEffect, type JSX } from "react";
import { BrowserRouter as Router, Routes, Route, Link, Navigate, useLocation } from 'react-router-dom';
import Chat from "./Chat";
import Login from "./Login";
import Home from "./Home";
import LearningList from "./LearningList";
import NotesList from "./NotesList";
import './App.css';
import './styles/markdown.css';

// Robust state initialization and rehydration on refresh
function App() {
  // Use undefined as initial state to distinguish between not yet loaded and null (logged out)
  const [token, setToken] = useState<string | null | undefined>(undefined);
  const [userId, setUserId] = useState<number | null | undefined>(undefined);

  // On mount, rehydrate auth state from localStorage
  useEffect(() => {
    const storedToken = localStorage.getItem("token");
    const storedUserId = localStorage.getItem("userId");
    if (storedToken && storedUserId) {
      setToken(storedToken);
      setUserId(parseInt(storedUserId, 10));
    } else {
      setToken(null);
      setUserId(null);
    }
  }, []);

  // Login and logout handlers update both state and localStorage
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

  function NavBar() {
    return (
      <nav style={{ 
        background: '#282c34', 
        padding: '1rem', 
        display: 'flex', 
        flexDirection: 'row',
        justifyContent: 'space-between',
        alignItems: 'center',
        height: '6vh'
      }}>
        <div>
          <Link to="/" style={{ color: 'white', marginRight: '1rem', textDecoration: 'none', fontWeight: 'bold' }}>Home</Link>
          <Link to="/chat" style={{ color: 'white', marginRight: '1rem', textDecoration: 'none', fontWeight: 'bold' }}>Chat</Link>
          {token && userId && (
            <Link to="/notes" style={{ color: 'white', marginRight: '1rem', textDecoration: 'none', fontWeight: 'bold' }}>Notes</Link>
          )}
          {token && userId && (
            <Link to="/learnings" style={{ color: 'white', marginRight: '1rem', textDecoration: 'none', fontWeight: 'bold' }}>Learnings</Link>
          )}
        </div>
        {token && userId && (
          <button className="connect-btn" style={{ marginLeft: '1rem' }} onClick={handleLogout}>Logout</button>
        )}
      </nav>
    );
  }

  // Ensure we wait for state hydration from localStorage before rendering routes
  if (token === undefined || userId === undefined) {
    return <div style={{ textAlign: 'center', marginTop: '15vh' }}><span>Loading...</span></div>;
  }

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
        <Route path="/notes" element={
          <ProtectedRoute>
            <NotesList token={token!} userId={userId!} />
          </ProtectedRoute>
        } />
        <Route path="/learnings" element={
          <ProtectedRoute>
            <LearningList token={token!} userId={userId!} />
          </ProtectedRoute>
        } />
        <Route path="*" element={<Navigate to="/" />} />
      </Routes>
    </Router>
  );
}

export default App;
