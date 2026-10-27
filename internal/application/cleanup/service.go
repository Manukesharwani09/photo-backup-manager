package cleanup

import (
	"context"

	"github.com/Manukesharwani09/photo-backup-manager/internal/application/backup"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/httperror"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/logger"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
	"github.com/Manukesharwani09/photo-backup-manager/internal/planner"
)

type CleanupService interface {
	// Plan lists phone files in the chosen folders that are safe to delete: verified in Google,
	// not protected, still on the phone. It deletes nothing.
	Plan(ctx context.Context, req PlanRequest) (PlanResponse, error)
	// Done records which files the phone actually deleted (the user may cancel Android's dialog).
	Done(ctx context.Context, req DoneRequest) (DoneResponse, error)
}

type cleanupService struct {
	repo   *repository.Store
	userID int64
}

func NewCleanupService(repo *repository.Store, userID int64) CleanupService {
	return &cleanupService{repo: repo, userID: userID}
}

func (s *cleanupService) Plan(ctx context.Context, req PlanRequest) (PlanResponse, error) {
	if len(req.Folders) == 0 {
		return PlanResponse{}, httperror.NewHTTPError(httperror.BAD_REQUEST, "folders must not be empty")
	}
	if err := backup.RequireDevice(ctx, s.repo, s.userID, req.DeviceID); err != nil {
		return PlanResponse{}, err
	}
	rules, err := s.repo.ListRules(ctx, s.userID)
	if err != nil {
		return PlanResponse{}, err
	}
	files, err := s.repo.DeviceFiles(ctx, req.DeviceID)
	if err != nil {
		return PlanResponse{}, err
	}
	out := PlanResponse{Eligible: []File{}, Kept: map[string]int{}}
	for _, f := range files {
		if !inFolders(req.Folders, f.RelativePath) {
			continue
		}
		if why := planner.CleanupBlocker(planner.RuleFor(rules, f.RelativePath), f.OnDevice, f.BackupStatus); why != "" {
			if why != planner.KeepNotOnDevice {
				out.Kept[why]++
			}
			continue
		}
		out.Eligible = append(out.Eligible, File{FileID: f.ID, RelativePath: f.RelativePath, DisplayName: f.DisplayName,
			Size: f.Size, AccountID: deref(f.AccountID), AccountEmail: derefS(f.AccountEmail), ProductURL: derefS(f.ProductURL)})
		out.TotalBytes += f.Size
	}
	return out, nil
}

func (s *cleanupService) Done(ctx context.Context, req DoneRequest) (DoneResponse, error) {
	if err := backup.RequireDevice(ctx, s.repo, s.userID, req.DeviceID); err != nil {
		return DoneResponse{}, err
	}
	n, err := s.repo.MarkRemovedFromDevice(ctx, req.DeviceID, req.FileIDs)
	if err != nil {
		return DoneResponse{}, err
	}
	logger.FromContext(ctx).Info("phone files removed after cleanup", "device_id", req.DeviceID, "removed", n)
	return DoneResponse{MarkedRemoved: n, Ignored: int64(len(req.FileIDs)) - n}, nil
}

func inFolders(folders []string, path string) bool {
	for _, f := range folders {
		if planner.Matches(f, path) {
			return true
		}
	}
	return false
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
