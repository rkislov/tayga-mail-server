package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/emersion/go-sasl"
)

// XOAuth2 is the SASL mechanism name used by many mail clients (Gmail-style).
const XOAuth2 = "XOAUTH2"

// XOAuth2Authenticator validates username + bearer token.
type XOAuth2Authenticator func(username, token string) error

type xoauth2Server struct {
	authenticate XOAuth2Authenticator
	done         bool
	failErr      error
}

// NewXOAuth2Server implements the XOAUTH2 SASL mechanism.
func NewXOAuth2Server(auth XOAuth2Authenticator) sasl.Server {
	return &xoauth2Server{authenticate: auth}
}

func (a *xoauth2Server) fail(status string) ([]byte, bool, error) {
	blob, err := json.Marshal(map[string]string{
		"status":  status,
		"schemes": "Bearer",
	})
	if err != nil {
		return nil, false, err
	}
	a.failErr = errors.New("sasl xoauth2: " + status)
	return blob, false, nil
}

func (a *xoauth2Server) Next(response []byte) (challenge []byte, done bool, err error) {
	if a.failErr != nil {
		return nil, true, a.failErr
	}
	if a.done {
		return nil, false, sasl.ErrUnexpectedClientResponse
	}
	if response == nil {
		return []byte{}, false, nil
	}
	a.done = true

	// Format: user=<email>\x01auth=Bearer <token>\x01\x01
	parts := bytes.Split(response, []byte{0x01})
	var username, token string
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		switch {
		case bytes.HasPrefix(p, []byte("user=")):
			username = string(bytes.TrimPrefix(p, []byte("user=")))
		case bytes.HasPrefix(bytes.ToLower(p), []byte("auth=bearer ")):
			// case-insensitive Bearer
			idx := bytes.IndexByte(p, ' ')
			if idx >= 0 {
				token = string(p[idx+1:])
			}
		}
	}
	if username == "" || token == "" {
		return a.fail("400")
	}
	if err := a.authenticate(username, token); err != nil {
		return a.fail("401")
	}
	return nil, true, nil
}

// ParseBearerToken extracts the token from "Bearer <token>" or raw token.
func ParseBearerToken(authz string) string {
	authz = strings.TrimSpace(authz)
	const prefix = "bearer "
	if strings.HasPrefix(strings.ToLower(authz), prefix) {
		return strings.TrimSpace(authz[len(prefix):])
	}
	return authz
}
