// Package api holds the backend's HTTP handlers.
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/Manukesharwani09/photo-backup-manager/internal/crypto"
	"github.com/Manukesharwani09/photo-backup-manager/internal/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/store"
)

type Server struct {
	Store    *store.Store
	OAuth    *oauth2.Config
	TokenKey []byte
	UserID   int64

	mu     sync.Mutex
	states map[string]pendingAuth // OAuth state -> what the sign-in is for
}

type pendingAuth struct {
	role    string
	expires time.Time
}

func (s *Server) Routes() http.Handler {
	s.states = map[string]pendingAuth{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /oauth/start", s.oauthStart)
	mux.HandleFunc("GET /oauth/callback", s.oauthCallback)
	mux.HandleFunc("GET /accounts", s.listAccounts)
	return mux
}

// oauthStart sends the browser to Google. ?role=primary|overflow (default overflow).
func (s *Server) oauthStart(w http.ResponseWriter, r *http.Request) {
	role := r.URL.Query().Get("role")
	if role == "" {
		role = "overflow"
	}
	if role != "primary" && role != "overflow" {
		http.Error(w, "role must be primary or overflow", http.StatusBadRequest)
		return
	}
	b := make([]byte, 16)
	rand.Read(b)
	state := hex.EncodeToString(b)
	s.mu.Lock()
	s.states[state] = pendingAuth{role: role, expires: time.Now().Add(10 * time.Minute)}
	s.mu.Unlock()
	http.Redirect(w, r, google.AuthURL(s.OAuth, state), http.StatusFound)
}

func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		http.Error(w, "Google returned: "+e, http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	pending, ok := s.states[q.Get("state")]
	delete(s.states, q.Get("state"))
	s.mu.Unlock()
	if !ok || time.Now().After(pending.expires) {
		http.Error(w, "unknown or expired sign-in, start again at /oauth/start", http.StatusBadRequest)
		return
	}

	tok, err := s.OAuth.Exchange(ctx, q.Get("code"))
	if err != nil {
		httpError(w, "exchange code", err)
		return
	}
	if tok.RefreshToken == "" {
		http.Error(w, "Google did not return a refresh token; remove the app's access at myaccount.google.com/permissions and retry", http.StatusBadGateway)
		return
	}
	email, err := google.Email(ctx, s.OAuth.Client(ctx, tok))
	if err != nil {
		httpError(w, "read account email", err)
		return
	}
	enc, err := crypto.Seal(s.TokenKey, []byte(tok.RefreshToken))
	if err != nil {
		httpError(w, "encrypt token", err)
		return
	}
	id, err := s.Store.UpsertAccount(ctx, s.UserID, email, pending.role, enc)
	if err != nil {
		httpError(w, "save account", err)
		return
	}
	log.Printf("connected account %d (%s, %s)", id, email, pending.role)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("Connected " + email + " as " + pending.role + ". You can close this tab.\n"))
}

type accountView struct {
	ID            int64            `json:"id"`
	Email         string           `json:"email"`
	Role          string           `json:"role"`
	Active        bool             `json:"active"`
	Full          bool             `json:"full"`
	StorageLimit  int64            `json:"storage_limit_bytes"`
	StorageUsed   int64            `json:"storage_used_bytes"`
	StorageFree   int64            `json:"storage_free_bytes"` // -1 = unlimited
	StorageFreeGB string           `json:"storage_free"`
	StorageError  string           `json:"storage_error,omitempty"`
	ActivePeriods []map[string]any `json:"active_periods"`
}

func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accts, err := s.Store.ListAccounts(ctx, s.UserID)
	if err != nil {
		httpError(w, "list accounts", err)
		return
	}
	periods, err := s.Store.ListPeriods(ctx, s.UserID)
	if err != nil {
		httpError(w, "list periods", err)
		return
	}
	out := make([]accountView, 0, len(accts))
	for _, a := range accts {
		v := accountView{ID: a.ID, Email: a.Email, Role: a.Role, Active: a.Active, Full: a.FullFlag, ActivePeriods: []map[string]any{}}
		for _, p := range periods {
			if p.AccountID == a.ID {
				v.ActivePeriods = append(v.ActivePeriods, map[string]any{"started_at": p.StartedAt, "ended_at": p.EndedAt, "end_reason": p.EndReason})
			}
		}
		if q, err := s.quota(ctx, a); err != nil {
			v.StorageError = err.Error()
		} else {
			v.StorageLimit, v.StorageUsed, v.StorageFree = q.Limit, q.Usage, q.Remaining()
			v.StorageFreeGB = gb(q.Remaining())
		}
		out = append(out, v)
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}

// Client returns an authorised HTTP client for a stored account.
func (s *Server) Client(ctx context.Context, a store.Account) (*http.Client, error) {
	rt, err := crypto.Open(s.TokenKey, a.RefreshTokenEnc)
	if err != nil {
		return nil, err
	}
	return s.OAuth.Client(ctx, &oauth2.Token{RefreshToken: string(rt)}), nil
}

func (s *Server) quota(ctx context.Context, a store.Account) (google.StorageQuota, error) {
	c, err := s.Client(ctx, a)
	if err != nil {
		return google.StorageQuota{}, err
	}
	return google.Quota(ctx, c)
}

func gb(b int64) string {
	if b < 0 {
		return "unlimited"
	}
	return formatFloat(float64(b)/(1<<30)) + " GB"
}

func formatFloat(f float64) string {
	b, _ := json.Marshal(float64(int(f*100)) / 100)
	return string(b)
}

func httpError(w http.ResponseWriter, what string, err error) {
	log.Printf("%s: %v", what, err)
	http.Error(w, what+": "+err.Error(), http.StatusInternalServerError)
}
