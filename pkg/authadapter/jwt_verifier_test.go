package authadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cristalhq/jwt/v5"
)

const (
	testJWTSecret = "test-secret-at-least-16"
	testJWTIssuer = "test-issuer-at-least-16"
)

// signToken signs claims the way go-cloud-k8s-auth does (HS512 by default).
func signToken(t *testing.T, alg jwt.Algorithm, secret string, claims any) string {
	t.Helper()
	signer, err := jwt.NewSignerHS(alg, []byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	token, err := jwt.NewBuilder(signer).Build(claims)
	if err != nil {
		t.Fatal(err)
	}
	return token.String()
}

// authClaims are valid claims of the auth service for user, issued now.
func authClaims(user *jwtUser) jwtClaims {
	now := time.Now()
	return jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID: "token-id", Issuer: testJWTIssuer,
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
			NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)),
		},
		User: user,
	}
}

func newTestJWTVerifier(t *testing.T) *JWTVerifier {
	t.Helper()
	v, err := NewJWTVerifier(JWTConfig{Secret: testJWTSecret, Issuer: testJWTIssuer}, []string{"goeland:read", "goeland:write"})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestJWTVerifierMapsAuthClaims(t *testing.T) {
	v := newTestJWTVerifier(t)
	token := signToken(t, jwt.HS512, testJWTSecret, authClaims(&jwtUser{UserID: 77, Name: "Admin", Email: "admin@example.test", IsAdmin: true}))
	user, err := v.VerifyBearerToken(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if user.AppUserID != 77 || user.DisplayName != "Admin" || user.Email != "admin@example.test" ||
		!user.HasScope("goeland:write") || !user.HasScope("goeland:admin") {
		t.Fatalf("unexpected mapped user: %#v", user)
	}
	plain := signToken(t, jwt.HS512, testJWTSecret, authClaims(&jwtUser{UserID: 78}))
	if user, err := v.VerifyBearerToken(context.Background(), plain); err != nil || user.HasScope("goeland:admin") {
		t.Fatalf("a non-admin user gets only the default scopes: %#v (%v)", user, err)
	}
}

// TestJWTVerifierAcceptsTheAuthServiceFormat decodes a payload written as
// go-cloud-k8s-auth writes it (the user under "User", unused fields included).
func TestJWTVerifierAcceptsTheAuthServiceFormat(t *testing.T) {
	v := newTestJWTVerifier(t)
	now := time.Now().Unix()
	claims := map[string]any{
		"jti": "x", "iss": testJWTIssuer, "exp": now + 600, "iat": now - 5, "nbf": now - 5,
		"User": map[string]any{"user_id": 12, "external_id": 0, "name": "Jane", "email": "jane@example.test", "login": "jane", "is_admin": false, "groups": []int{1}},
	}
	user, err := v.VerifyBearerToken(context.Background(), signToken(t, jwt.HS512, testJWTSecret, claims))
	if err != nil || user.AppUserID != 12 || user.DisplayName != "Jane" {
		t.Fatalf("auth service token: %#v (%v)", user, err)
	}
}

func TestJWTVerifierRefusals(t *testing.T) {
	v := newTestJWTVerifier(t)
	user := &jwtUser{UserID: 5}
	expired := authClaims(user)
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
	notYet := authClaims(user)
	notYet.NotBefore = jwt.NewNumericDate(time.Now().Add(time.Hour))
	noExpiry := authClaims(user)
	noExpiry.ExpiresAt = nil
	otherIssuer := authClaims(user)
	otherIssuer.Issuer = "another-issuer-of-tokens"
	cases := map[string]string{
		"wrong secret":    signToken(t, jwt.HS512, "another-secret-value", authClaims(user)),
		"other algorithm": signToken(t, jwt.HS256, testJWTSecret, authClaims(user)),
		"expired":         signToken(t, jwt.HS512, testJWTSecret, expired),
		"not yet valid":   signToken(t, jwt.HS512, testJWTSecret, notYet),
		"no expiry":       signToken(t, jwt.HS512, testJWTSecret, noExpiry),
		"other issuer":    signToken(t, jwt.HS512, testJWTSecret, otherIssuer),
		"no user":         signToken(t, jwt.HS512, testJWTSecret, authClaims(nil)),
		"user id zero":    signToken(t, jwt.HS512, testJWTSecret, authClaims(&jwtUser{Name: "x"})),
		"not a token":     "not-a-jwt",
		"empty":           "",
		"truncated":       signToken(t, jwt.HS512, testJWTSecret, authClaims(user))[:40],
	}
	for name, token := range cases {
		if got, err := v.VerifyBearerToken(context.Background(), token); !errors.Is(err, ErrInvalidToken) || got != nil {
			t.Errorf("%s: want ErrInvalidToken, got %#v (%v)", name, got, err)
		}
	}
}

func TestNewJWTVerifierRequiresSecretAndIssuer(t *testing.T) {
	for _, cfg := range []JWTConfig{{Issuer: testJWTIssuer}, {Secret: testJWTSecret}} {
		if _, err := NewJWTVerifier(cfg, nil); err == nil {
			t.Fatal("an incomplete JWT config was accepted")
		}
	}
}
