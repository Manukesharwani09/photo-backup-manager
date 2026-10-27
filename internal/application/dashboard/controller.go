package dashboard

import (
	"net/http"

	"github.com/Manukesharwani09/photo-backup-manager/internal/common"
)

type DashboardController struct {
	service DashboardService
}

func NewDashboardController(service DashboardService) *DashboardController {
	return &DashboardController{service: service}
}

// Dashboard: GET /v1/dashboard.
func (c *DashboardController) Dashboard(w http.ResponseWriter, r *http.Request) error {
	out, err := c.service.Dashboard(r.Context())
	if err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusOK, out)
	return nil
}

// Where: GET /v1/photos/where?month=YYYY-MM or ?from=YYYY-MM-DD&to=YYYY-MM-DD.
func (c *DashboardController) Where(w http.ResponseWriter, r *http.Request) error {
	out, err := c.service.Where(r.Context(), r.URL.Query())
	if err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusOK, out)
	return nil
}
