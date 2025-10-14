/* eslint-disable @typescript-eslint/no-explicit-any */
import { useState, useEffect, useRef } from "react";
import type { UpdateUserProfilePayload } from "../api";
import '../styles/ProfileEditDialog.css';

export interface UserProfile {
  id: number;
  username: string;
  email: string;
  full_name?: string | null;
  image_url?: string | null;
}

function isValidImageUrl(url: string) {
  return /^https?:\/\/.+\.(jpg|jpeg|png|gif|webp)$/i.test(url);
}

export default function ProfileEditDialog({ open, onClose, profile, onSave } : {
  open: boolean;
  onClose: () => void;
  profile: UserProfile | null;
  onSave: (fields: UpdateUserProfilePayload) => Promise<void>;
}) {
  const [form, setForm] = useState<UpdateUserProfilePayload>({});
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [imgError, setImgError] = useState(false);
  const dialogRef = useRef<HTMLDivElement>(null);
  const inputFirstRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (profile && open) {
      setForm({
        username: profile.username,
        email: profile.email,
        full_name: profile.full_name || "",
        image_url: profile.image_url || ""
      });
      setError("");
      setSuccess("");
      setImgError(false);
    }
  }, [profile, open]);

  useEffect(() => {
    if (open && inputFirstRef.current) {
      // focus first field
      inputFirstRef.current.focus();
    }
    function handleKey(e: KeyboardEvent) {
      if (open && e.key === "Escape") {
        onClose();
      }
    }
    if (open) {
      window.addEventListener("keydown", handleKey);
      return () => window.removeEventListener("keydown", handleKey);
    }
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div className="profile-edit-backdrop" aria-modal="true" role="dialog">
      <div className="profile-edit-dialog" ref={dialogRef} tabIndex={-1}>
        <div className="profile-edit-header">
          <h2>Edit Profile</h2>
        </div>
        <div className="profile-edit-avatar-preview">
          <div className="profile-edit-avatar-ring">
            {form.image_url && isValidImageUrl(form.image_url) && !imgError ? (
              <img
                src={form.image_url}
                alt="Avatar Preview"
                className="profile-edit-avatar-img"
                onError={() => setImgError(true)}
                onLoad={() => setImgError(false)}
              />
            ) : (
              <div className="profile-edit-avatar-fallback">
                <svg width="64" height="64" viewBox="0 0 64 64" fill="none">
                  <circle cx="32" cy="32" r="32" fill="#E0E0E0" />
                  <path d="M32 42c-6.627 0-12-2.686-12-6v4a12 12 0 0024 0v-4c0 3.314-5.373 6-12 6zm0-22a6 6 0 100 12 6 6 0 000-12z" fill="#BDBDBD" />
                </svg>
              </div>
            )}
          </div>
          <div className="profile-edit-avatar-desc">Live preview</div>
        </div>
        <form className="profile-edit-form" onSubmit={async e => {
          e.preventDefault();
          setError("");
          setSuccess("");
          try {
            await onSave(form);
            setSuccess("Profile updated!");
            setTimeout(() => { setSuccess(""); onClose(); }, 1500);
          } catch (err: any) {
            setError(err.message || "Failed to update profile");
          }
        }}>
          <div className="profile-edit-field">
            <label>User Name
              <input
                value={form.username || ""}
                onChange={e => setForm(f => ({ ...f, username: e.target.value }))}
                ref={inputFirstRef}
                required
                aria-label="User Name"
                className="profile-edit-input"
              />
            </label>
          </div>
          <div className="profile-edit-field">
            <label>Email
              <input
                type="email"
                value={form.email || ""}
                onChange={e => setForm(f => ({ ...f, email: e.target.value }))}
                required
                aria-label="Email"
                className="profile-edit-input"
              />
            </label>
          </div>
          <div className="profile-edit-field">
            <label>Full Name
              <input
                value={form.full_name || ""}
                onChange={e => setForm(f => ({ ...f, full_name: e.target.value }))}
                aria-label="Full Name"
                className="profile-edit-input"
              />
            </label>
          </div>
          <div className="profile-edit-field">
            <label>Image URL
              <input
                value={form.image_url || ""}
                onChange={e => {
                  setImgError(false);
                  setForm(f => ({ ...f, image_url: e.target.value }));
                }}
                aria-label="Image URL"
                className="profile-edit-input"
              />
              <div className="profile-edit-field-hint">PNG/JPG/GIF/WebP URL. Leave empty for default.</div>
            </label>
          </div>
          {error && <div className="profile-edit-banner profile-edit-banner-error" role="alert">{error}</div>}
          {success && <div className="profile-edit-banner profile-edit-banner-success">{success}</div>}
          <div className="profile-edit-actions">
            <button type="button" className="profile-edit-btn secondary" onClick={onClose}>Cancel</button>
            <button type="submit" className="profile-edit-btn primary">Save</button>
          </div>
        </form>
      </div>
    </div>
  );
}
