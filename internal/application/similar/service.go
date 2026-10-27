package similar

import (
	"context"

	"github.com/Manukesharwani09/photo-backup-manager/internal/application/account"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/backup"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/client/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/httperror"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/logger"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
	"github.com/Manukesharwani09/photo-backup-manager/internal/planner"
)

// TO_DELETE_ALBUM_TITLE is the album, one per account, collecting cloud copies the user wants gone.
// The Library API cannot delete photos (or albums), so the user opens this album in Google Photos,
// selects all and deletes; the cloud check then notices they are gone. The emptied album is reused.
const TO_DELETE_ALBUM_TITLE = "🗑 To delete"

const MANUAL_ACTION = "Open each album in Google Photos, select all, and delete. Google does not let apps delete photos."

type SimilarService interface {
	Groups(ctx context.Context) (GroupsResponse, error)
	// MarkToDelete puts the cloud copies of the chosen photos into each account's "To delete" album.
	MarkToDelete(ctx context.Context, req ToDeleteRequest) (ToDeleteResponse, error)
}

type similarService struct {
	repo   *repository.Store
	access *account.Access
	userID int64
}

func NewSimilarService(repo *repository.Store, access *account.Access, userID int64) SimilarService {
	return &similarService{repo: repo, access: access, userID: userID}
}

func (s *similarService) Groups(ctx context.Context) (GroupsResponse, error) {
	inputs, err := s.repo.SimilarInputs(ctx, s.userID)
	if err != nil {
		return GroupsResponse{}, err
	}
	groups := planner.SimilarGroups(inputs)
	var ids []int64
	for _, g := range groups {
		ids = append(ids, g.MediaIDs...)
	}
	details, err := s.repo.MediaDetails(ctx, s.userID, ids)
	if err != nil {
		return GroupsResponse{}, err
	}
	out := GroupsResponse{Groups: []Group{}}
	for _, g := range groups {
		gj := Group{Reason: g.Reason}
		for _, id := range g.MediaIDs {
			if d := details[id]; d != nil {
				gj.Photos = append(gj.Photos, d)
			}
		}
		out.Groups = append(out.Groups, gj)
	}
	return out, nil
}

func (s *similarService) MarkToDelete(ctx context.Context, req ToDeleteRequest) (ToDeleteResponse, error) {
	if len(req.MediaIDs) == 0 {
		return ToDeleteResponse{}, httperror.NewHTTPError(httperror.BAD_REQUEST, "media_ids must not be empty")
	}
	items, err := s.repo.LiveBackups(ctx, s.userID, req.MediaIDs)
	if err != nil {
		return ToDeleteResponse{}, err
	}
	byAccount := map[int64][]repository.ToDeleteItem{}
	var order []int64
	for _, it := range items {
		if byAccount[it.AccountID] == nil {
			order = append(order, it.AccountID)
		}
		byAccount[it.AccountID] = append(byAccount[it.AccountID], it)
	}
	out := ToDeleteResponse{Accounts: []AccountCard{}, NotInGoogle: len(uniqInt(req.MediaIDs)) - countMedia(items), ManualAction: MANUAL_ACTION}

	for _, acctID := range order {
		acct, client, err := s.access.ClientFor(ctx, acctID)
		if err != nil {
			return ToDeleteResponse{}, err
		}
		_, albumID, err := backup.EnsureAlbum(ctx, s.repo, client, acctID, repository.ToDeleteFolder, TO_DELETE_ALBUM_TITLE)
		if err != nil {
			return ToDeleteResponse{}, err
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
				return ToDeleteResponse{}, err
			}
		}
		if err := s.repo.MarkForDelete(ctx, mediaIDs); err != nil {
			return ToDeleteResponse{}, err
		}
		link, _ := s.repo.AlbumLink(ctx, acctID, repository.ToDeleteFolder)
		out.Accounts = append(out.Accounts, AccountCard{AccountID: acctID, AccountEmail: acct.Email, AlbumURL: link, Added: len(list)})
		logger.FromContext(ctx).Info("photos added to To delete album", "account_id", acctID, "count", len(list))
	}
	pending, err := s.repo.PendingDeletes(ctx, s.userID)
	if err != nil {
		return ToDeleteResponse{}, err
	}
	for i := range out.Accounts {
		out.Accounts[i].Waiting = pending[out.Accounts[i].AccountID]
	}
	return out, nil
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
