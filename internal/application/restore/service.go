package restore

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/Manukesharwani09/photo-backup-manager/internal/application/account"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/backup"
	"github.com/Manukesharwani09/photo-backup-manager/internal/enum"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/client/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/httperror"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/logger"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
	"github.com/Manukesharwani09/photo-backup-manager/internal/planner"
)

type RestoreService interface {
	// Plan returns download links for every backed-up photo of a folder, from whichever accounts hold
	// them, skipping files the device already has.
	Plan(ctx context.Context, req PlanRequest) (PlanResponse, error)
	// Done links restored bytes (Google changes them on download) to the original backup, so they are
	// never uploaded again.
	Done(ctx context.Context, req DoneRequest) (DoneResponse, error)
}

type restoreService struct {
	repo   *repository.Store
	access *account.Access
	userID int64
}

func NewRestoreService(repo *repository.Store, access *account.Access, userID int64) RestoreService {
	return &restoreService{repo: repo, access: access, userID: userID}
}

func (s *restoreService) Plan(ctx context.Context, req PlanRequest) (PlanResponse, error) {
	if planner.NormalizeFolder(req.Folder) == "" {
		return PlanResponse{}, httperror.NewHTTPError(httperror.BAD_REQUEST, "folder must not be empty")
	}
	if err := backup.RequireDevice(ctx, s.repo, s.userID, req.DeviceID); err != nil {
		return PlanResponse{}, err
	}
	cands, err := s.repo.RestoreCandidates(ctx, s.userID)
	if err != nil {
		return PlanResponse{}, err
	}
	present, err := s.repo.PresentPaths(ctx, req.DeviceID)
	if err != nil {
		return PlanResponse{}, err
	}
	out := PlanResponse{Items: []Item{}, MissingInGoogle: []string{}, Tokens: map[int64]account.AccessToken{}}
	byAccount := map[int64][]repository.RestoreCandidate{}
	var accountOrder []int64
	for _, c := range cands {
		if !planner.Matches(req.Folder, c.RelativePath) {
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
		_, client, err := s.access.ClientFor(ctx, acctID)
		if err != nil {
			return PlanResponse{}, err
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
				return PlanResponse{}, err
			}
			for _, c := range chunk {
				m, ok := found[c.MediaItemID]
				if !ok {
					out.MissingInGoogle = append(out.MissingInGoogle, c.RelativePath+"/"+c.DisplayName)
					continue
				}
				out.Items = append(out.Items, Item{MediaID: c.MediaID, RelativePath: c.RelativePath, DisplayName: c.DisplayName,
					Size: c.Size, AccountID: acctID, DownloadURL: m.DownloadURL(), MimeType: m.MimeType,
					DateTaken: c.DateTaken, GPSLat: c.GPSLat, GPSLng: c.GPSLng})
			}
		}
		tok, err := s.access.AccessToken(ctx, acctID)
		if err != nil {
			return PlanResponse{}, fmt.Errorf("access token for account %d: %w", acctID, err)
		}
		out.Tokens[acctID] = tok
	}
	return out, nil
}

func (s *restoreService) Done(ctx context.Context, req DoneRequest) (DoneResponse, error) {
	if err := backup.RequireDevice(ctx, s.repo, s.userID, req.DeviceID); err != nil {
		return DoneResponse{}, err
	}
	out := DoneResponse{Rejected: []string{}}
	for _, it := range req.Items {
		sum, err := hex.DecodeString(it.SHA256)
		if err != nil || len(sum) != 32 || it.DisplayName == "" {
			out.Rejected = append(out.Rejected, it.RelativePath+"/"+it.DisplayName+": bad sha256 or name")
			continue
		}
		f := repository.FileMeta{RelativePath: planner.NormalizeFolder(it.RelativePath), DisplayName: it.DisplayName,
			Size: it.Size, SHA256: sum, Source: string(enum.FILE_SOURCE_OTHER)}
		ok, err := s.repo.RecordRestored(ctx, s.userID, req.DeviceID, it.MediaID, f)
		if err != nil {
			return DoneResponse{}, err
		}
		if !ok {
			out.Rejected = append(out.Rejected, it.RelativePath+"/"+it.DisplayName+": not a backed-up photo")
			continue
		}
		out.Linked++
	}
	logger.FromContext(ctx).Info("restore recorded", "device_id", req.DeviceID, "linked", out.Linked, "rejected", len(out.Rejected))
	return out, nil
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
