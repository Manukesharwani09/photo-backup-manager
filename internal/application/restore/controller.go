package restore

import (
	"net/http"

	"github.com/Manukesharwani09/photo-backup-manager/internal/common"
)

type RestoreController struct {
	service RestoreService
}

func NewRestoreController(service RestoreService) *RestoreController {
	return &RestoreController{service: service}
}

// Plan: POST /v1/restore/plan.
func (c *RestoreController) Plan(w http.ResponseWriter, r *http.Request) error {
	var req PlanRequest
	if err := common.DecodeJSON(w, r, &req); err != nil {
		return err
	}
	out, err := c.service.Plan(r.Context(), req)
	if err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusOK, out)
	return nil
}

// Done: POST /v1/restore/done.
func (c *RestoreController) Done(w http.ResponseWriter, r *http.Request) error {
	var req DoneRequest
	if err := common.DecodeJSON(w, r, &req); err != nil {
		return err
	}
	out, err := c.service.Done(r.Context(), req)
	if err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusOK, out)
	return nil
}
