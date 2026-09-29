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
	// Workspace, when set, is an ID or slug naming the workspace to use
	// instead of the stored default, for this session only.
	Workspace string
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

// WorkspaceToken returns a token for the workspace in use that will stay
// valid for at least a minute, minting a new one when the stored one is too
// close to expiry.
func (s *Session) WorkspaceToken(ctx context.Context) (string, error) {
	c, err := s.Load()
	if err != nil {
		return "", err
	}
	ws, err := s.Current(ctx, c)
	if err != nil {
		return "", err
	}
	return s.tokenFor(ctx, c, ws.ID)
}

// Current is the workspace commands act in: the one named by s.Workspace,
// or else the stored default.
func (s *Session) Current(ctx context.Context, c *Credentials) (*Workspace, error) {
	if s.Workspace != "" {
		return s.ResolveWorkspace(ctx, c, s.Workspace)
	}
	if c.WorkspaceID == "" {
		return nil, errors.New("no workspace selected: run `invopop auth workspace`")
	}
	ws := &Workspace{ID: c.WorkspaceID, Name: c.WorkspaceName}
	if k := c.Workspaces[c.WorkspaceID]; k != nil {
		ws.Slug, ws.Sandbox = k.Slug, k.Sandbox
	}
	return ws, nil
}

// ResolveWorkspace finds the workspace ref names, as FindWorkspace does.
// Workspaces already known are matched without asking the API; anything
// else lists the user's workspaces afresh.
func (s *Session) ResolveWorkspace(ctx context.Context, c *Credentials, ref string) (*Workspace, error) {
	if ws, err := FindWorkspace(c.knownList(), ref); err == nil {
		return ws, nil
	}
	orgs, err := s.Orgs(ctx, c)
	if err != nil {
		return nil, err
	}
	var all []*Workspace
	for _, o := range orgs {
		all = append(all, o.Workspaces...)
	}
	return FindWorkspace(all, ref)
}

// TokenExpiry reports when the stored token for workspace id expires, zero
// if there is none.
func (c *Credentials) TokenExpiry(id string) time.Time {
	if k := c.Workspaces[id]; k != nil && k.Token != "" {
		return k.TokenExpiresAt
	}
	return time.Time{}
}

// tokenFor returns a usable token for workspace id, reusing the stored one
// while it has time left.
func (s *Session) tokenFor(ctx context.Context, c *Credentials, id string) (string, error) {
	if k := c.Workspaces[id]; k != nil && k.Token != "" && time.Until(k.TokenExpiresAt) > refreshMargin {
		return k.Token, nil
	}
	tok, exp, err := s.mint(ctx, c, id)
	if err != nil {
		return "", err
	}
	k := c.known(id)
	k.Token, k.TokenExpiresAt = tok, exp
	// Store just this token, so commands running at the same time in other
	// workspaces keep theirs.
	err = s.Store.modify(s.Endpoints.APIHost, func(stored *Credentials) bool {
		if stored.UserID != c.UserID {
			return false // signed in again as someone else meanwhile
		}
		sk := stored.known(id)
		sk.Token, sk.TokenExpiresAt = tok, exp
		if sk.Slug == "" {
			sk.Name, sk.Slug, sk.Sandbox = k.Name, k.Slug, k.Sandbox
		}
		return true
	})
	if err != nil {
		return "", err
	}
	return tok, nil
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

// SelectWorkspace makes ws the default workspace later commands act in.
func (s *Session) SelectWorkspace(ctx context.Context, c *Credentials, ws *Workspace) error {
	c.WorkspaceID = ws.ID
	c.WorkspaceName = ws.Name
	k := c.known(ws.ID)
	k.Name, k.Slug, k.Sandbox = ws.Name, ws.Slug, ws.Sandbox
	// Get a token straight away, so a workspace the user cannot use is
	// reported now rather than on the next command.
	if _, err := s.tokenFor(ctx, c, ws.ID); err != nil {
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
	if err := s.rememberWorkspaces(c, orgs); err != nil {
		return nil, err
	}
	return orgs, nil
}

// rememberWorkspaces records the workspaces listed so they can be found by
// slug later, forgetting any the user no longer belongs to. Stored tokens
// for those still listed are kept.
func (s *Session) rememberWorkspaces(c *Credentials, orgs []*Org) error {
	apply := func(c *Credentials) {
		old := c.Workspaces
		c.Workspaces = make(map[string]*KnownWorkspace)
		for _, o := range orgs {
			for _, w := range o.Workspaces {
				k := &KnownWorkspace{Name: w.Name, Slug: w.Slug, Sandbox: w.Sandbox}
				if prev := old[w.ID]; prev != nil {
					k.Token, k.TokenExpiresAt = prev.Token, prev.TokenExpiresAt
				}
				c.Workspaces[w.ID] = k
			}
		}
	}
	apply(c)
	return s.Store.modify(s.Endpoints.APIHost, func(stored *Credentials) bool {
		if stored.UserID != c.UserID {
			return false
		}
		apply(stored)
		return true
	})
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

// mint exchanges the user token for a token restricted to workspace id.
func (s *Session) mint(ctx context.Context, c *Credentials, id string) (string, time.Time, error) {
	ut, err := s.UserToken(ctx, c)
	if err != nil {
		return "", time.Time{}, err
	}
	body, err := json.Marshal(&mintRequest{ClientID: s.endpoints(c).ClientID})
	if err != nil {
		return "", time.Time{}, err
	}
	p := "/access/v1/workspace/" + url.PathEscape(id) + "/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL(p), bytes.NewReader(body))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Authorization", "Bearer "+ut)
	req.Header.Set("Content-Type", "application/json")
	out := new(mintResponse)
	if err := s.Endpoints.do(req, out); err != nil {
		return "", time.Time{}, fmt.Errorf("getting a token for workspace %s: %w", id, err)
	}
	if out.AccessToken == "" {
		return "", time.Time{}, errors.New("getting a workspace token: none returned")
	}
	return out.AccessToken, time.Unix(out.ExpiresAt, 0), nil
}

func (s *Session) apiURL(p string) string {
	return strings.TrimSuffix(s.Endpoints.APIHost, "/") + p
}
