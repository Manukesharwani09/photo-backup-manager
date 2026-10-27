package appserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Manukesharwani09/photo-backup-manager/internal/application"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/client/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/httperror"
)

func serve(h HandlerFunc, header http.Header) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	for k, v := range header {
		req.Header.Set(k, v[0])
	}
	withRequestContext(handle(h)).ServeHTTP(rec, req)
	return rec
}

func TestHandle_MapsHTTPErrorToStatus(t *testing.T) {
	rec := serve(func(w http.ResponseWriter, r *http.Request) error {
		return httperror.NewHTTPError(httperror.CONFLICT, "already linked")
	}, nil)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "already linked") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestHandle_HidesInternalErrorDetails(t *testing.T) {
	rec := serve(func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("pq: password authentication failed for user secret")
	}, nil)
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("internal details leaked: %d %q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), rec.Header().Get(CORRELATION_HEADER)) {
		t.Fatal("500 body should carry the correlation id for log lookup")
	}
}

func TestWithRequestContext_CorrelationID(t *testing.T) {
	ok := func(w http.ResponseWriter, r *http.Request) error { return nil }
	if got := serve(ok, nil).Header().Get(CORRELATION_HEADER); len(got) != 16 {
		t.Fatalf("generated correlation id %q", got)
	}
	if got := serve(ok, http.Header{CORRELATION_HEADER: {"from-app-123"}}).Header().Get(CORRELATION_HEADER); got != "from-app-123" {
		t.Fatalf("caller's correlation id not propagated: %q", got)
	}
}

func TestNewRouter_OldUnversionedPathsAreGone(t *testing.T) {
	h := NewRouter(testServicesNoDB())
	for _, p := range []string{"/sync", "/dashboard", "/accounts"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404 (API lives under /v1)", p, rec.Code)
		}
	}
}

// testServicesNoDB builds services without a database, enough for routing tests that never call them.
func testServicesNoDB() application.Services {
	return application.NewServices(application.Deps{OAuth: google.OAuthConfig("c", "s", "http://localhost/cb")})
}
