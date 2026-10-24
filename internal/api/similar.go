package api

import (
	"net/http"

	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/client/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
	"github.com/Manukesharwani09/photo-backup-manager/internal/planner"
)

// ToDeleteAlbumTitle is the album, one per account, collecting cloud copies the user wants gone.
// The Library API cannot delete photos, so the user opens this album in Google Photos,
// selects all and deletes; the cloud check then notices they are gone.
const ToDeleteAlbumTitle = "🗑 To delete"

// similar returns groups of look-alike photos (different files, same picture) with where each copy is.
func (s *Server) similar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	inputs, err := s.Store.SimilarInputs(ctx, s.UserID)
	if err != nil {
		httpError(w, "load photos", err)
		return
	}
	groups := planner.SimilarGroups(inputs)
	var ids []int64
	for _, g := range groups {
		ids = append(ids, g.MediaIDs...)
	}
	details, err := s.Store.MediaDetails(ctx, s.UserID, ids)
	if err != nil {
		httpError(w, "load details", err)
		return
	}
	type groupJSON struct {
		Reason string                    `json:"reason"`
		Photos []*repository.MediaDetail `json:"photos"`
	}
	out := []groupJSON{}
	for _, g := range groups {
		gj := groupJSON{Reason: g.Reason}
		for _, id := range g.MediaIDs {
			if d := details[id]; d != nil {
				gj.Photos = append(gj.Photos, d)
			}
		}
		out = append(out, gj)
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": out})
}

// markToDelete puts the cloud copies of the chosen photos into each account's "To delete" album.
// It returns, per account, the album link and how many photos wait there for manual deletion.
func (s *Server) markToDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		MediaIDs []int64 `json:"media_ids"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.MediaIDs) == 0 {
		http.Error(w, "media_ids must not be empty", http.StatusBadRequest)
		return
	}
	items, err := s.Store.LiveBackups(ctx, s.UserID, in.MediaIDs)
	if err != nil {
		httpError(w, "load backups", err)
		return
	}
	byAccount := map[int64][]repository.ToDeleteItem{}
	var order []int64
	for _, it := range items {
		if byAccount[it.AccountID] == nil {
			order = append(order, it.AccountID)
		}
		byAccount[it.AccountID] = append(byAccount[it.AccountID], it)
	}
	type card struct {
		AccountID    int64  `json:"account_id"`
		AccountEmail string `json:"account_email"`
		AlbumURL     string `json:"album_url"`
		Added        int    `json:"added"`
		Waiting      int    `json:"waiting"` // all photos in this account marked and not yet deleted
	}
	out := struct {
		Accounts     []card `json:"accounts"`
		NotInGoogle  int    `json:"not_in_google"` // chosen photos with no cloud copy (nothing to do)
		ManualAction string `json:"manual_action"`
	}{Accounts: []card{}, NotInGoogle: len(uniqInt(in.MediaIDs)) - countMedia(items),
		ManualAction: "Open each album in Google Photos, select all, and delete. Google does not let apps delete photos."}

	for _, acctID := range order {
		acct, err := s.Store.AccountOwned(ctx, s.UserID, acctID)
		if err != nil {
			httpError(w, "load account", err)
			return
		}
		client, err := s.Client(ctx, acct)
		if err != nil {
			httpError(w, "account client", err)
			return
		}
		_, albumID, err := s.ensureNamedAlbum(ctx, client, acctID, repository.ToDeleteFolder, ToDeleteAlbumTitle)
		if err != nil {
			httpError(w, "to-delete album", err)
			return
		}
		list := byAccount[acctID]
		var mediaIDs []int64
		for start := 0; start < len(list); start += google.MaxBatch {
			chunk := list[start:min(start+google.MaxBatch, len(list))]
			ids := make([]string, len(chunk))
			for i, it := range chunk {
				ids[i] = it.MediaItemID
				mediaIDs = append(mediaIDs, it.MediaID)
			}
			if err := google.AddToAlbum(ctx, client, albumID, ids); err != nil {
				httpError(w, "add to album", err)
				return
			}
		}
		if err := s.Store.MarkForDelete(ctx, mediaIDs); err != nil {
			httpError(w, "mark for delete", err)
			return
		}
		link, _ := s.Store.AlbumLink(ctx, acctID, repository.ToDeleteFolder)
		out.Accounts = append(out.Accounts, card{AccountID: acctID, AccountEmail: acct.Email, AlbumURL: link, Added: len(list)})
	}
	pending, err := s.Store.PendingDeletes(ctx, s.UserID)
	if err != nil {
		httpError(w, "pending deletes", err)
		return
	}
	for i := range out.Accounts {
		out.Accounts[i].Waiting = pending[out.Accounts[i].AccountID]
	}
	writeJSON(w, http.StatusOK, out)
}

func countMedia(items []repository.ToDeleteItem) int {
	seen := map[int64]bool{}
	for _, it := range items {
		seen[it.MediaID] = true
	}
	return len(seen)
}

func uniqInt(ids []int64) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
