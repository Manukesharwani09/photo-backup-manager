package appserver

import (
	"fmt"
	"net/http"
	"testing"
)

// After an account was full, the user frees space and switches back to it.
func TestAccountStateEndpoints_FullAvailableCurrent(t *testing.T) {
	e := setup(t)
	e.sync()
	full := fmt.Sprintf("/v1/accounts/%d/full", e.primary)
	avail := fmt.Sprintf("/v1/accounts/%d/available", e.primary)
	current := fmt.Sprintf("/v1/accounts/%d/current", e.primary)

	// Primary full: new photos go to overflow (period switches with reason "full").
	e.call("POST", full, nil, http.StatusOK, nil)
	if r := e.sync(file("DCIM", "a.jpg", "A")); len(r.Plan) != 1 || r.Plan[0].AccountID != e.over {
		t.Fatalf("plan while primary full: %+v", r.Plan)
	}
	// A full account can't be made current.
	e.call("POST", current, nil, http.StatusConflict, nil)

	// Space freed: available again, but overflow stays current until the user switches.
	e.call("POST", avail, nil, http.StatusOK, nil)
	if r := e.sync(file("DCIM", "b.jpg", "B")); len(r.Plan) != 1 || r.Plan[0].AccountID != e.over {
		t.Fatalf("after available, overflow should still receive: %+v", r.Plan)
	}
	// Manual switch back: period closed as "manual", primary receives again.
	e.call("POST", current, nil, http.StatusOK, nil)
	e.call("POST", current, nil, http.StatusOK, nil) // idempotent: no extra period
	if r := e.sync(file("DCIM", "c.jpg", "C")); r.Plan[len(r.Plan)-1].AccountID != e.primary {
		t.Fatalf("after make-current: %+v", r.Plan)
	}
	if n := e.scalar(`SELECT count(*) FROM account_period WHERE end_reason = 'manual'`); n != 1 {
		t.Fatalf("manual period endings: %d", n)
	}
	if n := e.scalar(`SELECT count(*) FROM account_period WHERE ended_at IS NULL`); n != 1 {
		t.Fatalf("open periods: %d", n)
	}
	// Unknown / bad ids.
	e.call("POST", "/v1/accounts/424242/available", nil, http.StatusNotFound, nil)
	e.call("POST", "/v1/accounts/424242/current", nil, http.StatusNotFound, nil)
	e.call("POST", "/v1/accounts/x/current", nil, http.StatusBadRequest, nil)
}
