package cleanup

import (
	"net/http"

	"github.com/Manukesharwani09/photo-backup-manager/internal/common"
)

type CleanupController struct {
	service CleanupService
}

func NewCleanupController(service CleanupService) *CleanupController {
	return &CleanupController{service: service}
}

// Plan: POST /v1/cleanup/plan.
func (c *CleanupController) Plan(w http.ResponseWriter, r *http.Request) error {
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

// Done: POST /v1/cleanup/done.
func (c *CleanupController) Done(w http.ResponseWriter, r *http.Request) error {
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
