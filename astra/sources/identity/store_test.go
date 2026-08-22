package identity

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSignupLoginLogoutUsesPrivateFiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), "identity")
	store := New(root)
	profile, err := store.Signup("Astra Owner", "owner@example.test", "correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	if profile.ID == "" {
		t.Fatal("expected profile id")
	}
	if _, err := store.LoggedIn(); err != nil {
		t.Fatalf("signup should log in: %v", err)
	}
	if info, err := os.Stat(root); err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("identity root must be private: info=%v err=%v", info, err)
	}
	if err := store.Logout(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoggedIn(); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected logged-out error, got %v", err)
	}
	if err := store.Login("owner@example.test", "correct-horse-battery"); err != nil {
		t.Fatal(err)
	}
	if err := store.Login("owner@example.test", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

func TestSignupRejectsSecondProfile(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "identity"))
	if _, err := store.Signup("Owner", "", "12345678"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Signup("Second", "", "12345678"); err == nil {
		t.Fatal("expected one-user profile to reject second signup")
	}
}
