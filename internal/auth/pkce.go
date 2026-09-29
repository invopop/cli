package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// pkce holds a PKCE verifier and the S256 challenge derived from it. The CLI
// is a public client with no secret, so PKCE is what proves that whoever
// redeems the authorization code is whoever asked for it. Only S256 is used:
// the server also accepts "plain", which would hand the verifier to anyone
// who sees the authorize URL.
type pkce struct {
	Verifier  string
	Challenge string
}

const pkceMethod = "S256"

func newPKCE() (*pkce, error) {
	v, err := randomString(32)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(v))
	return &pkce{
		Verifier:  v,
		Challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
	}, nil
}

// randomString returns n random bytes, base64url encoded without padding.
func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
