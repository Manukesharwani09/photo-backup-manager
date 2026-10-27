package backup

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Manukesharwani09/photo-backup-manager/internal/application/account"
	"github.com/Manukesharwani09/photo-backup-manager/internal/enum"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/client/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/httperror"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/logger"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
	"github.com/Manukesharwani09/photo-backup-manager/internal/planner"
)

type BackupService interface {
	CreateDevice(ctx context.Context, req CreateDeviceRequest) (CreateDeviceResponse, error)
	// Sync decides, per reported file, whether it is skipped, already backed up, excluded, or planned for
	// upload to a specific account. A device's first sync only records its start point.
	Sync(ctx context.Context, req SyncRequest) (SyncResponse, error)
	// CompleteUploads turns upload tokens (bytes the phone already sent to Google) into media items in the
	// right account and folder album, records them and verifies them. Failures stay pending for the next sync.
	CompleteUploads(ctx context.Context, req UploadsRequest) (UploadsResponse, error)
}

type backupService struct {
	repo   *repository.Store
	access *account.Access
	userID int64
}

func NewBackupService(repo *repository.Store, access *account.Access, userID int64) BackupService {
	return &backupService{repo: repo, access: access, userID: userID}
}

// RequireDevice returns a 404 error unless the device belongs to the owner.
func RequireDevice(ctx context.Context, repo *repository.Store, userID, deviceID int64) error {
	_, err := repo.DeviceFirstScan(ctx, userID, deviceID)
	if errors.Is(err, repository.ErrNotFound) {
		return httperror.NewHTTPError(httperror.NOT_FOUND, "unknown device_id; register with POST /v1/devices")
	}
	return err
}

func (s *backupService) CreateDevice(ctx context.Context, req CreateDeviceRequest) (CreateDeviceResponse, error) {
	id, err := s.repo.CreateDevice(ctx, s.userID, req.Model)
	if err != nil {
		return CreateDeviceResponse{}, err
	}
	logger.FromContext(ctx).Info("device registered", "device_id", id)
	return CreateDeviceResponse{DeviceID: id}, nil
}

func (s *backupService) Sync(ctx context.Context, req SyncRequest) (SyncResponse, error) {
	firstScanAt, err := s.repo.DeviceFirstScan(ctx, s.userID, req.DeviceID)
	if errors.Is(err, repository.ErrNotFound) {
		return SyncResponse{}, httperror.NewHTTPError(httperror.NOT_FOUND, "unknown device_id; register with POST /v1/devices")
	}
	if err != nil {
		return SyncResponse{}, err
	}
	firstScan := firstScanAt == nil
	rules, err := s.repo.ListRules(ctx, s.userID)
	if err != nil {
		return SyncResponse{}, err
	}
	accts, err := s.repo.PlannerAccounts(ctx, s.userID)
	if err != nil {
		return SyncResponse{}, err
	}

	out := SyncResponse{FirstScan: firstScan, Counts: map[string]int{}, Plan: []PlanItem{}, Waiting: []WaitingItem{},
		Rejected: []string{}, Tokens: map[int64]account.AccessToken{}}
	plannedMedia := map[int64]bool{} // the same content in two folders is uploaded once

	for _, f := range req.Files {
		meta, problem := toFileMeta(f)
		if problem != "" {
			out.Rejected = append(out.Rejected, problem)
			continue
		}
		fileID, mediaID, prevState, changed, err := s.repo.UpsertFile(ctx, s.userID, req.DeviceID, meta)
		if err != nil {
			return SyncResponse{}, err
		}
		backed, err := s.repo.IsBackedUp(ctx, mediaID)
		if err != nil {
			return SyncResponse{}, err
		}
		deliberate := false
		if !backed {
			if deliberate, err = s.repo.DeletedOnPurpose(ctx, mediaID); err != nil {
				return SyncResponse{}, err
			}
		}
		rule := planner.RuleFor(rules, meta.RelativePath)
		var state enum.FileState
		switch {
		case backed:
			state = enum.FILE_STATE_BACKED_UP
		case firstScan:
			state = enum.FILE_STATE_EXISTING_SKIP
		case enum.FileState(prevState) == enum.FILE_STATE_EXISTING_SKIP && !changed:
			state = enum.FILE_STATE_EXISTING_SKIP // present at the start point: never uploaded
		case deliberate:
			state = enum.FILE_STATE_EXCLUDED // user removed it from Google via "To delete": don't bring it back
		case rule != nil && rule.Backup == string(enum.RULE_BACKUP_EXCLUDE):
			state = enum.FILE_STATE_EXCLUDED
		default:
			state = enum.FILE_STATE_PENDING
		}
		if err := s.repo.SetFileState(ctx, fileID, string(state)); err != nil {
			return SyncResponse{}, err
		}
		out.Counts[string(state)]++
		if state != enum.FILE_STATE_PENDING || plannedMedia[mediaID] {
			continue
		}

		acctID, switched, reason := planner.ChooseAccount(accts, rule)
		if reason != "" {
			out.Waiting = append(out.Waiting, WaitingItem{FileID: fileID, Reason: reason})
			continue
		}
		if switched {
			if err := s.repo.SwitchCurrent(ctx, s.userID, acctID, string(enum.PERIOD_END_FULL)); err != nil {
				return SyncResponse{}, err
			}
			for i := range accts {
				accts[i].IsCurrent = accts[i].ID == acctID
			}
			logger.FromContext(ctx).Info("current account switched", "account_id", acctID, "reason", enum.PERIOD_END_FULL)
		}
		plannedMedia[mediaID] = true
		out.Plan = append(out.Plan, PlanItem{FileID: fileID, SHA256: f.SHA256, RelativePath: meta.RelativePath,
			DisplayName: meta.DisplayName, AccountID: acctID, AlbumTitle: planner.AlbumTitle(meta.RelativePath)})
	}
	out.Counts["planned"] = len(out.Plan)

	if firstScan {
		if err := s.repo.MarkFirstScan(ctx, req.DeviceID); err != nil {
			return SyncResponse{}, err
		}
	}
	for _, p := range out.Plan {
		if _, ok := out.Tokens[p.AccountID]; ok {
			continue
		}
		tok, err := s.access.AccessToken(ctx, p.AccountID)
		if err != nil {
			return SyncResponse{}, fmt.Errorf("access token for account %d: %w", p.AccountID, err)
		}
		out.Tokens[p.AccountID] = tok
	}
	return out, nil
}

