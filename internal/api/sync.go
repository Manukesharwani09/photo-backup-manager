package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2"

	"github.com/Manukesharwani09/photo-backup-manager/internal/crypto"
	"github.com/Manukesharwani09/photo-backup-manager/internal/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/plan"
	"github.com/Manukesharwani09/photo-backup-manager/internal/store"
)

// Device file states (device_file.state).
const (
	StateExistingSkip = "existing_skip"
	StatePending      = "pending"
	StateBackedUp     = "backed_up"
	StateExcluded     = "excluded"
)

var validSources = map[string]bool{"camera": true, "whatsapp": true, "screenshot": true, "download": true, "other": true}

// ---- POST /devices ----

func (s *Server) createDevice(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Model string `json:"model"`
	}
	if !decode(w, r, &in) {
		return
	}
	id, err := s.Store.CreateDevice(r.Context(), s.UserID, in.Model)
	if err != nil {
		httpError(w, "create device", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"device_id": id})
}

// ---- POST /sync ----

type syncFile struct {
	RelativePath string   `json:"relative_path"`
	DisplayName  string   `json:"display_name"`
	Size         int64    `json:"size"`
	SHA256       string   `json:"sha256"` // hex
	DateTaken    *string  `json:"date_taken"`
	Width        *int     `json:"width"`
	Height       *int     `json:"height"`
	GPSLat       *float64 `json:"gps_lat"`
	GPSLng       *float64 `json:"gps_lng"`
	Source       string   `json:"source"`
}

type planItem struct {
	FileID       int64  `json:"file_id"`
	SHA256       string `json:"sha256"`
	RelativePath string `json:"relative_path"`
	DisplayName  string `json:"display_name"`
	AccountID    int64  `json:"account_id"`
	AlbumTitle   string `json:"album_title"`
}

type waitingItem struct {
	FileID int64  `json:"file_id"`
	Reason string `json:"reason"`
}

type accessToken struct {
	AccessToken string    `json:"access_token"`
	Expiry      time.Time `json:"expiry"`
}

type syncResponse struct {
	FirstScan bool                  `json:"first_scan"`
	Counts    map[string]int        `json:"counts"`
	Plan      []planItem            `json:"plan"`
	Waiting   []waitingItem         `json:"waiting"`
	Rejected  []string              `json:"rejected"` // input errors, one line per bad file
	Tokens    map[int64]accessToken `json:"tokens"`   // short-lived upload tokens for the accounts in Plan
}

// sync receives the phone's file list and decides, per file, whether it is skipped, already backed up,
// excluded, or planned for upload to a specific account. The very first sync of a device only records the
// "start point": everything already on the phone becomes existing_skip and nothing is uploaded.
func (s *Server) sync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		DeviceID int64      `json:"device_id"`
		Files    []syncFile `json:"files"`
	}
	if !decode(w, r, &in) {
		return
	}
	firstScanAt, err := s.Store.DeviceFirstScan(ctx, s.UserID, in.DeviceID)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "unknown device_id; register with POST /devices", http.StatusNotFound)
		return
	}
	if err != nil {
		httpError(w, "load device", err)
		return
	}
	firstScan := firstScanAt == nil

	rules, err := s.Store.ListRules(ctx, s.UserID)
	if err != nil {
		httpError(w, "load rules", err)
		return
	}
	accts, err := s.Store.PlannerAccounts(ctx, s.UserID)
	if err != nil {
		httpError(w, "load accounts", err)
		return
	}

	out := syncResponse{FirstScan: firstScan, Counts: map[string]int{}, Plan: []planItem{}, Waiting: []waitingItem{}, Rejected: []string{}, Tokens: map[int64]accessToken{}}
	plannedMedia := map[int64]bool{} // the same content in two folders is uploaded once

	for _, f := range in.Files {
		meta, problem := toMeta(f)
		if problem != "" {
			out.Rejected = append(out.Rejected, problem)
			continue
		}
		fileID, mediaID, prevState, changed, err := s.Store.UpsertFile(ctx, s.UserID, in.DeviceID, meta)
		if err != nil {
			httpError(w, "save file", err)
			return
		}
		backed, err := s.Store.IsBackedUp(ctx, mediaID)
		if err != nil {
			httpError(w, "check backup", err)
			return
		}
		rule := plan.RuleFor(rules, meta.RelativePath)
		var state string
		switch {
		case backed:
			state = StateBackedUp
		case firstScan:
			state = StateExistingSkip
		case prevState == StateExistingSkip && !changed:
			state = StateExistingSkip // present at the start point: never uploaded
		case rule != nil && rule.Backup == "exclude":
			state = StateExcluded
		default:
			state = StatePending
		}
		if err := s.Store.SetFileState(ctx, fileID, state); err != nil {
			httpError(w, "set state", err)
			return
		}
		out.Counts[state]++
		if state != StatePending || plannedMedia[mediaID] {
			continue
		}

		acctID, switched, reason := plan.ChooseAccount(accts, rule)
		if reason != "" {
			out.Waiting = append(out.Waiting, waitingItem{FileID: fileID, Reason: reason})
			continue
		}
		if switched {
			if err := s.Store.SwitchCurrent(ctx, s.UserID, acctID, "full"); err != nil {
				httpError(w, "switch account", err)
				return
			}
			for i := range accts {
				accts[i].IsCurrent = accts[i].ID == acctID
			}
			log.Printf("current account switched to %d", acctID)
		}
		plannedMedia[mediaID] = true
		out.Plan = append(out.Plan, planItem{FileID: fileID, SHA256: f.SHA256, RelativePath: meta.RelativePath,
			DisplayName: meta.DisplayName, AccountID: acctID, AlbumTitle: plan.AlbumTitle(meta.RelativePath)})
	}
	out.Counts["planned"] = len(out.Plan)

	if firstScan {
		if err := s.Store.MarkFirstScan(ctx, in.DeviceID); err != nil {
			httpError(w, "mark first scan", err)
			return
		}
	}
	for _, p := range out.Plan {
		if _, ok := out.Tokens[p.AccountID]; ok {
			continue
		}
		tok, err := s.accessToken(ctx, p.AccountID)
		if err != nil {
			httpError(w, fmt.Sprintf("access token for account %d", p.AccountID), err)
			return
		}
		out.Tokens[p.AccountID] = tok
	}
	writeJSON(w, http.StatusOK, out)
}

