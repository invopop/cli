// Package auth logs a user in to Invopop through OAuth and keeps the CLI
// supplied with short-lived workspace tokens.
//
// The CLI is a public OAuth client: it ships to anyone, so it cannot keep a
// client secret. The user signs in through the browser using the
// authorization code flow with PKCE (S256) and a loopback redirect, following
// RFC 8252. The resulting user token reaches every workspace the user belongs
// to, so it is only ever exchanged for a workspace token restricted to the
// workspace they chose, and narrowed to what they may do there. That
// workspace token is what API calls carry.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuth form parameters used by more than one request.
const (
	paramGrantType    = "grant_type"
	paramClientID     = "client_id"
	paramRedirectURI  = "redirect_uri"
	paramRefreshToken = "refresh_token"
)

// Endpoints identifies the services and OAuth client involved in a login.
type Endpoints struct {
	// APIHost is the base URL of the Invopop API, e.g. https://api.invopop.com.
	APIHost string
	// AccessHost is the base URL of the access service, which runs the
	// OAuth login pages and token endpoint, e.g. https://access.invopop.com.
	AccessHost string
	// ClientID of the CLI's public OAuth application.
	ClientID string
	// HTTPClient is used for every request; http.DefaultClient when nil.
	HTTPClient *http.Client
}

// AccessHostFor guesses the access service host from the API host by
// swapping a leading "api." for "access.", which is how Invopop's
// environments are laid out. Returns "" when there is nothing to swap.
func AccessHostFor(apiHost string) string {
	u, err := url.Parse(apiHost)
	if err != nil || !strings.HasPrefix(u.Host, "api.") {
		return ""
	}
	u.Host = "access." + strings.TrimPrefix(u.Host, "api.")
	u.Path = ""
	return u.String()
}

func (ep *Endpoints) client() *http.Client {
	if ep.HTTPClient != nil {
		return ep.HTTPClient
	}
	return http.DefaultClient
}

func (ep *Endpoints) validate() error {
	if ep.APIHost == "" {
		return errors.New("missing API host")
	}
	if ep.AccessHost == "" {
		return errors.New("missing access host: set --access-host or INVOPOP_ACCESS_HOST")
	}
	if ep.ClientID == "" {
		return errors.New("missing OAuth client ID: set --client-id or INVOPOP_CLIENT_ID")
	}
	return nil
}

// oauthToken is the access service's token endpoint response.
type oauthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	UserID       string `json:"user_id"`
	Info         struct {
		Email string `json:"email"`
	} `json:"info"`
}

// LoginOptions tune an interactive login.
type LoginOptions struct {
	// Scope requested from the user. The access service narrows it to what
	// the user may actually do in the chosen workspace, so asking broadly
	// is safe.
	Scope string
	// OpenBrowser is given the authorize URL; when nil or when it fails the
	// URL is only printed.
	OpenBrowser func(url string) error
	// Notify receives messages for the person at the terminal.
	Notify func(msg string)
	// Timeout bounds how long to wait for the browser to come back.
	Timeout time.Duration
}

