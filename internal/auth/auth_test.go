package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	listings  int
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
		f.mu.Lock()
		f.listings++
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode([]*Org{{
			ID: "o1", Name: "Org",
			Workspaces: []*Workspace{
				{ID: "w1", Name: "Live", Slug: "live"},
				{ID: "w2", Name: "Test", Slug: "test", Sandbox: true},
				{ID: "w3", Name: "Live (sandbox)", Slug: "live", Sandbox: true},
			},
		}})
	})
	mux.HandleFunc("GET /echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Header.Get("Authorization")))
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
// and remembers the challenge for the token endpoint to verify.
func (f *fakeInvopop) browser(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" {
		f.t.Errorf("challenge method = %q, want S256", q.Get("code_challenge_method"))
	}
	if q.Get(paramClientID) != testClientID {
		f.t.Errorf("client_id = %q", q.Get(paramClientID))
	}
	if q.Get(paramRedirectURI) != redirectOOB {
		f.t.Errorf("redirect_uri = %q, want out-of-band", q.Get(paramRedirectURI))
	}
	f.mu.Lock()
	f.challenge = q.Get("code_challenge")
	f.mu.Unlock()
	return nil
}

func pasted(code string) func() (string, error) {
	return func() (string, error) { return code, nil }
}

func TestLoginAndWorkspaceToken(t *testing.T) {
	f, s := setup(t)
	ctx := context.Background()

	c, err := Login(ctx, s.Endpoints, &LoginOptions{OpenBrowser: f.browser, ReadCode: pasted("  abc\n")})
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
	stored.Workspaces["w2"].TokenExpiresAt = time.Now().Add(30 * time.Second)
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
		stored.Workspaces["w1"].Token = ""
		if err := s.Save(stored); err != nil {
			t.Fatal(err)
		}
		_, err := s.WorkspaceToken(ctx)
		if err == nil {
			t.Fatal("expected an error")
		}
	})
}

