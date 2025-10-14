/* eslint-disable @typescript-eslint/no-explicit-any */
import { useState, useEffect } from "react";
import type { UpdateUserProfilePayload } from "../api";

export interface UserProfile {
  id: number;
  username: string;
  email: string;
  full_name?: string | null;
  image_url?: string | null;
}

export default function ProfileEditDialog({ open, onClose, profile, onSave } : {
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
