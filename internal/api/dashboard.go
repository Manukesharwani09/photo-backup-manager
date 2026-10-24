package api

import (
	"net/http"
	"time"

	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
	"github.com/Manukesharwani09/photo-backup-manager/internal/planner"
)

type dashboardAccount struct {
	ID            int64            `json:"id"`
	Email         string           `json:"email"`
	Role          string           `json:"role"`
	Active        bool             `json:"active"`
	Full          bool             `json:"full"`
	Current       bool             `json:"current"` // receiving new backups now
	StorageFree   string           `json:"storage_free,omitempty"`
	StorageError  string           `json:"storage_error,omitempty"`
	ActivePeriods []map[string]any `json:"active_periods"`
	ToDeleteAlbum string           `json:"to_delete_album_url,omitempty"`
	*repository.AccountStats
}

type alert struct {
	Kind    string `json:"kind"` // account_full | no_account_available | to_delete_waiting | account_unreachable
	Message string `json:"message"`
}

// dashboard is the Home/Accounts screen data: per account what it holds and when it was used,
// counts of phone files by state, and alerts that need the user.
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accts, err := s.Store.ListAccounts(ctx, s.UserID)
	if err != nil {
		httpError(w, "accounts", err)
		return
	}
	candidates, err := s.Store.PlannerAccounts(ctx, s.UserID)
	if err != nil {
		httpError(w, "planner accounts", err)
		return
	}
	periods, err := s.Store.ListPeriods(ctx, s.UserID)
	if err != nil {
		httpError(w, "periods", err)
		return
	}
	stats, err := s.Store.AccountStats(ctx, s.UserID)
	if err != nil {
		httpError(w, "stats", err)
		return
	}
	counts, err := s.Store.StateCounts(ctx, s.UserID)
	if err != nil {
		httpError(w, "counts", err)
		return
	}
	current := map[int64]bool{}
	for _, p := range candidates {
		current[p.ID] = p.IsCurrent
	}

	out := struct {
		Accounts []dashboardAccount `json:"accounts"`
		Phone    map[string]int     `json:"phone_files"` // by state, files still on a phone
		Alerts   []alert            `json:"alerts"`
	}{Accounts: []dashboardAccount{}, Phone: counts, Alerts: []alert{}}

	for _, a := range accts {
		d := dashboardAccount{ID: a.ID, Email: a.Email, Role: string(a.Role), Active: a.Active, Full: a.FullFlag, Current: current[a.ID],
			ActivePeriods: []map[string]any{}, AccountStats: stats[a.ID]}
		if d.AccountStats == nil {
			d.AccountStats = &repository.AccountStats{Folders: []string{}}
		}
		for _, p := range periods {
			if p.AccountID == a.ID {
				d.ActivePeriods = append(d.ActivePeriods, map[string]any{"started_at": p.StartedAt, "ended_at": p.EndedAt, "end_reason": p.EndReason})
			}
		}
		if q, err := s.quota(ctx, a); err != nil {
			d.StorageError = err.Error()
			out.Alerts = append(out.Alerts, alert{"account_unreachable", a.Email + ": can't reach Google; reconnect the account if this persists"})
		} else {
			d.StorageFree = gb(q.Remaining())
		}
		if a.FullFlag {
			out.Alerts = append(out.Alerts, alert{"account_full", a.Email + " is full; new photos go to the next account"})
		}
		if d.PendingDelete > 0 {
			d.ToDeleteAlbum, _ = s.Store.AlbumLink(ctx, a.ID, repository.ToDeleteFolder)
			out.Alerts = append(out.Alerts, alert{"to_delete_waiting", a.Email + ": photos are waiting in the 🗑 To delete album"})
		}
		out.Accounts = append(out.Accounts, d)
	}
	if _, _, why := planner.ChooseAccount(candidates, nil); why != "" {
		out.Alerts = append(out.Alerts, alert{"no_account_available", "No account can take new photos: connect another account or free up space"})
	}
	writeJSON(w, http.StatusOK, out)
}

// photosWhere answers "which account has my photos from <period>?". Query: from=YYYY-MM-DD&to=YYYY-MM-DD
// (to inclusive) or month=YYYY-MM.
func (s *Server) photosWhere(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var from, to time.Time
	var err error
	switch {
	case q.Get("month") != "":
		from, err = time.Parse("2006-01", q.Get("month"))
		to = from.AddDate(0, 1, 0)
	case q.Get("from") != "" && q.Get("to") != "":
		from, err = time.Parse(time.DateOnly, q.Get("from"))
		if err == nil {
			to, err = time.Parse(time.DateOnly, q.Get("to"))
			to = to.AddDate(0, 0, 1) // inclusive end date
		}
	default:
		http.Error(w, "give month=YYYY-MM or from=YYYY-MM-DD&to=YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	if err != nil || !to.After(from) {
		http.Error(w, "bad dates (format YYYY-MM or YYYY-MM-DD, from before to)", http.StatusBadRequest)
		return
	}
	res, err := s.Store.WhereTaken(r.Context(), s.UserID, from, to)
	if err != nil {
		httpError(w, "lookup", err)
		return
	}
	undated, err := s.Store.UndatedBackups(r.Context(), s.UserID)
	if err != nil {
		httpError(w, "undated", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"from": from.Format(time.DateOnly), "to_exclusive": to.Format(time.DateOnly),
		"accounts": res, "undated_backups": undated})
}