func TestLoginNeedsACode(t *testing.T) {
	f, s := setup(t)
	_, err := Login(context.Background(), s.Endpoints, &LoginOptions{OpenBrowser: f.browser, ReadCode: pasted("\n")})
	if err == nil {
		t.Fatal("expected an error for an empty code")
	}
	if len(f.grants) != 0 {
		t.Fatalf("nothing should have been redeemed, got %v", f.grants)
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

func TestLoginRefusesMismatchedEnvironments(t *testing.T) {
	ep := &Endpoints{
		APIHost:    "https://api.invopop.com",
		AccessHost: "https://access.invopop.dev",
		ClientID:   testClientID,
	}
	_, err := Login(context.Background(), ep, &LoginOptions{ReadCode: pasted("abc")})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("err = %v, want a host mismatch", err)
	}

	ep.AccessHost = "https://access.invopop.com/"
	if err := ep.validate(); err != nil {
		t.Fatalf("matching hosts refused: %v", err)
	}
}

func TestRefreshUntil(t *testing.T) {
	exp := time.Date(2026, 10, 5, 15, 1, 34, 0, time.UTC)
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"src":"a","exp":` + strconv.FormatInt(exp.Unix(), 10) + `}`))
	c := &Credentials{RefreshToken: "eyJhbGciOiJFUzI1NiJ9." + payload + ".sig"}
	if got := c.RefreshUntil(); !got.Equal(exp) {
		t.Fatalf("RefreshUntil = %v, want %v", got, exp)
	}
	for _, tok := range []string{"", "not-a-token", "a.!!!.c", "a." + base64.RawURLEncoding.EncodeToString([]byte("{}")) + ".c"} {
		if got := (&Credentials{RefreshToken: tok}).RefreshUntil(); !got.IsZero() {
			t.Errorf("RefreshUntil(%q) = %v, want zero", tok, got)
		}
	}
}

func TestEndpointsRequireHTTPS(t *testing.T) {
	ep := &Endpoints{APIHost: "https://api.invopop.com", AccessHost: "http://access.invopop.com", ClientID: testClientID}
	if err := ep.validate(); err == nil {
		t.Fatal("plain http access host accepted")
	}
	ep = &Endpoints{APIHost: "http://127.0.0.1:3000", AccessHost: "http://localhost:3001", ClientID: testClientID}
	if err := ep.validate(); err != nil {
		t.Fatalf("loopback http refused: %v", err)
	}
}

func TestNormalizeHost(t *testing.T) {
	for in, want := range map[string]string{
		"https://api.invopop.com/":  "https://api.invopop.com",
		"HTTPS://API.Invopop.com":   "https://api.invopop.com",
		" https://api.invopop.dev ": "https://api.invopop.dev",
		"http://127.0.0.1:3000/":    "http://127.0.0.1:3000",
	} {
		if got := NormalizeHost(in); got != want {
			t.Errorf("NormalizeHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStoreConcurrentSaves(t *testing.T) {
	st := &Store{Path: filepath.Join(t.TempDir(), "invopop", "credentials.json")}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := st.Save("https://host"+strconv.Itoa(i), &Credentials{AccessToken: strconv.Itoa(i)}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	for i := 0; i < 20; i++ {
		c, err := st.Load("https://host" + strconv.Itoa(i))
		if err != nil || c == nil || c.AccessToken != strconv.Itoa(i) {
			t.Fatalf("host %d lost: %+v %v", i, c, err)
		}
	}
	entries, _ := os.ReadDir(filepath.Dir(st.Path))
	if len(entries) != 1 {
		t.Fatalf("left behind temp or lock files: %v", entries)
	}
}

func TestFindWorkspace(t *testing.T) {
	list := []*Workspace{
		{ID: "w1", Name: "Live", Slug: "shop"},
		{ID: "w2", Name: "Shop sandbox", Slug: "shop", Sandbox: true},
		{ID: "w3", Name: "Lab", Slug: "lab", Sandbox: true},
		{ID: "w4", Name: "Other org", Slug: "twin"},
		{ID: "w5", Name: "Another org", Slug: "twin"},
	}
	for ref, want := range map[string]string{
		"w2":           "w2",
		"shop":         "w1",
		"SHOP":         "w1",
		"sandbox/shop": "w2",
		"Sandbox/Shop": "w2",
		"sandbox/lab":  "w3",
	} {
		ws, err := FindWorkspace(list, ref)
		if err != nil || ws.ID != want {
			t.Errorf("FindWorkspace(%q) = %v, %v; want %s", ref, ws, err, want)
		}
	}

	// A bare slug never picks a sandbox, but points to it.
	if _, err := FindWorkspace(list, "lab"); err == nil || !strings.Contains(err.Error(), `"sandbox/lab"`) {
		t.Errorf("bare sandbox slug: err = %v", err)
	}
	if _, err := FindWorkspace(list, "sandbox/"); !errors.Is(err, ErrUnknownWorkspace) {
		t.Errorf("empty sandbox slug: err = %v", err)
	}
	if _, err := FindWorkspace(list, "nope"); !errors.Is(err, ErrUnknownWorkspace) {
		t.Errorf("unknown: err = %v", err)
	}
	if _, err := FindWorkspace(list, "twin"); err == nil || !strings.Contains(err.Error(), "w4") || !strings.Contains(err.Error(), "w5") {
		t.Errorf("ambiguous slug: err = %v", err)
	}
	if got := list[1].Ref(); got != "sandbox/shop" {
		t.Errorf("Ref = %q", got)
	}
	if got := (&Workspace{ID: "w9", Sandbox: true}).Ref(); got != "w9" {
		t.Errorf("Ref without a slug = %q, want the ID", got)
	}
}

// loggedIn signs in and makes w1 the default.
func loggedIn(t *testing.T, f *fakeInvopop, s *Session) *Credentials {
	t.Helper()
	ctx := context.Background()
	c, err := Login(ctx, s.Endpoints, &LoginOptions{OpenBrowser: f.browser, ReadCode: pasted("abc")})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(c); err != nil {
		t.Fatal(err)
	}
	ws, err := s.ResolveWorkspace(ctx, c, "live")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SelectWorkspace(ctx, c, ws); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestWorkspaceOverride(t *testing.T) {
	f, s := setup(t)
	ctx := context.Background()
	loggedIn(t, f, s)
	if f.listings != 1 || f.mints != 1 {
		t.Fatalf("after login: %d listings, %d mints", f.listings, f.mints)
	}

	other := *s
	other.Workspace = "sandbox/live"
	tok, err := other.WorkspaceToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tok != "ws-w3-2" {
		t.Fatalf("override token = %q", tok)
	}
	// Found from what the login listed, without asking again.
	if f.listings != 1 {
		t.Fatalf("slug lookup listed workspaces again: %d", f.listings)
	}

	// The default is untouched, and each workspace keeps its own token, so
	// alternating mints nothing new.
	for range 2 {
		if tok, _ := s.WorkspaceToken(ctx); tok != "ws-w1-1" {
			t.Fatalf("default token = %q", tok)
		}
		if tok, _ := other.WorkspaceToken(ctx); tok != "ws-w3-2" {
			t.Fatalf("override token = %q", tok)
		}
	}
	if f.mints != 2 {
		t.Fatalf("mints = %d, want 2", f.mints)
	}
	stored, _ := s.Load()
	if stored.WorkspaceID != "w1" {
		t.Fatalf("default changed to %q", stored.WorkspaceID)
	}

	cur, err := other.Current(ctx, stored)
	if err != nil || cur.ID != "w3" || cur.Ref() != "sandbox/live" {
		t.Fatalf("Current = %+v, %v", cur, err)
	}

	t.Run("an ID not yet known is looked up", func(t *testing.T) {
		stored, _ := s.Load()
		delete(stored.Workspaces, "w2")
		if err := s.Save(stored); err != nil {
			t.Fatal(err)
		}
		byID := *s
		byID.Workspace = "w2"
		if _, err := byID.WorkspaceToken(ctx); err != nil {
			t.Fatal(err)
		}
		if f.listings != 2 {
			t.Fatalf("listings = %d, want 2", f.listings)
		}
	})

	t.Run("unknown workspace", func(t *testing.T) {
		bad := *s
		bad.Workspace = "missing"
		if _, err := bad.WorkspaceToken(ctx); !errors.Is(err, ErrUnknownWorkspace) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestLegacyWorkspaceTokenMigrates(t *testing.T) {
	f, s := setup(t)
	legacy := map[string]any{
		s.Endpoints.APIHost: map[string]any{
			"client_id":                  testClientID,
			"access_token":               "user",
			"expires_at":                 time.Now().Add(time.Hour),
			"workspace_id":               "w1",
			"workspace_name":             "Live",
			"workspace_token":            "old-ws-token",
			"workspace_token_expires_at": time.Now().Add(20 * time.Minute),
		},
	}
	data, _ := json.Marshal(legacy)
	if err := os.MkdirAll(filepath.Dir(s.Store.Path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Store.Path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	tok, err := s.WorkspaceToken(context.Background())
	if err != nil || tok != "old-ws-token" || f.mints != 0 {
		t.Fatalf("token = %q, %v after %d mints", tok, err, f.mints)
	}
	c, _ := s.Load()
	if err := s.Save(c); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(s.Store.Path)
	if strings.Contains(string(raw), `"workspace_token"`) {
		t.Fatalf("legacy field written back: %s", raw)
	}
}

func TestLoginDropsWorkspaceTokens(t *testing.T) {
	f, s := setup(t)
	c := loggedIn(t, f, s)
	c.applyToken(&oauthToken{AccessToken: "new", ExpiresIn: 3600})
	if c.Workspaces["w1"].Token != "" || c.Workspaces["w1"].Slug != "live" {
		t.Fatalf("after a new user token: %+v", c.Workspaces["w1"])
	}
}

func TestSelectWorkspaceKeepsNewerCredentials(t *testing.T) {
	f, s := setup(t)
	ctx := context.Background()
	loggedIn(t, f, s)
	snapshot, _ := s.Load()

	// Meanwhile another terminal renews the sign-in and caches a token for
	// another workspace.
	other := *s
	other.Workspace = "sandbox/test"
	if _, err := other.WorkspaceToken(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.modify(s.Endpoints.APIHost, func(c *Credentials) bool {
		c.RefreshToken = "rotated"
		return true
	}); err != nil {
		t.Fatal(err)
	}

	ws, err := s.ResolveWorkspace(ctx, snapshot, "sandbox/live")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SelectWorkspace(ctx, snapshot, ws); err != nil {
		t.Fatal(err)
	}
	stored, _ := s.Load()
	if stored.WorkspaceID != "w3" {
		t.Errorf("default = %q, want w3", stored.WorkspaceID)
	}
	if stored.RefreshToken != "rotated" {
		t.Errorf("refresh token put back to %q", stored.RefreshToken)
	}
	if stored.Workspaces["w2"].Token == "" {
		t.Error("another workspace's token was dropped")
	}
}

func TestRefreshKeepsNewerDefault(t *testing.T) {
	f, s := setup(t)
	ctx := context.Background()
	loggedIn(t, f, s)

	// This command read the credentials with the user token about to expire...
	stale, _ := s.Load()
	stale.ExpiresAt = time.Now()
	if err := s.Save(stale); err != nil {
		t.Fatal(err)
	}
	stale, _ = s.Load()
	// ...then another terminal changed the default.
	if err := s.Store.modify(s.Endpoints.APIHost, func(c *Credentials) bool {
		c.WorkspaceID = "w2"
		return true
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.UserToken(ctx, stale); err != nil {
		t.Fatal(err)
	}
	stored, _ := s.Load()
	if stored.AccessToken != "user-refresh_token" {
		t.Errorf("renewed token not stored: %q", stored.AccessToken)
	}
	if stored.WorkspaceID != "w2" {
		t.Errorf("default put back to %q", stored.WorkspaceID)
	}
}

func TestTransportRenewsTokens(t *testing.T) {
	f, s := setup(t)
	loggedIn(t, f, s)
	hc := &http.Client{Transport: s.Transport(nil)}
	get := func() string {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, s.Endpoints.APIHost+"/echo", nil)
		req.Header.Set("Authorization", "Bearer stale")
		res, err := hc.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close() //nolint:errcheck
		b, _ := io.ReadAll(res.Body)
		if req.Header.Get("Authorization") != "Bearer stale" {
			t.Error("the caller's request was changed")
		}
		return string(b)
	}

	if got := get(); got != "Bearer ws-w1-1" {
		t.Fatalf("Authorization = %q", got)
	}
	// Once the token is near expiry, the next request gets a fresh one.
	if err := s.Store.modify(s.Endpoints.APIHost, func(c *Credentials) bool {
		c.Workspaces["w1"].TokenExpiresAt = time.Now()
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if got := get(); got != "Bearer ws-w1-2" {
		t.Fatalf("Authorization after expiry = %q", got)
	}
}
