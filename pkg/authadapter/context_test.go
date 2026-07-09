package authadapter

import (
	"context"
	"errors"
	"testing"
)

func TestAuthenticatedUserContext(t *testing.T) {
	if _, err := RequireUser(context.Background()); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("error = %v, want ErrUnauthenticated", err)
	}
	user := &AuthenticatedUser{AppUserID: 42, Scopes: []string{"goeland:read"}}
	got, err := RequireUser(ContextWithUser(context.Background(), user))
	if err != nil || got.AppUserID != 42 || !got.HasScope("goeland:read") {
		t.Fatalf("got user %#v, error %v", got, err)
	}
}
