package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const (
	testClientID = "cli"
	testRefresh  = "refresh-1"
)

// fakeInvopop stands in for both the access service (OAuth) and the API
// (orgs listing and workspace token exchange).
type fakeInvopop struct {
	t *testing.T

	mu        sync.Mutex
	challenge string
	grants    []url.Values
	mints     int
	tokenTTL  int64
}

func (f *fakeInvopop) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth/v1/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.grants = append(f.grants, r.PostForm)
		if r.PostForm.Get("client_secret") != "" {
			f.t.Error("a public client must never send a secret")
		}
		switch r.PostForm.Get(paramGrantType) {
		case "authorization_code":
			sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
			if base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge {
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{"error_description": "code verifier hash mismatch"})
				return
			}
		case paramRefreshToken:
			if r.PostForm.Get(paramRefreshToken) != testRefresh {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error_description": "bad refresh"})
				return
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":    "user-" + r.PostForm.Get(paramGrantType),
			paramRefreshToken: testRefresh,
			"expires_in":      f.tokenTTL,
			"user_id":         "u1",
			"info":            map[string]string{"email": "user@example.com"},
		})
	})
	mux.HandleFunc("GET /access/v1/orgs", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode([]*Org{{
			ID: "o1", Name: "Org",
			Workspaces: []*Workspace{
				{ID: "w1", Name: "Live", Slug: "live"},
				{ID: "w2", Name: "Test", Slug: "test", Sandbox: true},
			},
		}})
	})
	mux.HandleFunc("POST /access/v1/workspace/{id}/token", func(w http.ResponseWriter, r *http.Request) {
		var in mintRequest
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.ClientID != testClientID {
			f.t.Errorf("mint must name the client, got %q", in.ClientID)
		}
		f.mu.Lock()
		f.mints++
		n := f.mints
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(&mintResponse{
			AccessToken: "ws-" + r.PathValue("id") + "-" + string(rune('0'+n)),
			WorkspaceID: r.PathValue("id"),
			ExpiresAt:   time.Now().Add(30 * time.Minute).Unix(),
		})
	})
	return mux
}

func setup(t *testing.T) (*fakeInvopop, *Session) {
	t.Helper()
	f := &fakeInvopop{t: t, tokenTTL: 3600}
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	s := &Session{
		Store: &Store{Path: filepath.Join(t.TempDir(), "invopop", "credentials.json")},
		Endpoints: &Endpoints{
			APIHost:    srv.URL,
			AccessHost: srv.URL,
			ClientID:   testClientID,
		},
	}
	return f, s
}

// browser plays the part of the user signing in: it checks the authorize URL
// and follows the redirect back to the CLI with a code.
func (f *fakeInvopop) browser(state string) func(string) error {
	return func(raw string) error {
		u, err := url.Parse(raw)
		if err != nil {
			return err
		}
		q := u.Query()
		if q.Get("code_challenge_method") != "S256" {
			f.t.Errorf("challenge method = %q, want S256", q.Get("code_challenge_method"))
		}
		if q.Get(paramClientID) != testClientID {
			f.t.Errorf("client_id = %q", q.Get("client_id"))
		}
		f.mu.Lock()
		f.challenge = q.Get("code_challenge")
		f.mu.Unlock()
		if state == "" {
			state = q.Get("state")
		}
		go func() {
			res, err := http.Get(q.Get(paramRedirectURI) + "?code=abc&state=" + url.QueryEscape(state))
			if err == nil {
				res.Body.Close() //nolint:errcheck
			}
		}()
		return nil
	}
}

