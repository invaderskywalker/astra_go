/* eslint-disable @typescript-eslint/no-unused-vars */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { useState, useEffect, useRef, type JSX } from "react";
import { BrowserRouter as Router, Routes, Route, Link, Navigate, useLocation } from 'react-router-dom';
import Chat from "./pages/Chat";
import './App.css';
import './styles/markdown.css';
import Home from "./pages/Home";
import Login from "./pages/Login";
import NotesList from "./pages/NotesList";
import LearningList from "./pages/LearningList";
import { getCurrentUserProfile, updateCurrentUserProfile, type UpdateUserProfilePayload } from "./api";

interface UserProfile {
  id: number;
  username: string;
  email: string;
  full_name?: string | null;
  image_url?: string | null;
}

function ProfileEditDialog({ open, onClose, profile, onSave } : {
  open: boolean;
  onClose: () => void;
  profile: UserProfile | null;
  onSave: (fields: UpdateUserProfilePayload) => Promise<void>;
}) {
  const [form, setForm] = useState<UpdateUserProfilePayload>({});
  const [error, setError] = useState("");
  useEffect(() => {
    if (profile) {
      setForm({
        username: profile.username,
        email: profile.email,
        full_name: profile.full_name || "",
        image_url: profile.image_url || ""
      });
      setError("");
    }
  }, [profile, open]);
  if (!open) return null;
  return (
    <div className="profile-edit-dialog__backdrop" style={{ position: 'fixed', top: 0, left: 0, width: '100vw', height: '100vh', background: 'rgba(0,0,0,.27)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 20 }}>
      <div style={{ background: "#fff", padding: 32, borderRadius: 16, minWidth: 340, boxShadow: '0 2px 12px #2227' }}>
        <h3>Edit Profile</h3>
        <form onSubmit={async e => {
          e.preventDefault();
          try {
            await onSave(form);
            onClose();
          } catch (err: any) {
            setError(err.message || "Failed to update profile");
          }
        }}>
          <div style={{ marginBottom: 12 }}>
            <label>User Name<br/>
              <input value={form.username || ""} onChange={e => setForm(f => ({ ...f, username: e.target.value }))} required style={{ width: '100%' }} />
            </label>
          </div>
          <div style={{ marginBottom: 12 }}>
            <label>Email<br/>
              <input type="email" value={form.email || ""} onChange={e => setForm(f => ({ ...f, email: e.target.value }))} required style={{ width: '100%' }} />
            </label>
          </div>
          <div style={{ marginBottom: 12 }}>
            <label>Full Name<br/>
              <input value={form.full_name || ""} onChange={e => setForm(f => ({ ...f, full_name: e.target.value }))} style={{ width: '100%' }} />
            </label>
          </div>
          <div style={{ marginBottom: 12 }}>
            <label>Image URL<br/>
              <input value={form.image_url || ""} onChange={e => setForm(f => ({ ...f, image_url: e.target.value }))} style={{ width: '100%' }} />
            </label>
          </div>
          {error && <div style={{color:'red', marginBottom:8}}>{error}</div>}
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
            <button type="button" onClick={onClose}>Cancel</button>
            <button type="submit">Save</button>
          </div>
        </form>
      </div>
    </div>
  );
}

function ProfileMenu({ user, onLogout, onEdit } : {
  user: UserProfile,
  onLogout: () => void,
  onEdit: () => void
}) {
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (menuRef.current && e.target instanceof Node && !menuRef.current.contains(e.target)) {
        setOpen(false);
      }
    }
    if (open) {
      window.addEventListener('mousedown', handleClick);
      return () => window.removeEventListener('mousedown', handleClick);
    }
  }, [open]);
  const initials = user.full_name?.[0]?.toUpperCase() || user.username[0]?.toUpperCase() || '?';
  const avatar = user.image_url ? (
    <img src={user.image_url} alt="avatar" style={{ width:36, height:36, borderRadius: '50%', objectFit: 'cover', background:'#eee', border: '1.5px solid #ccc' }} />
  ) : (
    <div style={{ width:36, height:36, borderRadius: '50%', background:'#7752d6', color:'#fff', fontWeight:'bold', textAlign:'center', lineHeight:'36px', fontSize:18 }}>{initials}</div>
  );
  return (
    <div ref={menuRef} style={{ position: 'relative', marginLeft: 12 }}>
      <button
        aria-label="User menu"
        onClick={() => setOpen(o => !o)}
        style={{ background: 'none', border: 'none', padding: 0, margin: 0, cursor: 'pointer', display:'flex', alignItems:'center' }}
      >
        {avatar}
      </button>
      {open && (
        <div style={{ position:'absolute', right:0, top:'calc(100% + 7px)', minWidth:170, zIndex: 12, background: '#fff', border: '1px solid #bbb', borderRadius: 8, boxShadow: '0 3px 18px #9999', padding: '9px 0', display:'flex', flexDirection:'column' }}>
          <div style={{padding:'8px 18px', borderBottom: '1px solid #eee', color:'#545169', fontWeight: 600}}>{user.full_name || user.username}</div>
          <button style={menuBtnStyle} onClick={() => { setOpen(false); onEdit(); }}>Edit Profile</button>
          <button style={menuBtnStyle} onClick={() => { setOpen(false); onLogout(); }}>Logout</button>
        </div>
      )}
    </div>
  );
}
const menuBtnStyle: React.CSSProperties = {
  background: 'none',
  border: 'none',
  outline: 'none',
  textAlign: 'left',
  width: '100%',
  fontSize: 15,
  padding: '8px 18px',
  cursor: 'pointer',
  color: '#5133ca'
};


