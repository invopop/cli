package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Credentials are what a login leaves behind for one API host: the user's
// OAuth tokens, the workspace they chose, and the most recent workspace
// token minted for it.
//
// The user tokens reach every workspace the user belongs to, so they never
// leave this file: API calls are made with the short-lived workspace token
// only, which is what keeps an agent driving the CLI inside one workspace.
type Credentials struct {
	ClientID   string `json:"client_id"`
	AccessHost string `json:"access_host"`

	UserID       string    `json:"user_id,omitempty"`
	Email        string    `json:"email,omitempty"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`

	WorkspaceID   string `json:"workspace_id,omitempty"`
	WorkspaceName string `json:"workspace_name,omitempty"`

	WorkspaceToken          string    `json:"workspace_token,omitempty"`
	WorkspaceTokenExpiresAt time.Time `json:"workspace_token_expires_at"`
}

// RefreshUntil is the latest the user token can be renewed without signing
// in again, read from the refresh token's own expiry. Access moves it on with
// every refresh and can end it sooner by revoking the sign-in. Zero if unknown.
func (c *Credentials) RefreshUntil() time.Time {
	parts := strings.Split(c.RefreshToken, ".")
	if len(parts) != 3 {
		return time.Time{}
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return time.Time{}
	}
	// Read only for display; access verifies the signature when it is used.
	var p struct {
		Expires int64 `json:"exp"`
	}
	if json.Unmarshal(raw, &p) != nil || p.Expires == 0 {
		return time.Time{}
	}
	return time.Unix(p.Expires, 0).UTC()
}

// Store persists credentials for each API host in a single file readable
// only by the current user.
type Store struct {
	Path string
}

// DefaultStore places credentials in the user's config directory, e.g.
// ~/.config/invopop/credentials.json on Linux.
func DefaultStore() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("locating config directory: %w", err)
	}
	return &Store{Path: filepath.Join(dir, "invopop", "credentials.json")}, nil
}

func (s *Store) readAll() (map[string]*Credentials, error) {
	all := make(map[string]*Credentials)
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return all, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading credentials: %w", err)
	}
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, fmt.Errorf("parsing credentials %s: %w", s.Path, err)
	}
	return all, nil
}

func (s *Store) writeAll(all map[string]*Credentials) error {
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	// Write a temp file of our own then rename, so a failed write never
	// leaves a truncated file and concurrent writers never share one.
	f, err := os.CreateTemp(filepath.Dir(s.Path), filepath.Base(s.Path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}
	tmp := f.Name()
	defer os.Remove(tmp) //nolint:errcheck // gone after a successful rename
	if err := f.Chmod(0o600); err != nil {
		f.Close() //nolint:errcheck,gosec
		return fmt.Errorf("writing credentials: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close() //nolint:errcheck,gosec
		return fmt.Errorf("writing credentials: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}
	return os.Rename(tmp, s.Path)
}

// update runs fn on the stored credentials and writes the result, holding a
// lock throughout so concurrent CLI processes (an agent often runs several)
// cannot lose each other's changes.
func (s *Store) update(fn func(all map[string]*Credentials) bool) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	unlock, err := lockFile(s.Path + ".lock")
	if err != nil {
		return err
	}
	defer unlock()
	all, err := s.readAll()
	if err != nil {
		return err
	}
	if !fn(all) {
		return nil
	}
	return s.writeAll(all)
}

// lockStale is when a lock file is presumed left by a process that died.
const lockStale = 30 * time.Second

// lockFile takes an exclusive lock by creating path, which works the same on
// every platform the CLI ships for. It waits for another holder, and breaks a
// lock older than lockStale.
func lockFile(path string) (func(), error) {
	deadline := time.Now().Add(10 * time.Second)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()                              //nolint:errcheck,gosec
			return func() { os.Remove(path) }, nil //nolint:errcheck
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("locking credentials: %w", err)
		}
		if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) > lockStale {
			os.Remove(path) //nolint:errcheck,gosec // stale; retry at once
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("locking credentials: %s is held by another invopop process", path)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Load returns the credentials saved for host, or nil if there are none.
func (s *Store) Load(host string) (*Credentials, error) {
	all, err := s.readAll()
	if err != nil {
		return nil, err
	}
	return all[host], nil
}

// Save stores the credentials for host, replacing any already there.
func (s *Store) Save(host string, c *Credentials) error {
	return s.update(func(all map[string]*Credentials) bool {
		all[host] = c
		return true
	})
}

// Delete forgets the credentials for host. Deleting what is not there is not
// an error.
func (s *Store) Delete(host string) error {
	return s.update(func(all map[string]*Credentials) bool {
		if _, ok := all[host]; !ok {
			return false
		}
		delete(all, host)
		return true
	})
}
