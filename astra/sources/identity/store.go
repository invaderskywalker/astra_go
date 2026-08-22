// Package identity provides Astra's private, single-user local identity.
//
// The CLI intentionally has no directory-derived accounts and no database
// account table. A profile and a short-lived login marker live below the
// user's private ~/.astra directory with restrictive permissions.
package identity

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

const LocalUserID = 1

var ErrNotSignedUp = errors.New("no Astra profile exists; run `astra signup`")
var ErrNotLoggedIn = errors.New("you are not logged in; run `astra login`")
var ErrInvalidCredentials = errors.New("invalid Astra credentials")

type Profile struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email,omitempty"`
	PasswordSalt string    `json:"password_salt"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type LoginSession struct {
	ProfileID  string    `json:"profile_id"`
	LoggedInAt time.Time `json:"logged_in_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type Store struct{ root string }

func New(root string) Store { return Store{root: root} }

func Default() Store {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return New(filepath.Join(".astra", "identity"))
	}
	return New(filepath.Join(home, ".astra", "identity"))
}

func (s Store) Root() string        { return s.root }
func (s Store) profilePath() string { return filepath.Join(s.root, "profile.json") }
func (s Store) sessionPath() string { return filepath.Join(s.root, "login.json") }

func (s Store) Profile() (Profile, error) {
	data, err := os.ReadFile(s.profilePath())
	if os.IsNotExist(err) {
		return Profile{}, ErrNotSignedUp
	}
	if err != nil {
		return Profile{}, err
	}
	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return Profile{}, fmt.Errorf("read Astra profile: %w", err)
	}
	if profile.ID == "" || profile.PasswordHash == "" || profile.PasswordSalt == "" {
		return Profile{}, fmt.Errorf("Astra profile is incomplete; remove %s and run `astra signup`", s.profilePath())
	}
	return profile, nil
}

func (s Store) LoggedIn() (Profile, error) {
	profile, err := s.Profile()
	if err != nil {
		return Profile{}, err
	}
	data, err := os.ReadFile(s.sessionPath())
	if os.IsNotExist(err) {
		return Profile{}, ErrNotLoggedIn
	}
	if err != nil {
		return Profile{}, err
	}
	var session LoginSession
	if err := json.Unmarshal(data, &session); err != nil || session.ProfileID != profile.ID {
		return Profile{}, ErrNotLoggedIn
	}
	session.LastSeenAt = time.Now().UTC()
	if err := s.writeJSON(s.sessionPath(), session, 0600); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func (s Store) Signup(name, email, password string) (Profile, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if name == "" {
		return Profile{}, errors.New("name is required")
	}
	if len(password) < 8 {
		return Profile{}, errors.New("password must be at least 8 characters")
	}
	if _, err := s.Profile(); err == nil {
		return Profile{}, errors.New("an Astra profile already exists; use `astra login`")
	} else if !errors.Is(err, ErrNotSignedUp) {
		return Profile{}, err
	}
	salt, err := randomBytes(16)
	if err != nil {
		return Profile{}, err
	}
	idBytes, err := randomBytes(16)
	if err != nil {
		return Profile{}, err
	}
	now := time.Now().UTC()
	profile := Profile{ID: "user_" + hex.EncodeToString(idBytes), Name: name, Email: email, PasswordSalt: hex.EncodeToString(salt), PasswordHash: hashPassword(password, salt), CreatedAt: now, UpdatedAt: now}
	if err := s.writeJSON(s.profilePath(), profile, 0600); err != nil {
		return Profile{}, err
	}
	if err := s.Login(name, password); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func (s Store) Login(identifier, password string) error {
	profile, err := s.Profile()
	if err != nil {
		return err
	}
	identifier = strings.TrimSpace(identifier)
	if !strings.EqualFold(identifier, profile.Name) && !strings.EqualFold(identifier, profile.Email) {
		return ErrInvalidCredentials
	}
	salt, err := hex.DecodeString(profile.PasswordSalt)
	if err != nil || !checkPassword(password, salt, profile.PasswordHash) {
		return ErrInvalidCredentials
	}
	now := time.Now().UTC()
	return s.writeJSON(s.sessionPath(), LoginSession{ProfileID: profile.ID, LoggedInAt: now, LastSeenAt: now}, 0600)
}

func (s Store) Logout() error {
	if err := os.Remove(s.sessionPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func PromptCredentials(stdinTTY, stdoutTTY bool) (name, email, password string, err error) {
	if !stdinTTY || !stdoutTTY {
		return "", "", "", errors.New("interactive signup/login requires a terminal; use command flags for automation")
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Name: ")
	nameLine, err := reader.ReadString('\n')
	if err != nil {
		return "", "", "", err
	}
	fmt.Print("Email (optional): ")
	emailLine, _ := reader.ReadString('\n')
	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", "", "", err
	}
	return strings.TrimSpace(nameLine), strings.TrimSpace(emailLine), string(passwordBytes), nil
}

func (s Store) writeJSON(path string, value any, mode os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.root, 0700); err != nil {
		return err
	}
	if err := os.Chmod(s.root, 0700); err != nil && !os.IsNotExist(err) {
		return err
	}
	temporary, err := os.CreateTemp(s.root, ".identity-")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}

func randomBytes(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	return b, err
}

func hashPassword(password string, salt []byte) string {
	digest, err := bcrypt.GenerateFromPassword(append(append([]byte{}, salt...), []byte(password)...), bcrypt.DefaultCost)
	if err != nil {
		return ""
	}
	return string(digest)
}

func checkPassword(password string, salt []byte, encoded string) bool {
	return bcrypt.CompareHashAndPassword([]byte(encoded), append(append([]byte{}, salt...), []byte(password)...)) == nil
}