// Updated robust navbar
function NavBar({token, user, onLogout, onEditProfile}: {
  token: string | null;
  user: UserProfile | null;
  onLogout: () => void;
  onEditProfile: () => void;
}) {
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
        {token && user && (
          <Link to="/notes" style={{ color: 'white', marginRight: '1rem', textDecoration: 'none', fontWeight: 'bold' }}>Notes</Link>
        )}
        {token && user && (
          <Link to="/learnings" style={{ color: 'white', marginRight: '1rem', textDecoration: 'none', fontWeight: 'bold' }}>Learnings</Link>
        )}
      </div>
      {token && user && (
        <ProfileMenu user={user} onLogout={onLogout} onEdit={onEditProfile} />
      )}
    </nav>
  );
}

function App() {
  const [token, setToken] = useState<string | null | undefined>(undefined);
  const [userId, setUserId] = useState<number | null | undefined>(undefined);
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [profileLoading, setProfileLoading] = useState(false);
  const [profileEditOpen, setProfileEditOpen] = useState(false);
  const [profileError, setProfileError] = useState<string>("");

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

  // Fetch profile when token is ready
  useEffect(() => {
    if (!token) {
      setProfile(null);
      return;
    }
    setProfileLoading(true);
    getCurrentUserProfile(token)
      .then(setProfile)
      .catch(() => setProfile(null))
      .finally(() => setProfileLoading(false));
  }, [token]);

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
    setProfile(null);
    localStorage.removeItem("token");
    localStorage.removeItem("userId");
  };

  const openProfileEdit = () => setProfileEditOpen(true);
  const closeProfileEdit = () => setProfileEditOpen(false);

  async function handleProfileEdit(fields: UpdateUserProfilePayload) {
    if (!token) throw new Error("Not logged in");
    setProfileLoading(true);
    setProfileError("");
    try {
      const updated = await updateCurrentUserProfile(token, fields);
      setProfile(updated);
      setProfileEditOpen(false);
    } catch (e: any) {
      setProfileError(e.message || "Failed to update profile");
    } finally {
      setProfileLoading(false);
    }
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
      <NavBar 
        token={token} 
        user={profile} 
        onLogout={handleLogout} 
        onEditProfile={openProfileEdit}
      />
      <ProfileEditDialog 
        open={profileEditOpen}
        onClose={closeProfileEdit}
        profile={profile}
        onSave={handleProfileEdit}
      />
      {profileError && (
        <div style={{ position: 'absolute', top: 60, left: 0, right: 0, textAlign: 'center', zIndex: 20 }}>
          <span style={{ background: '#fff2f0', color: '#c00', padding: '7px 24px', borderRadius: 9 }}>{profileError}</span>
        </div>
      )}
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