type uploadJob struct {
	file repository.UploadFile
	item UploadItem
}

func (s *backupService) CompleteUploads(ctx context.Context, req UploadsRequest) (UploadsResponse, error) {
	if err := RequireDevice(ctx, s.repo, s.userID, req.DeviceID); err != nil {
		return UploadsResponse{}, err
	}
	results := map[int64]*UploadResult{}
	var order []int64
	byAccount := map[int64]map[string][]uploadJob{} // account -> folder -> jobs
	var accountOrder []int64
	for _, it := range req.Items {
		if _, dup := results[it.FileID]; dup {
			continue
		}
		res := &UploadResult{FileID: it.FileID}
		results[it.FileID] = res
		order = append(order, it.FileID)
		f, err := s.repo.FileOnDevice(ctx, req.DeviceID, it.FileID)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			res.Status, res.Message = UPLOAD_INVALID, "file not on this device"
			continue
		case err != nil:
			return UploadsResponse{}, err
		case it.UploadToken == "":
			res.Status, res.Message = UPLOAD_INVALID, "empty upload_token"
			continue
		}
		backed, err := s.repo.IsBackedUp(ctx, f.MediaID)
		if err != nil {
			return UploadsResponse{}, err
		}
		if backed {
			res.Status = UPLOAD_ALREADY_BACKED_UP
			continue
		}
		if enum.FileState(f.State) != enum.FILE_STATE_PENDING {
			res.Status, res.Message = UPLOAD_INVALID, "file is "+f.State+", not pending"
			continue
		}
		if _, err := s.repo.AccountOwned(ctx, s.userID, it.AccountID); err != nil {
			res.Status, res.Message = UPLOAD_INVALID, "unknown account_id"
			continue
		}
		if byAccount[it.AccountID] == nil {
			byAccount[it.AccountID] = map[string][]uploadJob{}
			accountOrder = append(accountOrder, it.AccountID)
		}
		byAccount[it.AccountID][f.RelativePath] = append(byAccount[it.AccountID][f.RelativePath], uploadJob{f, it})
	}

	for _, acctID := range accountOrder {
		if err := s.createInAccount(ctx, acctID, byAccount[acctID], results); err != nil {
			return UploadsResponse{}, err
		}
	}
	out := UploadsResponse{Results: make([]*UploadResult, 0, len(order))}
	for _, id := range order {
		out.Results = append(out.Results, results[id])
	}
	return out, nil
}

