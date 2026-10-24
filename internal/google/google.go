// Package google wraps the Google OAuth flow plus the Drive and Photos calls the backend needs.
package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
)

// Scopes requested for every connected account.
// openid + email identify which Google account was connected; the rest are the scopes
// declared on the consent screen (see docs/06-google-cloud-setup.md).
var Scopes = []string{
	"openid",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/photoslibrary.appendonly",
	"https://www.googleapis.com/auth/photoslibrary.readonly.appcreateddata",
	"https://www.googleapis.com/auth/photoslibrary.edit.appcreateddata",
	"https://www.googleapis.com/auth/drive.file",
}

// MissingScopes reports which required scopes are absent from Google's space-separated
// granted-scope string. The user can untick individual permissions on the consent screen.
func MissingScopes(granted string) []string {
	have := map[string]bool{}
	for _, s := range strings.Fields(granted) {
		have[s] = true
	}
	var missing []string
	for _, s := range Scopes {
		if s == "openid" {
			continue
		}
		if !have[s] {
			missing = append(missing, s)
		}
	}
	return missing
}

func OAuthConfig(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       Scopes,
		Endpoint:     googleoauth.Endpoint,
	}
}

// AuthURL forces the consent screen and offline access so Google always returns a refresh token,
// and shows the account chooser so a different account can be connected each time.
func AuthURL(cfg *oauth2.Config, state string) string {
	return cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent select_account"),
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
	)
}

// Email returns the email of the account the client is authorised for.
func Email(ctx context.Context, client *http.Client) (string, error) {
	var out struct {
		Email string `json:"email"`
	}
	if err := getJSON(ctx, client, "https://openidconnect.googleapis.com/v1/userinfo", &out); err != nil {
		return "", err
	}
	if out.Email == "" {
		return "", fmt.Errorf("userinfo returned no email")
	}
	return out.Email, nil
}

// DriveAPI is the Drive v3 base URL; tests point it at a fake server.
var DriveAPI = "https://www.googleapis.com/drive/v3"

// StorageQuota is the account's shared storage (Gmail + Drive + Photos), in bytes.
type StorageQuota struct {
	Limit int64 `json:"limit,string"` // 0 when the account has unlimited storage
	Usage int64 `json:"usage,string"`
}

func (q StorageQuota) Remaining() int64 {
	if q.Limit == 0 {
		return -1
	}
	return q.Limit - q.Usage
}

// Quota reads the remaining storage through Drive about.get (drive.file scope).
func Quota(ctx context.Context, client *http.Client) (StorageQuota, error) {
	var out struct {
		StorageQuota StorageQuota `json:"storageQuota"`
	}
	err := getJSON(ctx, client, DriveAPI+"/about?fields=storageQuota", &out)
	return out.StorageQuota, err
}

func getJSON(ctx context.Context, client *http.Client, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s: %s", url, resp.Status, body)
	}
	return json.Unmarshal(body, v)
}
