package api

import (
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/client/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
	"github.com/Manukesharwani09/photo-backup-manager/internal/planner"
)

type restoreItem struct {
	MediaID      int64      `json:"media_id"`
	RelativePath string     `json:"relative_path"`
	DisplayName  string     `json:"display_name"`
	Size         int64      `json:"size"`
	AccountID    int64      `json:"account_id"`
	DownloadURL  string     `json:"download_url"` // valid ~60 minutes; fetch with the account's access token
	MimeType     string     `json:"mime_type"`
	DateTaken    *time.Time `json:"date_taken,omitempty"`
	GPSLat       *float64   `json:"gps_lat,omitempty"` // the phone writes these back if the download lost them
	GPSLng       *float64   `json:"gps_lng,omitempty"`
}

// restorePlan returns download links for every backed-up photo of a folder (and its sub-folders),
// from whichever accounts hold them, skipping files the device already has.
func (s *Server) restorePlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		DeviceID int64  `json:"device_id"`
		Folder   string `json:"folder"`
	}
	if !decode(w, r, &in) {
		return
	}
	if planner.NormalizeFolder(in.Folder) == "" {
		http.Error(w, "folder must not be empty", http.StatusBadRequest)
		return
	}
	if !s.deviceExists(w, r, in.DeviceID) {
		return
	}
	cands, err := s.Store.RestoreCandidates(ctx, s.UserID)
	if err != nil {
		httpError(w, "load candidates", err)
		return
	}
	present, err := s.Store.PresentPaths(ctx, in.DeviceID)
	if err != nil {
		httpError(w, "load device files", err)
		return
	}

	out := struct {
		Items           []restoreItem         `json:"items"`
		AlreadyOnDevice int                   `json:"already_on_device"`
		MissingInGoogle []string              `json:"missing_in_google"` // deleted in Google Photos; can't restore
		Tokens          map[int64]accessToken `json:"tokens"`
	}{Items: []restoreItem{}, MissingInGoogle: []string{}, Tokens: map[int64]accessToken{}}

	byAccount := map[int64][]repository.RestoreCandidate{}
	var accountOrder []int64
	for _, c := range cands {
		if !planner.Matches(in.Folder, c.RelativePath) {
			continue
		}
		if present[c.RelativePath+"/"+c.DisplayName] {
			out.AlreadyOnDevice++
			continue
		}
		if byAccount[c.AccountID] == nil {
			accountOrder = append(accountOrder, c.AccountID)
		}
		byAccount[c.AccountID] = append(byAccount[c.AccountID], c)
	}
	for _, acctID := range accountOrder {
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
		list := byAccount[acctID]
		for start := 0; start < len(list); start += google.MaxBatch {
			chunk := list[start:min(start+google.MaxBatch, len(list))]
			ids := make([]string, len(chunk))
			for i, c := range chunk {
				ids[i] = c.MediaItemID
			}
			found, err := google.BatchGet(ctx, client, uniq(ids))
			if err != nil {
				httpError(w, "batchGet", err)
				return
			}
			for _, c := range chunk {
				m, ok := found[c.MediaItemID]
				if !ok {
					out.MissingInGoogle = append(out.MissingInGoogle, c.RelativePath+"/"+c.DisplayName)
					continue
				}
				out.Items = append(out.Items, restoreItem{MediaID: c.MediaID, RelativePath: c.RelativePath, DisplayName: c.DisplayName,
					Size: c.Size, AccountID: acctID, DownloadURL: m.DownloadURL(), MimeType: m.MimeType,
					DateTaken: c.DateTaken, GPSLat: c.GPSLat, GPSLng: c.GPSLng})
			}
		}
		tok, err := s.accessToken(ctx, acctID)
		if err != nil {
			httpError(w, fmt.Sprintf("access token for account %d", acctID), err)
			return
		}
		out.Tokens[acctID] = tok
	}
	writeJSON(w, http.StatusOK, out)
}

// restoreDone records the files the phone wrote back, with the hash of the bytes it actually got.
func (s *Server) restoreDone(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		DeviceID int64 `json:"device_id"`
		Items    []struct {
			MediaID      int64  `json:"media_id"`
			RelativePath string `json:"relative_path"`
			DisplayName  string `json:"display_name"`
			Size         int64  `json:"size"`
			SHA256       string `json:"sha256"`
		} `json:"items"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !s.deviceExists(w, r, in.DeviceID) {
		return
	}
	linked, rejected := 0, []string{}
	for _, it := range in.Items {
		sum, err := hex.DecodeString(it.SHA256)
		if err != nil || len(sum) != 32 || it.DisplayName == "" {
			rejected = append(rejected, it.RelativePath+"/"+it.DisplayName+": bad sha256 or name")
			continue
		}
		f := repository.FileMeta{RelativePath: planner.NormalizeFolder(it.RelativePath), DisplayName: it.DisplayName, Size: it.Size, SHA256: sum, Source: "other"}
		ok, err := s.Store.RecordRestored(ctx, s.UserID, in.DeviceID, it.MediaID, f)
		if err != nil {
			httpError(w, "record restored", err)
			return
		}
		if !ok {
			rejected = append(rejected, it.RelativePath+"/"+it.DisplayName+": not a backed-up photo")
			continue
		}
		linked++
	}
	log.Printf("restore: device %d linked %d files", in.DeviceID, linked)
	writeJSON(w, http.StatusOK, map[string]any{"linked": linked, "rejected": rejected})
}

func uniq(ss []string) []string {
	seen := map[string]bool{}
	out := ss[:0:0]
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
