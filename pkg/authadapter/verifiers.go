package authadapter

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"slices"
)

// ErrInvalidToken is returned by any TokenVerifier when the supplied token cannot be validated.
var ErrInvalidToken = errors.New("invalid bearer token")

// DevTokenVerifier is an explicit local-development verifier. It must only be
// selected when GOELAND_AUTH_MODE=dev.
type DevTokenVerifier struct {
	token string
	user  AuthenticatedUser
}

// NewDevTokenVerifier creates a verifier that accepts exactly one static token. Must not be used outside GOELAND_AUTH_MODE=dev.
func NewDevTokenVerifier(token string, user AuthenticatedUser) (*DevTokenVerifier, error) {
	if token == "" {
		return nil, errors.New("development token is required")
	}
	if user.AppUserID <= 0 {
		return nil, fmt.Errorf("development user ID must be positive")
	}
	user.Scopes = slices.Clone(user.Scopes)
	return &DevTokenVerifier{token: token, user: user}, nil
}

// VerifyBearerToken accepts the token only when it matches the configured dev secret via a constant-time comparison.
func (v *DevTokenVerifier) VerifyBearerToken(_ context.Context, token string) (*AuthenticatedUser, error) {
	if subtle.ConstantTimeCompare([]byte(token), []byte(v.token)) != 1 {
		return nil, ErrInvalidToken
	}
	user := v.user
	user.Scopes = slices.Clone(v.user.Scopes)
	return &user, nil
}