func toMeta(f syncFile) (store.FileMeta, string) {
	m := store.FileMeta{RelativePath: plan.NormalizeFolder(f.RelativePath), DisplayName: f.DisplayName, Size: f.Size,
		Width: f.Width, Height: f.Height, GPSLat: f.GPSLat, GPSLng: f.GPSLng, Source: f.Source}
	if m.DisplayName == "" {
		return m, "file with empty display_name"
	}
	sum, err := hex.DecodeString(f.SHA256)
	if err != nil || len(sum) != 32 {
		return m, m.RelativePath + "/" + m.DisplayName + ": sha256 must be 64 hex characters"
	}
	m.SHA256 = sum
	if m.Size < 0 {
		return m, m.RelativePath + "/" + m.DisplayName + ": negative size"
	}
	if m.Source == "" {
		m.Source = "other"
	}
	if !validSources[m.Source] {
		return m, m.RelativePath + "/" + m.DisplayName + ": unknown source " + m.Source
	}
	if f.DateTaken != nil && *f.DateTaken != "" {
		t, err := time.Parse(time.RFC3339, *f.DateTaken)
		if err != nil {
			return m, m.RelativePath + "/" + m.DisplayName + ": date_taken must be RFC 3339"
		}
		m.DateTaken = &t
	}
	if (m.GPSLat == nil) != (m.GPSLng == nil) ||
		(m.GPSLat != nil && (*m.GPSLat < -90 || *m.GPSLat > 90 || *m.GPSLng < -180 || *m.GPSLng > 180)) {
		return m, m.RelativePath + "/" + m.DisplayName + ": invalid GPS"
	}
	return m, ""
}

// accessToken mints a short-lived access token for the phone to upload bytes to an account.
func (s *Server) accessToken(ctx context.Context, accountID int64) (accessToken, error) {
	a, err := s.Store.AccountOwned(ctx, s.UserID, accountID)
	if err != nil {
		return accessToken{}, err
	}
	rt, err := crypto.Open(s.TokenKey, a.RefreshTokenEnc)
	if err != nil {
		return accessToken{}, err
	}
	tok, err := s.OAuth.TokenSource(ctx, &oauth2.Token{RefreshToken: string(rt)}).Token()
	if err != nil {
		return accessToken{}, err
	}
	return accessToken{AccessToken: tok.AccessToken, Expiry: tok.Expiry}, nil
}

// ---- POST /uploads ----

type uploadItem struct {
	FileID      int64  `json:"file_id"`
	AccountID   int64  `json:"account_id"`
	UploadToken string `json:"upload_token"`
}

type uploadResult struct {
	FileID      int64  `json:"file_id"`
	Status      string `json:"status"` // backed_up | already_backed_up | account_full | failed | invalid
	MediaItemID string `json:"media_item_id,omitempty"`
	ProductURL  string `json:"product_url,omitempty"`
	Verified    bool   `json:"verified,omitempty"`
	Message     string `json:"message,omitempty"`
}

