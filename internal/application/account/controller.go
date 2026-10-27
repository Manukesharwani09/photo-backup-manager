package account

import (
	"net/http"

	"github.com/Manukesharwani09/photo-backup-manager/internal/common"
)

type AccountController struct {
	service AccountService
}

func NewAccountController(service AccountService) *AccountController {
	return &AccountController{service: service}
}

// OAuthStart: GET /oauth/start?role=primary|overflow (default overflow) → redirect to Google.
func (c *AccountController) OAuthStart(w http.ResponseWriter, r *http.Request) error {
	target, err := c.service.StartSignIn(r.URL.Query().Get("role"))
	if err != nil {
		return err
	}
	http.Redirect(w, r, target, http.StatusFound)
	return nil
}

// OAuthCallback: GET /oauth/callback (the redirect URI registered with Google; not versioned).
func (c *AccountController) OAuthCallback(w http.ResponseWriter, r *http.Request) error {
	res, err := c.service.CompleteSignIn(r.Context(), r.URL.Query())
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("Connected " + res.Email + " as " + res.Role + ". You can close this tab.\n"))
	return nil
}

// List: GET /v1/accounts.
func (c *AccountController) List(w http.ResponseWriter, r *http.Request) error {
	out, err := c.service.List(r.Context())
	if err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusOK, out)
	return nil
}

// MarkFull: POST /v1/accounts/{id}/full.
func (c *AccountController) MarkFull(w http.ResponseWriter, r *http.Request) error {
	id, err := common.PathInt64(r, "id")
	if err != nil {
		return err
	}
	if err := c.service.MarkFull(r.Context(), id); err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusOK, MarkFullResponse{AccountID: id, Full: true})
	return nil
}

// MarkAvailable: POST /v1/accounts/{id}/available.
func (c *AccountController) MarkAvailable(w http.ResponseWriter, r *http.Request) error {
	id, err := common.PathInt64(r, "id")
	if err != nil {
		return err
	}
	if err := c.service.MarkAvailable(r.Context(), id); err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusOK, AccountStateResponse{AccountID: id, State: "available"})
	return nil
}

// MakeCurrent: POST /v1/accounts/{id}/current.
func (c *AccountController) MakeCurrent(w http.ResponseWriter, r *http.Request) error {
	id, err := common.PathInt64(r, "id")
	if err != nil {
		return err
	}
	if err := c.service.MakeCurrent(r.Context(), id); err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusOK, AccountStateResponse{AccountID: id, State: "current"})
	return nil
}
