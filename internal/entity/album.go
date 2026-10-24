package entity

// Album is a Google Photos album in one account for one phone folder.
// FolderPath ":to-delete:" is the account's "🗑 To delete" album.
type Album struct {
	BaseModel
	AccountID      int64   `db:"account_id" json:"accountId"`
	FolderPath     string  `db:"folder_path" json:"folderPath"`
	Title          string  `db:"title" json:"title"`
	GPhotosAlbumID string  `db:"gphotos_album_id" json:"-"`
	ProductURL     *string `db:"product_url" json:"productUrl"`
}