// uploads turns upload tokens (bytes the phone already sent to Google) into media items in the right
// account and folder album, records them, and verifies them. Failed items stay pending for the next sync.
func (s *Server) uploads(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		DeviceID int64        `json:"device_id"`
		Items    []uploadItem `json:"items"`
	}
	if !decode(w, r, &in) {
		return
	}
	if _, err := s.Store.DeviceFirstScan(ctx, s.UserID, in.DeviceID); errors.Is(err, store.ErrNotFound) {
		http.Error(w, "unknown device_id", http.StatusNotFound)
		return
	} else if err != nil {
		httpError(w, "load device", err)
		return
	}

	results := map[int64]*uploadResult{}
	order := []int64{}
	type job struct {
		file store.UploadFile
		item uploadItem
	}
	byAccount := map[int64]map[string][]job{} // account -> folder -> jobs
	accountOrder := []int64{}
	for _, it := range in.Items {
		res := &uploadResult{FileID: it.FileID}
		if _, dup := results[it.FileID]; dup {
			continue
		}
		results[it.FileID] = res
		order = append(order, it.FileID)
		f, err := s.Store.FileOnDevice(ctx, in.DeviceID, it.FileID)
		switch {
		case errors.Is(err, store.ErrNotFound):
			res.Status, res.Message = "invalid", "file not on this device"
			continue
		case err != nil:
			httpError(w, "load file", err)
			return
		case it.UploadToken == "":
			res.Status, res.Message = "invalid", "empty upload_token"
			continue
		}
		if backed, err := s.Store.IsBackedUp(ctx, f.MediaID); err != nil {
			httpError(w, "check backup", err)
			return
		} else if backed {
			res.Status = "already_backed_up"
			continue
		}
		if f.State != StatePending {
			res.Status, res.Message = "invalid", "file is "+f.State+", not pending"
			continue
		}
		if _, err := s.Store.AccountOwned(ctx, s.UserID, it.AccountID); err != nil {
			res.Status, res.Message = "invalid", "unknown account_id"
			continue
		}
		if byAccount[it.AccountID] == nil {
			byAccount[it.AccountID] = map[string][]job{}
			accountOrder = append(accountOrder, it.AccountID)
		}
		byAccount[it.AccountID][f.RelativePath] = append(byAccount[it.AccountID][f.RelativePath], job{f, it})
	}

	for _, acctID := range accountOrder {
		acct, _ := s.Store.AccountOwned(ctx, s.UserID, acctID)
		client, err := s.Client(ctx, acct)
		if err != nil {
			httpError(w, "account client", err)
			return
		}
		full := false
		var recordIDs []int64
		itemToRecord := map[string]int64{}
		for folder, jobs := range byAccount[acctID] {
			if full {
				for _, j := range jobs {
					results[j.file.ID].Status = "account_full"
				}
				continue
			}
			albumRow, albumID, err := s.ensureAlbum(ctx, client, acctID, folder)
			if err != nil {
				for _, j := range jobs {
					results[j.file.ID].Status, results[j.file.ID].Message = "failed", "album: "+err.Error()
				}
				continue
			}
			for start := 0; start < len(jobs); start += google.MaxBatch {
				chunk := jobs[start:min(start+google.MaxBatch, len(jobs))]
				if full {
					for _, j := range chunk {
						results[j.file.ID].Status = "account_full"
					}
					continue
				}
				items := make([]google.NewItem, len(chunk))
				byToken := map[string]job{}
				for i, j := range chunk {
					items[i] = google.NewItem{UploadToken: j.item.UploadToken, FileName: j.file.DisplayName,
						Description: j.file.RelativePath + "/" + j.file.DisplayName}
					byToken[j.item.UploadToken] = j
				}
				created, err := google.BatchCreate(ctx, client, albumID, items)
				if err != nil {
					for _, j := range chunk {
						results[j.file.ID].Status, results[j.file.ID].Message = "failed", err.Error()
					}
					continue
				}
				for _, c := range created {
					j, ok := byToken[c.UploadToken]
					if !ok {
						continue
					}
					res := results[j.file.ID]
					if c.Code != 0 {
						if google.IsStorageFull(c.Code, c.Message) {
							full = true
							res.Status, res.Message = "account_full", c.Message
						} else {
							res.Status, res.Message = "failed", c.Message
						}
						continue
					}
					rid, err := s.Store.RecordBackup(ctx, j.file.MediaID, acctID, c.MediaItemID, c.ProductURL, &albumRow)
					if errors.Is(err, store.ErrAlreadyBacked) {
						res.Status, res.Message = "already_backed_up", "created a duplicate in Google Photos (race); remove "+c.ProductURL
						continue
					}
					if err != nil {
						httpError(w, "record backup", err)
						return
					}
					res.Status, res.MediaItemID, res.ProductURL = "backed_up", c.MediaItemID, c.ProductURL
					recordIDs = append(recordIDs, rid)
					itemToRecord[c.MediaItemID] = rid
				}
				for _, j := range chunk { // Google returned no result for this token
					if results[j.file.ID].Status == "" {
						results[j.file.ID].Status, results[j.file.ID].Message = "failed", "no result from batchCreate"
					}
				}
			}
		}
		if full {
			if err := s.Store.MarkFull(ctx, s.UserID, acctID); err != nil {
				httpError(w, "mark full", err)
				return
			}
			log.Printf("account %d reported full", acctID)
		}
		s.verify(ctx, client, itemToRecord, results)
	}

	out := make([]*uploadResult, 0, len(order))
	for _, id := range order {
		out = append(out, results[id])
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out})
}

