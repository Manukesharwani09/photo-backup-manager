package google

import (
	"net/url"
	"strings"
	"testing"
)

func TestMissingScopes(t *testing.T) {
	all := strings.Join(Scopes, " ")
	if m := MissingScopes(all); len(m) != 0 {
		t.Fatalf("all granted but missing %v", m)
	}
	// openid is not reported even if absent; Google sometimes omits it from the scope string.
	if m := MissingScopes(strings.Replace(all, "openid", "", 1)); len(m) != 0 {
		t.Fatalf("openid should not be required, got %v", m)
	}
	// User unticked the Drive checkbox.
	partial := strings.Replace(all, "https://www.googleapis.com/auth/drive.file", "", 1)
	m := MissingScopes(partial)
	if len(m) != 1 || m[0] != "https://www.googleapis.com/auth/drive.file" {
		t.Fatalf("got %v", m)
	}
	if m := MissingScopes(""); len(m) != len(Scopes)-1 {
		t.Fatalf("empty grant: got %v", m)
	}
}

func TestAuthURL(t *testing.T) {
	cfg := OAuthConfig("cid", "secret", "http://localhost:8080/oauth/callback")
	u, err := url.Parse(AuthURL(cfg, "st4te"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	checks := map[string]string{
		"access_type":  "offline", // needed for a refresh token
		"prompt":       "consent select_account",
		"state":        "st4te",
		"client_id":    "cid",
		"redirect_uri": "http://localhost:8080/oauth/callback",
	}
	for k, want := range checks {
		if q.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, q.Get(k), want)
		}
	}
	if q.Get("scope") != strings.Join(Scopes, " ") {
		t.Errorf("scope = %q", q.Get("scope"))
	}
}

func TestQuotaRemaining(t *testing.T) {
	if r := (StorageQuota{Limit: 100, Usage: 30}).Remaining(); r != 70 {
		t.Fatalf("got %d", r)
	}
	if r := (StorageQuota{Limit: 0, Usage: 30}).Remaining(); r != -1 {
		t.Fatalf("unlimited: got %d", r)
	}
}
