package similar

import "github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"

// Group is a set of look-alike photos (different files, same picture) with where each copy is.
type Group struct {
	Reason string                    `json:"reason"` // looks_alike | same_name_and_date
	Photos []*repository.MediaDetail `json:"photos"`
}

type GroupsResponse struct {
	Groups []Group `json:"groups"`
}

type ToDeleteRequest struct {
	MediaIDs []int64 `json:"media_ids"`
}

// AccountCard tells the user which album to open in Google Photos to finish the deletion.
type AccountCard struct {
	AccountID    int64  `json:"account_id"`
	AccountEmail string `json:"account_email"`
	AlbumURL     string `json:"album_url"`
	Added        int    `json:"added"`
	Waiting      int    `json:"waiting"` // all photos in this account marked and not yet deleted
}

type ToDeleteResponse struct {
	Accounts     []AccountCard `json:"accounts"`
	NotInGoogle  int           `json:"not_in_google"` // chosen photos with no cloud copy (nothing to do)
	ManualAction string        `json:"manual_action"`
}
