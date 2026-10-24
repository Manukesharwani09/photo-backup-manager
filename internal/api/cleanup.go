package api

import (
	"errors"
	"net/http"

	"github.com/Manukesharwani09/photo-backup-manager/internal/plan"
	"github.com/Manukesharwani09/photo-backup-manager/internal/store"
)

type cleanupFile struct {
	FileID       int64  `json:"file_id"`
	RelativePath string `json:"relative_path"`
	DisplayName  string `json:"display_name"`
	Size         int64  `json:"size"`
	AccountID    int64  `json:"account_id"`
	AccountEmail string `json:"account_email"`
	ProductURL   string `json:"product_url"`
}

// cleanupPlan lists the phone files in the chosen folders (and their sub-folders) that are safe to
// delete: verified in Google, not protected, still on the phone. It deletes nothing; the app shows a
// preview, the user confirms, the phone deletes, then reports back with /cleanup/done.
func (s *Server) cleanupPlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		DeviceID int64    `json:"device_id"`
		Folders  []string `json:"folders"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Folders) == 0 {
		http.Error(w, "folders must not be empty", http.StatusBadRequest)
		return
	}
	if !s.deviceExists(w, r, in.DeviceID) {
		return
	}
	rules, err := s.Store.ListRules(ctx, s.UserID)
	if err != nil {
		httpError(w, "load rules", err)
		return
	}
	files, err := s.Store.DeviceFiles(ctx, in.DeviceID)
	if err != nil {
		httpError(w, "load files", err)
		return
	}
	out := struct {
		Eligible   []cleanupFile  `json:"eligible"`
		TotalBytes int64          `json:"total_bytes"`
		Kept       map[string]int `json:"kept"` // reason -> count, for the preview's explanation
	}{Eligible: []cleanupFile{}, Kept: map[string]int{}}
	for _, f := range files {
		if !inFolders(in.Folders, f.RelativePath) {
			continue
		}
		if why := plan.CleanupBlocker(plan.RuleFor(rules, f.RelativePath), f.OnDevice, f.BackupStatus); why != "" {
			if why != plan.KeepNotOnDevice {
				out.Kept[why]++
			}
			continue
		}
		out.Eligible = append(out.Eligible, cleanupFile{FileID: f.ID, RelativePath: f.RelativePath, DisplayName: f.DisplayName,
			Size: f.Size, AccountID: deref(f.AccountID), AccountEmail: derefS(f.AccountEmail), ProductURL: derefS(f.ProductURL)})
		out.TotalBytes += f.Size
	}
	writeJSON(w, http.StatusOK, out)
}

// cleanupDone records which files the phone actually deleted (the user may cancel Android's dialog).
func (s *Server) cleanupDone(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DeviceID int64   `json:"device_id"`
		FileIDs  []int64 `json:"file_ids"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !s.deviceExists(w, r, in.DeviceID) {
		return
	}
	n, err := s.Store.MarkRemovedFromDevice(r.Context(), in.DeviceID, in.FileIDs)
	if err != nil {
		httpError(w, "mark removed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"marked_removed": n, "ignored": int64(len(in.FileIDs)) - n})
}

func inFolders(folders []string, path string) bool {
	for _, f := range folders {
		if plan.Matches(f, path) {
			return true
		}
	}
	return false
}

func (s *Server) deviceExists(w http.ResponseWriter, r *http.Request, deviceID int64) bool {
	_, err := s.Store.DeviceFirstScan(r.Context(), s.UserID, deviceID)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "unknown device_id", http.StatusNotFound)
		return false
	}
	if err != nil {
		httpError(w, "load device", err)
		return false
	}
	return true
}

func deref(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func derefS(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
