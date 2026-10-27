package rule

import (
	"net/http"

	"github.com/Manukesharwani09/photo-backup-manager/internal/common"
)

type RuleController struct {
	service RuleService
}

func NewRuleController(service RuleService) *RuleController {
	return &RuleController{service: service}
}

// List: GET /v1/rules.
func (c *RuleController) List(w http.ResponseWriter, r *http.Request) error {
	out, err := c.service.List(r.Context())
	if err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusOK, out)
	return nil
}

// Replace: PUT /v1/rules.
func (c *RuleController) Replace(w http.ResponseWriter, r *http.Request) error {
	var req []RuleDTO
	if err := common.DecodeJSON(w, r, &req); err != nil {
		return err
	}
	out, err := c.service.Replace(r.Context(), req)
	if err != nil {
		return err
	}
	common.WriteJSON(w, http.StatusOK, out)
	return nil
}
