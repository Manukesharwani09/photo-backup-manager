package account

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/Manukesharwani09/photo-backup-manager/internal/entity"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/client/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/crypto"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
)

// Access turns a stored account (encrypted refresh token) into Google API access.
// Every feature that calls Google on a user's behalf goes through it.
type Access struct {
	repo     *repository.Store
	oauth    *oauth2.Config
	tokenKey []byte
	userID   int64
}

func NewAccess(repo *repository.Store, oauth *oauth2.Config, tokenKey []byte, userID int64) *Access {
	return &Access{repo: repo, oauth: oauth, tokenKey: tokenKey, userID: userID}
}

// AccessToken is a short-lived token handed to the phone to upload or download bytes itself.
type AccessToken struct {
	AccessToken string    `json:"access_token"`
	Expiry      time.Time `json:"expiry"`
}

// Client returns an HTTP client authorised as the account.
func (a *Access) Client(ctx context.Context, acct entity.Account) (*http.Client, error) {
	rt, err := crypto.Open(a.tokenKey, acct.RefreshTokenEnc)
	if err != nil {
		return nil, err
	}
	return a.oauth.Client(ctx, &oauth2.Token{RefreshToken: string(rt)}), nil
}

// ClientFor loads one of the owner's accounts and returns it with an authorised client.
func (a *Access) ClientFor(ctx context.Context, accountID int64) (entity.Account, *http.Client, error) {
	acct, err := a.repo.AccountOwned(ctx, a.userID, accountID)
	if err != nil {
		return acct, nil, err
	}
	c, err := a.Client(ctx, acct)
	return acct, c, err
}

// AccessToken mints a short-lived access token for one of the owner's accounts.
func (a *Access) AccessToken(ctx context.Context, accountID int64) (AccessToken, error) {
	acct, err := a.repo.AccountOwned(ctx, a.userID, accountID)
	if err != nil {
		return AccessToken{}, err
	}
	rt, err := crypto.Open(a.tokenKey, acct.RefreshTokenEnc)
	if err != nil {
		return AccessToken{}, err
	}
	tok, err := a.oauth.TokenSource(ctx, &oauth2.Token{RefreshToken: string(rt)}).Token()
	if err != nil {
		return AccessToken{}, err
	}
	return AccessToken{AccessToken: tok.AccessToken, Expiry: tok.Expiry}, nil
}

// Quota reads the account's remaining Google storage.
func (a *Access) Quota(ctx context.Context, acct entity.Account) (google.StorageQuota, error) {
	c, err := a.Client(ctx, acct)
	if err != nil {
		return google.StorageQuota{}, err
	}
	return google.Quota(ctx, c)
}
