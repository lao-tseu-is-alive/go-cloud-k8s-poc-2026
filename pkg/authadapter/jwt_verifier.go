package authadapter

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/cristalhq/jwt/v5"
)

// The JWT verification below reads the tokens signed by go-cloud-k8s-auth. It
// replaces the checker of go-cloud-k8s-common-libs (pkg/goHttpEcho, MIT, same
// author), which brought the Echo framework into the binary for one function;
// it also checks the issuer, requires an expiry and never logs token content.

// JWTConfig is the shared HS512 secret and the issuer of the auth service's
// tokens (JWT_SECRET, JWT_ISSUER_ID: the same values as go-cloud-k8s-auth).
type JWTConfig struct {
	// Secret signs and verifies the tokens; a secret that must never be logged.
	Secret string
	// Issuer is the iss claim every accepted token must carry.
	Issuer string
}

// jwtClaims are the claims go-cloud-k8s-auth writes: the registered claims
// and the user under the "User" key.
type jwtClaims struct {
	jwt.RegisteredClaims
	// User is the authenticated user; a token without one is refused.
	User *jwtUser `json:"User"`
}

// jwtUser is the user part of the claims; other fields are ignored.
type jwtUser struct {
	// UserID is the application user id; it must be positive.
	UserID int64 `json:"user_id"`
	// Name is the display name.
	Name string `json:"name"`
	// Email is the user's e-mail address.
	Email string `json:"email"`
	// IsAdmin asks for the goeland:admin scope (re-decided by the recording verifier).
	IsAdmin bool `json:"is_admin"`
}

// JWTVerifier verifies the HS512 tokens of go-cloud-k8s-auth and maps them to
// the service's deliberately small authentication contract.
type JWTVerifier struct {
	alg           *jwt.HSAlg
	issuer        string
	defaultScopes []string
	now           func() time.Time
}

// NewJWTVerifier builds a verifier for the tokens signed with cfg.Secret by
// cfg.Issuer; defaultScopes are given to every verified user.
func NewJWTVerifier(cfg JWTConfig, defaultScopes []string) (*JWTVerifier, error) {
	if cfg.Secret == "" || cfg.Issuer == "" {
		return nil, errors.New("JWT secret and issuer are required")
	}
	alg, err := jwt.NewVerifierHS(jwt.HS512, []byte(cfg.Secret))
	if err != nil {
		return nil, err
	}
	return &JWTVerifier{alg: alg, issuer: cfg.Issuer, defaultScopes: slices.Clone(defaultScopes), now: time.Now}, nil
}

// VerifyBearerToken accepts an HS512 token with a valid signature, of the
// configured issuer, with an expiry, valid now and naming a user; an admin
// user also gets the "goeland:admin" scope (filtered again by the recording
// verifier). Every refusal is ErrInvalidToken, without detail.
func (v *JWTVerifier) VerifyBearerToken(_ context.Context, token string) (*AuthenticatedUser, error) {
	parsed, err := jwt.Parse([]byte(token), v.alg)
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims jwtClaims
	if err := parsed.DecodeClaims(&claims); err != nil {
		return nil, ErrInvalidToken
	}
	if claims.ExpiresAt == nil || !claims.IsValidAt(v.now()) || !claims.IsIssuer(v.issuer) ||
		claims.User == nil || claims.User.UserID <= 0 {
		return nil, ErrInvalidToken
	}
	scopes := slices.Clone(v.defaultScopes)
	if claims.User.IsAdmin && !slices.Contains(scopes, "goeland:admin") {
		scopes = append(scopes, "goeland:admin")
	}
	return &AuthenticatedUser{
		AppUserID:   claims.User.UserID,
		Email:       claims.User.Email,
		DisplayName: claims.User.Name,
		Scopes:      scopes,
	}, nil
}
