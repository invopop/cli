package auth

import (
	"net/http"
)

// Transport returns an http.RoundTripper that sets a workspace token on
// every request, renewing it as it nears expiry, so a long-running command
// or the terminal UI keeps working past the life of a single token.
// Requests go on through base, or http.DefaultTransport when nil.
func (s *Session) Transport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &renewingTransport{session: s, base: base}
}

type renewingTransport struct {
	session *Session
	base    http.RoundTripper
}

func (t *renewingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tok, err := t.session.WorkspaceToken(req.Context())
	if err != nil {
		return nil, err
	}
	// A RoundTripper must not change the request it was given.
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+tok)
	return t.base.RoundTrip(req)
}