// Login runs the browser sign-in and returns credentials holding the user's
// tokens. No workspace is chosen yet.
func Login(ctx context.Context, ep *Endpoints, opts *LoginOptions) (*Credentials, error) {
	if err := ep.validate(); err != nil {
		return nil, err
	}
	if opts == nil {
		opts = new(LoginOptions)
	}
	notify := opts.Notify
	if notify == nil {
		notify = func(string) {}
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	pk, err := newPKCE()
	if err != nil {
		return nil, err
	}
	state, err := randomString(16)
	if err != nil {
		return nil, err
	}

	// Listen on the IPv4 loopback literal rather than "localhost", as RFC
	// 8252 advises, on whatever port the system hands us.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting local callback listener: %w", err)
	}
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", ln.Addr().(*net.TCPAddr).Port)

	results := make(chan callbackResult, 1)
	srv := &http.Server{
		Handler:           callbackHandler(state, results),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close() //nolint:errcheck

	authURL := authorizeURL(ep, redirectURI, state, pk.Challenge, opts.Scope)
	notify("Opening your browser to sign in to Invopop. If it does not open, visit:\n\n  " + authURL + "\n")
	if opts.OpenBrowser != nil {
		if err := opts.OpenBrowser(authURL); err != nil {
			notify("Could not open a browser automatically: " + err.Error())
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var res callbackResult
	select {
	case res = <-results:
	case <-ctx.Done():
		return nil, errors.New("timed out waiting for the browser sign-in to complete")
	}
	if res.err != nil {
		return nil, res.err
	}

	tok, err := ep.grant(ctx, url.Values{
		paramGrantType:   {"authorization_code"},
		paramClientID:    {ep.ClientID},
		"code":           {res.code},
		"code_verifier":  {pk.Verifier},
		paramRedirectURI: {redirectURI},
	})
	if err != nil {
		return nil, err
	}
	c := &Credentials{
		ClientID:   ep.ClientID,
		AccessHost: ep.AccessHost,
	}
	c.applyToken(tok)
	return c, nil
}

func authorizeURL(ep *Endpoints, redirectURI, state, challenge, scope string) string {
	q := url.Values{
		"response_type":         {"code"},
		paramClientID:           {ep.ClientID},
		paramRedirectURI:        {redirectURI},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {pkceMethod},
	}
	if scope != "" {
		q.Set("scope", scope)
	}
	return strings.TrimSuffix(ep.AccessHost, "/") + "/oauth/v1/authorize?" + q.Encode()
}

type callbackResult struct {
	code string
	err  error
}

// callbackHandler receives the browser once the user has signed in. Only the
// first request that carries the right state is accepted; anything else is
// refused without ending the wait, so a stray request to the port cannot
// abort or hijack the login.
func callbackHandler(state string, results chan<- callbackResult) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "Invalid state. Please start the login again from the terminal.", http.StatusBadRequest)
			return
		}
		res := callbackResult{code: q.Get("code")}
		switch {
		case q.Get("error") != "":
			res.err = fmt.Errorf("sign-in failed: %s %s", q.Get("error"), q.Get("error_description"))
		case res.code == "":
			res.err = errors.New("sign-in failed: no authorization code returned")
		}
		select {
		case results <- res:
		default:
			// Already have an answer; ignore replays.
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if res.err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, "<p>Sign-in failed. You can close this window and check the terminal.</p>")
			return
		}
		_, _ = fmt.Fprint(w, "<p>Signed in to Invopop. You can close this window and return to the terminal.</p>")
	})
	return mux
}

// grant calls the access service's OAuth token endpoint.
func (ep *Endpoints) grant(ctx context.Context, form url.Values) (*oauthToken, error) {
	u := strings.TrimSuffix(ep.AccessHost, "/") + "/oauth/v1/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	tok := new(oauthToken)
	if err := ep.do(req, tok); err != nil {
		return nil, fmt.Errorf("oauth token request: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, errors.New("oauth token request: no access token returned")
	}
	return tok, nil
}

// do sends req and decodes a JSON response into out, turning error responses
// into errors that carry the server's description.
func (ep *Endpoints) do(req *http.Request, out any) error {
	res, err := ep.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close() //nolint:errcheck
	if res.StatusCode >= 300 {
		var e struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
			Message     string `json:"message"`
		}
		_ = json.NewDecoder(res.Body).Decode(&e)
		msg := strings.TrimSpace(strings.Join([]string{e.Error, e.Description, e.Message}, " "))
		if msg == "" {
			msg = http.StatusText(res.StatusCode)
		}
		return &StatusError{Code: res.StatusCode, Message: msg}
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// StatusError is an HTTP error response from Invopop.
type StatusError struct {
	Code    int
	Message string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}

func (c *Credentials) applyToken(tok *oauthToken) {
	c.AccessToken = tok.AccessToken
	if tok.RefreshToken != "" {
		c.RefreshToken = tok.RefreshToken
	}
	c.ExpiresAt = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	if tok.UserID != "" {
		c.UserID = tok.UserID
	}
	if tok.Info.Email != "" {
		c.Email = tok.Info.Email
	}
	// Any workspace token was minted from the previous user token.
	c.WorkspaceToken = ""
	c.WorkspaceTokenExpiresAt = time.Time{}
}
