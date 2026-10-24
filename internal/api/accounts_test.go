package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/client/google"
)

// These tests cover the paths that never reach Google or the DB.
func newServer() (*Server, http.Handler) {
	s := &Server{OAuth: google.OAuthConfig("cid", "secret", "http://localhost:8080/oauth/callback")}
	return s, s.Routes()
}

func do(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestOAuthStartRejectsBadRole(t *testing.T) {
	_, h := newServer()
	if rec := do(h, "/oauth/start?role=admin"); rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestOAuthStartRedirectsAndRemembersRole(t *testing.T) {
	s, h := newServer()
	for _, tc := range []struct{ query, role string }{{"?role=primary", "primary"}, {"", "overflow"}} {
		rec := do(h, "/oauth/start"+tc.query)
		if rec.Code != http.StatusFound {
			t.Fatalf("got %d", rec.Code)
		}
		loc, _ := url.Parse(rec.Header().Get("Location"))
		if loc.Host != "accounts.google.com" {
			t.Fatalf("redirected to %s", loc)
		}
		state := loc.Query().Get("state")
		if len(state) != 32 {
			t.Fatalf("weak state %q", state)
		}
		if got := s.states[state].role; got != tc.role {
			t.Fatalf("role %q, want %q", got, tc.role)
		}
	}
}

func TestOAuthCallbackRejectsUnknownState(t *testing.T) {
	_, h := newServer()
	rec := do(h, "/oauth/callback?state=forged&code=x")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unknown or expired") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestOAuthCallbackRejectsExpiredStateAndConsumesIt(t *testing.T) {
	s, h := newServer()
	s.states["old"] = pendingAuth{role: "primary", expires: time.Now().Add(-time.Minute)}
	if rec := do(h, "/oauth/callback?state=old&code=x"); rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d", rec.Code)
	}
	if _, still := s.states["old"]; still {
		t.Fatal("expired state was not removed")
	}
}

func TestOAuthCallbackSurfacesGoogleError(t *testing.T) {
	_, h := newServer()
	rec := do(h, "/oauth/callback?error=access_denied")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "access_denied") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestStartDropsAbandonedStates(t *testing.T) {
	s, h := newServer()
	s.states["stale"] = pendingAuth{role: "overflow", expires: time.Now().Add(-time.Hour)}
	do(h, "/oauth/start")
	if _, still := s.states["stale"]; still {
		t.Fatal("stale state kept")
	}
}

func TestStateIsSingleUse(t *testing.T) {
	s, h := newServer()
	s.states["once"] = pendingAuth{role: "primary", expires: time.Now().Add(time.Minute)}
	// The first use proceeds to the code exchange (which fails offline); either way the state is consumed.
	do(h, "/oauth/callback?state=once&code=bad")
	if _, still := s.states["once"]; still {
		t.Fatal("state reusable after callback")
	}
}
