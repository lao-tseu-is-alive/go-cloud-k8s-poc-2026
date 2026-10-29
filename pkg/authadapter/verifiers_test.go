package authadapter

import (
	"context"
	"testing"
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
