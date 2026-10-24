package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// PhotosAPI is the Library API base URL; tests point it at a fake server.
var PhotosAPI = "https://photoslibrary.googleapis.com/v1"

// Library API limits we design around.
const (
	MaxBatch          = 50   // newMediaItems per batchCreate, ids per batchGet
	MaxDescriptionLen = 1000 // characters
)

// UploadBytes sends raw bytes and returns an upload token (valid ~1 day). The phone calls this;
// the backend then turns the token into a media item with BatchCreate.
func UploadBytes(ctx context.Context, client *http.Client, fileName, mimeType string, data []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, PhotosAPI+"/uploads", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Goog-Upload-Content-Type", mimeType)
	req.Header.Set("X-Goog-Upload-Protocol", "raw")
	req.Header.Set("X-Goog-Upload-File-Name", fileName)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", &APIError{Op: "upload", Status: resp.StatusCode, Body: string(body)}
	}
	tok := strings.TrimSpace(string(body))
	if tok == "" {
		return "", fmt.Errorf("upload: empty upload token")
	}
	return tok, nil
}

// CreateAlbum makes an app-created album and returns its id.
func CreateAlbum(ctx context.Context, client *http.Client, title string) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	in := map[string]any{"album": map[string]string{"title": title}}
	if err := postJSON(ctx, client, PhotosAPI+"/albums", in, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", fmt.Errorf("create album: no id returned")
	}
	return out.ID, nil
}

// NewItem is one upload token to turn into a media item.
type NewItem struct {
	UploadToken string
	FileName    string
	Description string // original phone path; trimmed to MaxDescriptionLen
}

// CreateResult is Google's per-item outcome. Code 0 means success.
type CreateResult struct {
	UploadToken string
	Code        int
	Message     string
	MediaItemID string
	ProductURL  string
}

// BatchCreate creates up to MaxBatch media items, optionally adding them to albumID.
// A non-nil error means the whole call failed; per-item failures are in the results.
func BatchCreate(ctx context.Context, client *http.Client, albumID string, items []NewItem) ([]CreateResult, error) {
	if len(items) == 0 {
		return nil, nil
	}
	if len(items) > MaxBatch {
		return nil, fmt.Errorf("batchCreate: %d items exceeds %d", len(items), MaxBatch)
	}
	type simple struct {
		UploadToken string `json:"uploadToken"`
		FileName    string `json:"fileName,omitempty"`
	}
	type newItem struct {
		Description     string `json:"description,omitempty"`
		SimpleMediaItem simple `json:"simpleMediaItem"`
	}
	in := struct {
		AlbumID       string    `json:"albumId,omitempty"`
		NewMediaItems []newItem `json:"newMediaItems"`
	}{AlbumID: albumID}
	for _, it := range items {
		in.NewMediaItems = append(in.NewMediaItems, newItem{
			Description:     truncate(it.Description, MaxDescriptionLen),
			SimpleMediaItem: simple{UploadToken: it.UploadToken, FileName: it.FileName},
		})
	}
	var out struct {
		NewMediaItemResults []struct {
			UploadToken string `json:"uploadToken"`
			Status      struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"status"`
			MediaItem struct {
				ID         string `json:"id"`
				ProductURL string `json:"productUrl"`
			} `json:"mediaItem"`
		} `json:"newMediaItemResults"`
	}
	if err := postJSON(ctx, client, PhotosAPI+"/mediaItems:batchCreate", in, &out); err != nil {
		return nil, err
	}
	res := make([]CreateResult, 0, len(out.NewMediaItemResults))
	for _, r := range out.NewMediaItemResults {
		cr := CreateResult{UploadToken: r.UploadToken, Code: r.Status.Code, Message: r.Status.Message,
			MediaItemID: r.MediaItem.ID, ProductURL: r.MediaItem.ProductURL}
		if cr.MediaItemID == "" && cr.Code == 0 {
			cr.Code = -1 // no item and no error code: treat as failed
			if cr.Message == "" {
				cr.Message = "no media item returned"
			}
		}
		res = append(res, cr)
	}
	return res, nil
}

// MediaItem is the part of a Library API media item the backend uses.
type MediaItem struct {
	ID         string
	BaseURL    string // valid ~60 minutes; append "=d" (photo) or "=dv" (video) to download the file
	MimeType   string
	FileName   string
	ProductURL string
}

// DownloadURL is the link that returns the original file bytes.
func (m MediaItem) DownloadURL() string {
	if strings.HasPrefix(m.MimeType, "video/") {
		return m.BaseURL + "=dv"
	}
	return m.BaseURL + "=d"
}

// BatchGet returns the requested items Google still has (app-created only); missing ids are absent.
func BatchGet(ctx context.Context, client *http.Client, ids []string) (map[string]MediaItem, error) {
	if len(ids) > MaxBatch {
		return nil, fmt.Errorf("batchGet: %d ids exceeds %d", len(ids), MaxBatch)
	}
	q := url.Values{}
	for _, id := range ids {
		q.Add("mediaItemIds", id)
	}
	var out struct {
		MediaItemResults []struct {
			MediaItem struct {
				ID         string `json:"id"`
				BaseURL    string `json:"baseUrl"`
				MimeType   string `json:"mimeType"`
				Filename   string `json:"filename"`
				ProductURL string `json:"productUrl"`
			} `json:"mediaItem"`
		} `json:"mediaItemResults"`
	}
	if err := getJSON(ctx, client, PhotosAPI+"/mediaItems:batchGet?"+q.Encode(), &out); err != nil {
		return nil, err
	}
	found := make(map[string]MediaItem, len(ids))
	for _, r := range out.MediaItemResults {
		m := r.MediaItem
		if m.ID != "" {
			found[m.ID] = MediaItem{ID: m.ID, BaseURL: m.BaseURL, MimeType: m.MimeType, FileName: m.Filename, ProductURL: m.ProductURL}
		}
	}
	return found, nil
}

// IsStorageFull guesses whether a per-item failure means the account is out of storage.
// The exact error is unverified (open question A6), so this matches the documented
// RESOURCE_EXHAUSTED code (8) and storage/quota wording.
func IsStorageFull(code int, message string) bool {
	m := strings.ToLower(message)
	return code == 8 || strings.Contains(m, "storage") || strings.Contains(m, "quota exceeded") ||
		strings.Contains(m, "insufficient")
}

// APIError is a non-2xx response from a Google API.
type APIError struct {
	Op     string
	Status int
	Body   string
}

func (e *APIError) Error() string { return fmt.Sprintf("%s: HTTP %d: %s", e.Op, e.Status, e.Body) }

func postJSON(ctx context.Context, client *http.Client, url string, in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return &APIError{Op: "POST " + url, Status: resp.StatusCode, Body: string(body)}
	}
	return json.Unmarshal(body, out)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
