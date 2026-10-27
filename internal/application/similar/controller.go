package similar

import (
	"net/http"

	"github.com/Manukesharwani09/photo-backup-manager/internal/common"
)

type SimilarController struct {
	service SimilarService
}

func NewSimilarController(service SimilarService) *SimilarController {
	return &SimilarController{service: service}
}

// Groups: GET /v1/similar.
func (c *SimilarController) Groups(w http.ResponseWriter, r *http.Request) error {
	out, err := c.service.Groups(r.Context())
	if err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusOK, out)
	return nil
}

// MarkToDelete: POST /v1/cloud/to-delete.
func (c *SimilarController) MarkToDelete(w http.ResponseWriter, r *http.Request) error {
	var req ToDeleteRequest
	if err := common.DecodeJSON(w, r, &req); err != nil {
		return err
	}
	out, err := c.service.MarkToDelete(r.Context(), req)
	if err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusOK, out)
	return nil
}
