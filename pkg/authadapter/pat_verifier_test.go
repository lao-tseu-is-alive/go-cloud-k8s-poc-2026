package authadapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// newIntrospectionServer fakes the auth service introspection REST route.
func newIntrospectionServer(t *testing.T, active map[string]introspectResponse, calls *atomic.Int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/goapi/v1/auth/introspect" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		calls.Add(1)
		var req introspectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp, ok := active[req.Token]
		if !ok {
			resp = introspectResponse{Active: false}
		}
		// protojson renders int64 as a JSON string; mimic that.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"active": resp.Active,
			"userId": "4242",
			"email":  resp.Email,
			"name":   resp.Name,
			"scopes": resp.Scopes,
		})
	}))
}

func TestPatVerifier(t *testing.T) {
	var calls atomic.Int64
	server := newIntrospectionServer(t, map[string]introspectResponse{
		"pat_good": {Active: true, Email: "jane@example.com", Name: "Jane", Scopes: []string{"goeland:read", "goeland:write"}},
	}, &calls)
	defer server.Close()

	verifier, err := NewPatVerifier(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	t.Run("active token is accepted", func(t *testing.T) {
		user, err := verifier.VerifyBearerToken(ctx, "pat_good")
		if err != nil {
			t.Fatal(err)
		}
		if user.AppUserID != 4242 || user.Email != "jane@example.com" || len(user.Scopes) != 2 {
			t.Fatalf("user = %+v", user)
		}
	})

	t.Run("positive result is cached", func(t *testing.T) {
		before := calls.Load()
		if _, err := verifier.VerifyBearerToken(ctx, "pat_good"); err != nil {
			t.Fatal(err)
		}
		if calls.Load() != before {
			t.Fatalf("expected cached verification, got %d extra introspection calls", calls.Load()-before)
		}
	})

	t.Run("inactive token is rejected and not cached", func(t *testing.T) {
		before := calls.Load()
		if _, err := verifier.VerifyBearerToken(ctx, "pat_revoked"); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("error = %v, want ErrInvalidToken", err)
		}
		if _, err := verifier.VerifyBearerToken(ctx, "pat_revoked"); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("error = %v, want ErrInvalidToken", err)
		}
		if calls.Load() != before+2 {
			t.Fatalf("negative results must not be cached: calls=%d want %d", calls.Load(), before+2)
		}
	})

	t.Run("non-pat token is rejected without any call", func(t *testing.T) {
		before := calls.Load()
		if _, err := verifier.VerifyBearerToken(ctx, "eyJhbGci.x.y"); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("error = %v, want ErrInvalidToken", err)
		}
		if calls.Load() != before {
			t.Fatal("non-pat tokens must not hit introspection")
		}
	})
}

func TestPatVerifierServerDown(t *testing.T) {
	verifier, err := NewPatVerifier("http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.VerifyBearerToken(context.Background(), "pat_whatever"); err == nil {
		t.Fatal("expected an error when the auth service is unreachable")
	}
}

type stubVerifier struct {
	user *AuthenticatedUser
	err  error
	seen string
}

func (s *stubVerifier) VerifyBearerToken(_ context.Context, token string) (*AuthenticatedUser, error) {
	s.seen = token
	return s.user, s.err
}

func TestCompositeVerifierRoutesByPrefix(t *testing.T) {
	jwtStub := &stubVerifier{user: &AuthenticatedUser{AppUserID: 1}}
	patStub := &stubVerifier{user: &AuthenticatedUser{AppUserID: 2}}
	composite, err := NewCompositeVerifier(jwtStub, patStub)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	user, err := composite.VerifyBearerToken(ctx, "pat_abc")
	if err != nil || user.AppUserID != 2 || patStub.seen != "pat_abc" {
		t.Fatalf("pat token was not routed to the pat verifier: user=%+v err=%v", user, err)
	}

	user, err = composite.VerifyBearerToken(ctx, "eyJhbGci.x.y")
	if err != nil || user.AppUserID != 1 || jwtStub.seen != "eyJhbGci.x.y" {
		t.Fatalf("jwt token was not routed to the jwt verifier: user=%+v err=%v", user, err)
	}
}

func TestPatCacheStaysBounded(t *testing.T) {
	v := &PatVerifier{cacheTTL: time.Minute, maxEntries: 3, cache: make(map[[32]byte]patCacheEntry)}
	key := func(i byte) [32]byte { return [32]byte{i} }
	past, future := time.Now().Add(-time.Second), time.Now().Add(time.Minute)

	v.store(key(1), patCacheEntry{expiresAt: past})
	v.store(key(2), patCacheEntry{expiresAt: future})
	v.store(key(3), patCacheEntry{expiresAt: future})
	v.store(key(4), patCacheEntry{expiresAt: future})
	if _, stale := v.cache[key(1)]; stale || len(v.cache) != 3 {
		t.Fatalf("at the limit the expired entry is swept first: %d entries, stale kept=%v", len(v.cache), stale)
	}

	v.store(key(5), patCacheEntry{expiresAt: future})
	if len(v.cache) != 1 {
		t.Fatalf("a full cache of live entries is emptied rather than grown: %d entries", len(v.cache))
	}
	if _, ok := v.cache[key(5)]; !ok {
		t.Fatal("the new entry is kept")
	}
}
