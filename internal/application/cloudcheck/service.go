package cloudcheck

import (
	"context"
	"time"

	"github.com/Manukesharwani09/photo-backup-manager/internal/application/account"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/client/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/logger"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
)

type CloudCheckService interface {
	// Check asks Google, 50 items per request, which backed-up items still exist and records the ones the
	// user deleted in Google Photos. Quota cost: about one request per 50 backed-up photos.
	Check(ctx context.Context) (Result, error)
	// Run repeats Check every interval until ctx ends. interval <= 0 disables it.
	Run(ctx context.Context, interval time.Duration)
}

type cloudCheckService struct {
	repo   *repository.Store
	access *account.Access
	userID int64
}

func NewCloudCheckService(repo *repository.Store, access *account.Access, userID int64) CloudCheckService {
	return &cloudCheckService{repo: repo, access: access, userID: userID}
}

func (s *cloudCheckService) Check(ctx context.Context) (Result, error) {
	res := Result{Errors: []string{}}
	byAccount, order, err := s.repo.LiveRecordsByAccount(ctx, s.userID)
	if err != nil {
		return res, err
	}
	for _, acctID := range order {
		acct, client, err := s.access.ClientFor(ctx, acctID)
		if err != nil {
			res.Errors = append(res.Errors, logger.MaskEmail(acct.Email)+": "+err.Error())
			continue
		}
		recs := byAccount[acctID]
		for start := 0; start < len(recs); start += google.MaxBatch {
			chunk := recs[start:min(start+google.MaxBatch, len(recs))]
			ids := make([]string, len(chunk))
			for i, r := range chunk {
				ids[i] = r.MediaItemID
			}
			found, err := google.BatchGet(ctx, client, uniq(ids))
			if err != nil {
				// Never treat an API failure as "deleted": skip this chunk and try next time.
				res.Errors = append(res.Errors, logger.MaskEmail(acct.Email)+": "+err.Error())
				continue
			}
			var present []int64
			for _, r := range chunk {
				res.Checked++
				if _, ok := found[r.MediaItemID]; ok {
					present = append(present, r.ID)
					continue
				}
				n, err := s.repo.MarkDeletedInCloud(ctx, r)
				if err != nil {
					return res, err
				}
				res.DeletedInCloud++
				res.Requeued += n
			}
			res.StillThere += len(present)
			if err := s.repo.TouchChecked(ctx, present); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

func (s *cloudCheckService) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	log := logger.GetLogger()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			res, err := s.Check(ctx)
			if err != nil {
				log.Error("cloud check failed", "error", err)
				continue
			}
			log.Info("cloud check done", "checked", res.Checked, "deleted_in_cloud", res.DeletedInCloud,
				"requeued", res.Requeued, "errors", len(res.Errors))
		}
	}
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
