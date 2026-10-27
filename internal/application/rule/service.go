package rule

import (
	"context"
	"errors"

	"github.com/Manukesharwani09/photo-backup-manager/internal/enum"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/httperror"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
	"github.com/Manukesharwani09/photo-backup-manager/internal/planner"
)

type RuleService interface {
	List(ctx context.Context) ([]RuleDTO, error)
	// Replace swaps the whole rule set (old rules are soft-deleted) and returns the new list.
	Replace(ctx context.Context, rules []RuleDTO) ([]RuleDTO, error)
}

type ruleService struct {
	repo   *repository.Store
	userID int64
}

func NewRuleService(repo *repository.Store, userID int64) RuleService {
	return &ruleService{repo: repo, userID: userID}
}

func (s *ruleService) List(ctx context.Context) ([]RuleDTO, error) {
	rules, err := s.repo.ListRules(ctx, s.userID)
	if err != nil {
		return nil, err
	}
	out := make([]RuleDTO, 0, len(rules))
	for _, x := range rules {
		out = append(out, RuleDTO{x.PathGlob, x.TargetAccountID, x.Backup, x.Cleanup, x.Priority})
	}
	return out, nil
}

func (s *ruleService) Replace(ctx context.Context, in []RuleDTO) ([]RuleDTO, error) {
	rules := make([]planner.Rule, 0, len(in))
	for i, x := range in {
		if x.Backup == "" {
			x.Backup = string(enum.RULE_BACKUP_INCLUDE)
		}
		if x.Cleanup == "" {
			x.Cleanup = string(enum.RULE_CLEANUP_ALLOWED)
		}
		if planner.NormalizeFolder(x.PathGlob) == "" || !enum.RuleBackup(x.Backup).IsValid() || !enum.RuleCleanup(x.Cleanup).IsValid() {
			return nil, httperror.NewHTTPErrorf(httperror.BAD_REQUEST, "rule %d: need path, backup include|exclude, cleanup allowed|protected", i)
		}
		rules = append(rules, planner.Rule{PathGlob: x.PathGlob, TargetAccountID: x.TargetAccountID, Backup: x.Backup, Cleanup: x.Cleanup, Priority: x.Priority})
	}
	err := s.repo.ReplaceRules(ctx, s.userID, rules)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, httperror.NewHTTPError(httperror.BAD_REQUEST, "target_account_id is not one of your accounts")
	}
	if err != nil {
		return nil, err
	}
	return s.List(ctx)
}
