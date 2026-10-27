package backup

import (
	"net/http"

	"github.com/Manukesharwani09/photo-backup-manager/internal/common"
)

type BackupController struct {
	service BackupService
}

func NewBackupController(service BackupService) *BackupController {
	return &BackupController{service: service}
}

// CreateDevice: POST /v1/devices.
func (c *BackupController) CreateDevice(w http.ResponseWriter, r *http.Request) error {
	var req CreateDeviceRequest
	if err := common.DecodeJSON(w, r, &req); err != nil {
		return err
	}
	out, err := c.service.CreateDevice(r.Context(), req)
	if err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusCreated, out)
	return nil
}

// Sync: POST /v1/sync.
func (c *BackupController) Sync(w http.ResponseWriter, r *http.Request) error {
	var req SyncRequest
	if err := common.DecodeJSON(w, r, &req); err != nil {
		return err
	}
	out, err := c.service.Sync(r.Context(), req)
	if err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusOK, out)
	return nil
}

// CompleteUploads: POST /v1/uploads.
func (c *BackupController) CompleteUploads(w http.ResponseWriter, r *http.Request) error {
	var req UploadsRequest
	if err := common.DecodeJSON(w, r, &req); err != nil {
		return err
	}
	out, err := c.service.CompleteUploads(r.Context(), req)
	if err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusOK, out)
	return nil
}
