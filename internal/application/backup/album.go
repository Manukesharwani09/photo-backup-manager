package backup

import (
	"context"
	"errors"
	"net/http"

	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/client/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
)

// EnsureAlbum returns the album stored under key in an account, creating it in Google on first use.
// key is a phone folder, or repository.ToDeleteFolder for the "To delete" album.
func EnsureAlbum(ctx context.Context, repo *repository.Store, client *http.Client, accountID int64, key, title string) (rowID int64, googleID string, err error) {
	rowID, googleID, err = repo.Album(ctx, accountID, key)
	if err == nil {
		return rowID, googleID, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return 0, "", err
	}
	googleID, link, err := google.CreateAlbum(ctx, client, title)
	if err != nil {
		return 0, "", err
	}
	rowID, err = repo.SaveAlbum(ctx, accountID, key, title, googleID, link)
	return rowID, googleID, err
}
