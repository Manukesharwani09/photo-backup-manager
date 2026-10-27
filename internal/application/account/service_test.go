package account

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Manukesharwani09/photo-backup-manager/internal/enum"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/client/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/httperror"
)

// These tests cover the sign-in paths that never reach the database. Google's token endpoint is faked
// so a code exchange fails fast and offline.
func newTestService(t *testing.T) *accountService {
	t.Helper()
	token := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
	}))
	t.Cleanup(token.Close)
	cfg := google.OAuthConfig("cid", "secret", "http://localhost:8080/oauth/callback")
	cfg.Endpoint.TokenURL = token.URL
	return NewAccountService(nil, cfg, nil, nil, 1).(*accountService)
}

func wantCode(t *testing.T, err error, code int) {
	t.Helper()
	var be httperror.BSError
	if !errors.As(err, &be) || be.GetErrorCode() != code {
		t.Fatalf("want HTTP %d error, got %v", code, err)
	}
}

func TestAccountService_StartSignIn_RejectsBadRole(t *testing.T) {
	_, err := newTestService(t).StartSignIn("admin")
	wantCode(t, err, httperror.BAD_REQUEST)
}

func TestAccountService_StartSignIn_RemembersRoleWithStrongState(t *testing.T) {
	s := newTestService(t)
	for _, tc := range []struct {
		role string
		want enum.AccountRole
	}{{"primary", enum.ACCOUNT_ROLE_PRIMARY}, {"", enum.ACCOUNT_ROLE_OVERFLOW}} {
		target, err := s.StartSignIn(tc.role)
		if err != nil {
			t.Fatal(err)
		}
		loc, _ := url.Parse(target)
		if loc.Host != "accounts.google.com" {
			t.Fatalf("redirect to %s", loc)
		}
		state := loc.Query().Get("state")
		if len(state) != 32 {
			t.Fatalf("weak state %q", state)
		}
		if got := s.states[state].role; got != tc.want {
			t.Fatalf("role %q, want %q", got, tc.want)
		}
	}
}

func TestAccountService_CompleteSignIn_RejectsUnknownState(t *testing.T) {
	_, err := newTestService(t).CompleteSignIn(context.Background(), url.Values{"state": {"forged"}, "code": {"x"}})
	wantCode(t, err, httperror.BAD_REQUEST)
}

func TestAccountService_CompleteSignIn_RejectsExpiredStateAndConsumesIt(t *testing.T) {
	s := newTestService(t)
	s.states["old"] = pendingSignIn{role: enum.ACCOUNT_ROLE_PRIMARY, expires: time.Now().Add(-time.Minute)}
	_, err := s.CompleteSignIn(context.Background(), url.Values{"state": {"old"}, "code": {"x"}})
	wantCode(t, err, httperror.BAD_REQUEST)
	if _, still := s.states["old"]; still {
		t.Fatal("expired state was not removed")
	}
}

func TestAccountService_CompleteSignIn_SurfacesGoogleError(t *testing.T) {
	_, err := newTestService(t).CompleteSignIn(context.Background(), url.Values{"error": {"access_denied"}})
	wantCode(t, err, httperror.BAD_REQUEST)
	if err.Error() != "Google returned: access_denied" {
		t.Fatalf("message %q", err.Error())
	}
}

func TestAccountService_StartSignIn_DropsAbandonedStates(t *testing.T) {
	s := newTestService(t)
	s.states["stale"] = pendingSignIn{role: enum.ACCOUNT_ROLE_OVERFLOW, expires: time.Now().Add(-time.Hour)}
	if _, err := s.StartSignIn(""); err != nil {
		t.Fatal(err)
	}
	if _, still := s.states["stale"]; still {
		t.Fatal("stale state kept")
	}
}

func TestAccountService_CompleteSignIn_StateIsSingleUse(t *testing.T) {
	s := newTestService(t)
	s.states["once"] = pendingSignIn{role: enum.ACCOUNT_ROLE_PRIMARY, expires: time.Now().Add(time.Minute)}
	// The exchange fails (fake Google rejects the code): a 502, and the state is consumed regardless.
	_, err := s.CompleteSignIn(context.Background(), url.Values{"state": {"once"}, "code": {"bad"}})
	wantCode(t, err, httperror.BAD_GATEWAY)
	if _, still := s.states["once"]; still {
		t.Fatal("state reusable after callback")
	}
}
