package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrNotLoggedIn is returned when no credentials are stored for a host.
var ErrNotLoggedIn = errors.New("not logged in: run `invopop auth login`, or set INVOPOP_TOKEN")

// refreshMargin is how close to expiry a token may get before it is renewed,
// so a token does not lapse partway through a command.
const refreshMargin = time.Minute

// Session supplies API credentials from what a login stored.
type Session struct {
	Store     *Store
	Endpoints *Endpoints
}

// Load returns the stored credentials for the API host.
func (s *Session) Load() (*Credentials, error) {
	c, err := s.Store.Load(s.Endpoints.APIHost)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrNotLoggedIn
	}
	return c, nil
}

// Save stores credentials for the API host.
func (s *Session) Save(c *Credentials) error {
	return s.Store.Save(s.Endpoints.APIHost, c)
}

// endpoints fills in whatever the stored credentials already settled, so a
// later command does not need the flags the login was given.
func (s *Session) endpoints(c *Credentials) *Endpoints {
	ep := *s.Endpoints
	if c.ClientID != "" {
		ep.ClientID = c.ClientID
	}
	if c.AccessHost != "" {
		ep.AccessHost = c.AccessHost
	}
	return &ep
}

// WorkspaceToken returns a workspace token that will stay valid for at least
// a minute, minting a new one when the stored one is too close to expiry.
func (s *Session) WorkspaceToken(ctx context.Context) (string, error) {
	c, err := s.Load()
	if err != nil {
		return "", err
	}
	if c.WorkspaceID == "" {
		return "", errors.New("no workspace selected: run `invopop auth workspace`")
	}
	if c.WorkspaceToken != "" && time.Until(c.WorkspaceTokenExpiresAt) > refreshMargin {
		return c.WorkspaceToken, nil
	}
	if err := s.mint(ctx, c); err != nil {
		return "", err
	}
	if err := s.Save(c); err != nil {
		return "", err
	}
	return c.WorkspaceToken, nil
}

// UserToken returns the user's account-wide OAuth token, refreshed if
// needed. It is used only to list workspaces and to mint workspace tokens.
func (s *Session) UserToken(ctx context.Context, c *Credentials) (string, error) {
	if time.Until(c.ExpiresAt) > refreshMargin {
		return c.AccessToken, nil
	}
	if c.RefreshToken == "" {
		return "", errors.New("session expired: run `invopop auth login`")
	}
	tok, err := s.endpoints(c).grant(ctx, url.Values{
		paramGrantType:    {paramRefreshToken},
		paramClientID:     {s.endpoints(c).ClientID},
		paramRefreshToken: {c.RefreshToken},
	})
	if err != nil {
		var se *StatusError
		if errors.As(err, &se) && se.Code < 500 {
			return "", fmt.Errorf("session expired, run `invopop auth login`: %w", err)
		}
		return "", err
	}
	c.applyToken(tok)
	if err := s.Save(c); err != nil {
		return "", err
	}
	return c.AccessToken, nil
}

// SelectWorkspace makes id the workspace later commands act in.
func (s *Session) SelectWorkspace(ctx context.Context, c *Credentials, ws *Workspace) error {
	c.WorkspaceID = ws.ID
	c.WorkspaceName = ws.Name
	c.WorkspaceToken = ""
	c.WorkspaceTokenExpiresAt = time.Time{}
	// Mint straight away, so a workspace the user cannot use is reported
	// now rather than on the next command.
	if err := s.mint(ctx, c); err != nil {
		return err
	}
	return s.Save(c)
}

// Org is an organization the user belongs to, with its workspaces.
type Org struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Workspaces []*Workspace `json:"workspaces,omitempty"`
}

// Workspace is one the user can choose to work in.
type Workspace struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Sandbox bool   `json:"sandbox"`
}

// Orgs lists the user's orgs and their workspaces.
func (s *Session) Orgs(ctx context.Context, c *Credentials) ([]*Org, error) {
	ut, err := s.UserToken(ctx, c)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL("/access/v1/orgs"), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+ut)
	var orgs []*Org
	if err := s.Endpoints.do(req, &orgs); err != nil {
		var se *StatusError
		if errors.As(err, &se) && se.Code == http.StatusForbidden {
			// The sign-in worked but the token came back without the scopes
			// asked for, which access does when the OAuth app may not issue
			// them.
			return nil, fmt.Errorf("listing workspaces: %w (the CLI's OAuth app %s may not be allowed to issue the requested scopes)", err, s.endpoints(c).ClientID)
		}
		return nil, fmt.Errorf("listing workspaces: %w", err)
	}
	return orgs, nil
}

type mintRequest struct {
	ClientID string `json:"client_id"`
}

type mintResponse struct {
	AccessToken string `json:"access_token"`
	WorkspaceID string `json:"workspace_id"`
	Scope       string `json:"scope"`
	ExpiresAt   int64  `json:"expires_at"`
}

// mint exchanges the user token for a token restricted to c.WorkspaceID.
func (s *Session) mint(ctx context.Context, c *Credentials) error {
	ut, err := s.UserToken(ctx, c)
	if err != nil {
		return err
	}
	body, err := json.Marshal(&mintRequest{ClientID: s.endpoints(c).ClientID})
	if err != nil {
		return err
	}
	p := "/access/v1/workspace/" + url.PathEscape(c.WorkspaceID) + "/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL(p), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+ut)
	req.Header.Set("Content-Type", "application/json")
	out := new(mintResponse)
	if err := s.Endpoints.do(req, out); err != nil {
		return fmt.Errorf("getting a token for workspace %s: %w", c.WorkspaceID, err)
	}
	if out.AccessToken == "" {
		return errors.New("getting a workspace token: none returned")
	}
	c.WorkspaceToken = out.AccessToken
	c.WorkspaceTokenExpiresAt = time.Unix(out.ExpiresAt, 0)
	return nil
}

func (s *Session) apiURL(p string) string {
	return strings.TrimSuffix(s.Endpoints.APIHost, "/") + p
}