func (s *Server) ensureAlbum(ctx context.Context, client *http.Client, accountID int64, folder string) (int64, string, error) {
	row, gid, err := s.Store.Album(ctx, accountID, folder)
	if err == nil {
		return row, gid, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return 0, "", err
	}
	title := plan.AlbumTitle(folder)
	gid, err = google.CreateAlbum(ctx, client, title)
	if err != nil {
		return 0, "", err
	}
	row, err = s.Store.SaveAlbum(ctx, accountID, folder, title, gid)
	return row, gid, err
}

// verify asks Google for the just-created items; found ones become verified. A failed check is not an
// error: the records stay "uploaded" and are re-checked later.
func (s *Server) verify(ctx context.Context, client *http.Client, itemToRecord map[string]int64, results map[int64]*uploadResult) {
	ids := make([]string, 0, len(itemToRecord))
	for id := range itemToRecord {
		ids = append(ids, id)
	}
	for start := 0; start < len(ids); start += google.MaxBatch {
		chunk := ids[start:min(start+google.MaxBatch, len(ids))]
		found, err := google.BatchGet(ctx, client, chunk)
		if err != nil {
			log.Printf("verify: %v", err)
			continue
		}
		var ok []int64
		for _, id := range chunk {
			if _, present := found[id]; present {
				ok = append(ok, itemToRecord[id])
			}
		}
		if err := s.Store.MarkVerified(ctx, ok); err != nil {
			log.Printf("mark verified: %v", err)
			continue
		}
		for _, res := range results {
			if _, present := found[res.MediaItemID]; present {
				res.Verified = true
			}
		}
	}
}

// ---- rules ----

type ruleJSON struct {
	PathGlob        string `json:"path"`
	TargetAccountID *int64 `json:"target_account_id"`
	Backup          string `json:"backup"`
	Cleanup         string `json:"cleanup"`
	Priority        int    `json:"priority"`
}

func (s *Server) getRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.Store.ListRules(r.Context(), s.UserID)
	if err != nil {
		httpError(w, "list rules", err)
		return
	}
	out := make([]ruleJSON, 0, len(rules))
	for _, x := range rules {
		out = append(out, ruleJSON{x.PathGlob, x.TargetAccountID, x.Backup, x.Cleanup, x.Priority})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) putRules(w http.ResponseWriter, r *http.Request) {
	var in []ruleJSON
	if !decode(w, r, &in) {
		return
	}
	rules := make([]plan.Rule, 0, len(in))
	for i, x := range in {
		if x.Backup == "" {
			x.Backup = "include"
		}
		if x.Cleanup == "" {
			x.Cleanup = "allowed"
		}
		if plan.NormalizeFolder(x.PathGlob) == "" || (x.Backup != "include" && x.Backup != "exclude") || (x.Cleanup != "allowed" && x.Cleanup != "protected") {
			http.Error(w, fmt.Sprintf("rule %d: need path, backup include|exclude, cleanup allowed|protected", i), http.StatusBadRequest)
			return
		}
		rules = append(rules, plan.Rule{PathGlob: x.PathGlob, TargetAccountID: x.TargetAccountID, Backup: x.Backup, Cleanup: x.Cleanup, Priority: x.Priority})
	}
	if err := s.Store.ReplaceRules(r.Context(), s.UserID, rules); errors.Is(err, store.ErrNotFound) {
		http.Error(w, "target_account_id is not one of your accounts", http.StatusBadRequest)
		return
	} else if err != nil {
		httpError(w, "save rules", err)
		return
	}
	s.getRules(w, r)
}

// ---- POST /accounts/{id}/full ----

// markFull lets the user (or a test) declare an account full; the next sync moves to the next account.
func (s *Server) markFull(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad account id", http.StatusBadRequest)
		return
	}
	if err := s.Store.MarkFull(r.Context(), s.UserID, id); errors.Is(err, store.ErrNotFound) {
		http.Error(w, "unknown account", http.StatusNotFound)
		return
	} else if err != nil {
		httpError(w, "mark full", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"account_id": id, "full": true})
}

// ---- helpers ----

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}