func TestLoginAndWorkspaceToken(t *testing.T) {
	f, s := setup(t)
	ctx := context.Background()

	c, err := Login(ctx, s.Endpoints, &LoginOptions{OpenBrowser: f.browser(""), Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if c.AccessToken != "user-authorization_code" || c.Email != "user@example.com" {
		t.Fatalf("unexpected credentials: %+v", c)
	}
	if err := s.Save(c); err != nil {
		t.Fatal(err)
	}

	if _, err := s.WorkspaceToken(ctx); err == nil {
		t.Fatal("expected an error before a workspace is chosen")
	}

	if err := s.SelectWorkspace(ctx, c, &Workspace{ID: "w2", Name: "Test"}); err != nil {
		t.Fatalf("select: %v", err)
	}
	tok, err := s.WorkspaceToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tok != "ws-w2-1" {
		t.Fatalf("token = %q", tok)
	}

	// A token with time left is reused rather than minted again.
	if tok, _ := s.WorkspaceToken(ctx); tok != "ws-w2-1" || f.mints != 1 {
		t.Fatalf("expected reuse, got %q after %d mints", tok, f.mints)
	}

	// Once near expiry a fresh one is minted.
	stored, _ := s.Load()
	stored.WorkspaceTokenExpiresAt = time.Now().Add(30 * time.Second)
	if err := s.Save(stored); err != nil {
		t.Fatal(err)
	}
	if tok, _ := s.WorkspaceToken(ctx); tok != "ws-w2-2" {
		t.Fatalf("expected a fresh token, got %q", tok)
	}

	info, err := os.Stat(s.Store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credentials file mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestUserTokenRefresh(t *testing.T) {
	f, s := setup(t)
	ctx := context.Background()

	c := &Credentials{
		ClientID:     testClientID,
		AccessToken:  "old",
		RefreshToken: testRefresh,
		ExpiresAt:    time.Now().Add(-time.Minute),
		WorkspaceID:  "w1",
	}
	if err := s.Save(c); err != nil {
		t.Fatal(err)
	}
	tok, err := s.WorkspaceToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tok != "ws-w1-1" {
		t.Fatalf("token = %q", tok)
	}
	if len(f.grants) != 1 || f.grants[0].Get(paramGrantType) != paramRefreshToken {
		t.Fatalf("expected one refresh grant, got %v", f.grants)
	}
	stored, _ := s.Load()
	if stored.AccessToken != "user-refresh_token" {
		t.Fatalf("refreshed user token not stored: %q", stored.AccessToken)
	}

	t.Run("a rejected refresh asks for a new login", func(t *testing.T) {
		stored.RefreshToken = "revoked"
		stored.ExpiresAt = time.Now().Add(-time.Minute)
		stored.WorkspaceToken = ""
		if err := s.Save(stored); err != nil {
			t.Fatal(err)
		}
		_, err := s.WorkspaceToken(ctx)
		if err == nil {
			t.Fatal("expected an error")
		}
	})
}

func TestLoginRejectsWrongState(t *testing.T) {
	f, s := setup(t)
	// The browser comes back with someone else's state: the login must not
	// accept that code, and keeps waiting until it times out.
	_, err := Login(context.Background(), s.Endpoints, &LoginOptions{
		OpenBrowser: f.browser("forged"),
		Timeout:     500 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected the login to fail")
	}
	if len(f.grants) != 0 {
		t.Fatalf("no code should have been redeemed, got %v", f.grants)
	}
}

func TestNotLoggedIn(t *testing.T) {
	_, s := setup(t)
	if _, err := s.WorkspaceToken(context.Background()); err != ErrNotLoggedIn {
		t.Fatalf("err = %v, want ErrNotLoggedIn", err)
	}
}

func TestStoreKeepsHostsApart(t *testing.T) {
	st := &Store{Path: filepath.Join(t.TempDir(), "credentials.json")}
	if err := st.Save("https://api.invopop.com", &Credentials{AccessToken: "prod"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Save("https://api.invopop.dev", &Credentials{AccessToken: "dev"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Delete("https://api.invopop.dev"); err != nil {
		t.Fatal(err)
	}
	prod, _ := st.Load("https://api.invopop.com")
	dev, _ := st.Load("https://api.invopop.dev")
	if prod == nil || prod.AccessToken != "prod" || dev != nil {
		t.Fatalf("prod = %+v, dev = %+v", prod, dev)
	}
}

func TestAccessHostFor(t *testing.T) {
	cases := map[string]string{
		"https://api.invopop.com":     "https://access.invopop.com",
		"https://api.invopop.dev/":    "https://access.invopop.dev",
		"http://localhost:3000":       "",
		"https://gateway.invopop.com": "",
	}
	for in, want := range cases {
		if got := AccessHostFor(in); got != want {
			t.Errorf("AccessHostFor(%q) = %q, want %q", in, got, want)
		}
	}
}
