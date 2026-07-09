package authadapter

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-common-libs/pkg/goHttpEcho"
)

func TestDevTokenVerifier(t *testing.T) {
	verifier, err := NewDevTokenVerifier("secret", AuthenticatedUser{AppUserID: 42, Scopes: []string{"goeland:read"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.VerifyBearerToken(context.Background(), "wrong"); err == nil {
		t.Fatal("wrong token was accepted")
	}
	user, err := verifier.VerifyBearerToken(context.Background(), "secret")
	if err != nil || user.AppUserID != 42 || !user.HasScope("goeland:read") {
		t.Fatalf("user = %#v, error = %v", user, err)
	}
}

func TestJWTVerifierMapsAuthClaims(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	checker := goHttpEcho.NewJwtChecker("test-secret", "test-issuer", "goeland", "jwt", 5, logger)
	verifier, err := NewJWTVerifier(checker, []string{"goeland:read", "goeland:write"})
	if err != nil {
		t.Fatal(err)
	}
	token, err := checker.GetTokenFromUserInfo(&goHttpEcho.UserInfo{
		UserId: 77, Name: "Admin", Email: "admin@example.test", IsAdmin: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	user, err := verifier.VerifyBearerToken(context.Background(), token.String())
	if err != nil {
		t.Fatal(err)
	}
	if user.AppUserID != 77 || !user.HasScope("goeland:write") || !user.HasScope("goeland:admin") {
		t.Fatalf("unexpected mapped user: %#v", user)
	}
}
