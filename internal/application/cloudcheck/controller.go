package cloudcheck

import (
	"net/http"

	"github.com/Manukesharwani09/photo-backup-manager/internal/common"
)

type CloudCheckController struct {
	service CloudCheckService
}

func NewCloudCheckController(service CloudCheckService) *CloudCheckController {
	return &CloudCheckController{service: service}
}

// Check: POST /v1/cloud/check (it also runs on a schedule).
func (c *CloudCheckController) Check(w http.ResponseWriter, r *http.Request) error {
	out, err := c.service.Check(r.Context())
	if err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusOK, out)
	return nil
}