// createInAccount runs batchCreate for one account, folder by folder, and records the outcomes.
func (s *backupService) createInAccount(ctx context.Context, acctID int64, byFolder map[string][]uploadJob, results map[int64]*UploadResult) error {
	log := logger.FromContext(ctx)
	_, client, err := s.access.ClientFor(ctx, acctID)
	if err != nil {
		return err
	}
	full := false
	itemToRecord := map[string]int64{}
	for folder, jobs := range byFolder {
		if full {
			markAll(jobs, results, UPLOAD_ACCOUNT_FULL, "")
			continue
		}
		albumRow, albumID, err := EnsureAlbum(ctx, s.repo, client, acctID, folder, planner.AlbumTitle(folder))
		if err != nil {
			markAll(jobs, results, UPLOAD_FAILED, "album: "+err.Error())
			continue
		}
		for start := 0; start < len(jobs); start += google.MaxBatch {
			chunk := jobs[start:min(start+google.MaxBatch, len(jobs))]
			if full {
				markAll(chunk, results, UPLOAD_ACCOUNT_FULL, "")
				continue
			}
			items := make([]google.NewItem, len(chunk))
			byToken := map[string]uploadJob{}
			for i, j := range chunk {
				items[i] = google.NewItem{UploadToken: j.item.UploadToken, FileName: j.file.DisplayName,
					Description: j.file.RelativePath + "/" + j.file.DisplayName}
				byToken[j.item.UploadToken] = j
			}
			created, err := google.BatchCreate(ctx, client, albumID, items)
			if err != nil {
				markAll(chunk, results, UPLOAD_FAILED, err.Error())
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
						res.Status, res.Message = UPLOAD_ACCOUNT_FULL, c.Message
					} else {
						res.Status, res.Message = UPLOAD_FAILED, c.Message
					}
					continue
				}
				rid, err := s.repo.RecordBackup(ctx, j.file.MediaID, acctID, c.MediaItemID, c.ProductURL, &albumRow)
				if errors.Is(err, repository.ErrAlreadyBacked) {
					res.Status, res.Message = UPLOAD_ALREADY_BACKED_UP, "created a duplicate in Google Photos (race); remove "+c.ProductURL
					continue
				}
				if err != nil {
					return err
				}
				res.Status, res.MediaItemID, res.ProductURL = UPLOAD_BACKED_UP, c.MediaItemID, c.ProductURL
				itemToRecord[c.MediaItemID] = rid
			}
			for _, j := range chunk { // Google returned no result for this token
				if results[j.file.ID].Status == "" {
					results[j.file.ID].Status, results[j.file.ID].Message = UPLOAD_FAILED, "no result from batchCreate"
				}
			}
		}
	}
	if full {
		if err := s.repo.MarkFull(ctx, s.userID, acctID); err != nil {
			return err
		}
		log.Warn("account reported full by Google", "account_id", acctID)
	}
	s.verify(ctx, client, itemToRecord, results)
	return nil
}

func markAll(jobs []uploadJob, results map[int64]*UploadResult, status, msg string) {
	for _, j := range jobs {
		results[j.file.ID].Status, results[j.file.ID].Message = status, msg
	}
}

// verify asks Google for the just-created items; found ones become verified. A failed check is not an
// error: the records stay "uploaded" and the scheduled cloud check confirms them later.
func (s *backupService) verify(ctx context.Context, client *http.Client, itemToRecord map[string]int64, results map[int64]*UploadResult) {
	log := logger.FromContext(ctx)
	ids := make([]string, 0, len(itemToRecord))
	for id := range itemToRecord {
		ids = append(ids, id)
	}
	for start := 0; start < len(ids); start += google.MaxBatch {
		chunk := ids[start:min(start+google.MaxBatch, len(ids))]
		found, err := google.BatchGet(ctx, client, chunk)
		if err != nil {
			log.Warn("verify uploads failed; will retry in cloud check", "error", err)
			continue
		}
		var ok []int64
		for _, id := range chunk {
			if _, present := found[id]; present {
				ok = append(ok, itemToRecord[id])
			}
		}
		if err := s.repo.MarkVerified(ctx, ok); err != nil {
			log.Warn("mark verified failed", "error", err)
			continue
		}
		for _, res := range results {
			if _, present := found[res.MediaItemID]; present {
				res.Verified = true
			}
		}
	}
}
