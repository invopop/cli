package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	// Write then rename so a failed write never leaves a truncated file.
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}
	return os.Rename(tmp, s.Path)
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
	all, err := s.readAll()
	if err != nil {
		return err
	}
	all[host] = c
	return s.writeAll(all)
}

// Delete forgets the credentials for host. Deleting what is not there is not
// an error.
func (s *Store) Delete(host string) error {
	all, err := s.readAll()
	if err != nil {
		return err
	}
	if _, ok := all[host]; !ok {
		return nil
	}
	delete(all, host)
	return s.writeAll(all)
}
