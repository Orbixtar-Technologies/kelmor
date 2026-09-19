package httpserver

import (
	"net/http"
	"strconv"

	"github.com/hosting-panel/panel/agent/operations"
	"github.com/hosting-panel/panel/internal/rbac"
)

func (a *API) mailDeliveryReports(w http.ResponseWriter, r *http.Request) {
	if !a.requireAny(w, r, rbac.MailRead, rbac.AccountsRead) {
		return
	}
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	month, _ := strconv.Atoi(r.URL.Query().Get("month"))
	day, _ := strconv.Atoi(r.URL.Query().Get("day"))
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ReadMailDelivery",
		Params: mustJSON(operations.MailDeliveryParams{
			Mode:  "report",
			Year:  year,
			Month: month,
			Day:   day,
			Query: r.URL.Query().Get("q"),
		}),
	})
	if err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	writeJSON(w, 200, raw)
}

func (a *API) mailDeliveryTrack(w http.ResponseWriter, r *http.Request) {
	if !a.require(w, r, rbac.MailRead) {
		return
	}
	raw, err := a.Agent.Dispatch(r.Context(), operations.Request{
		Method: "ReadMailDelivery",
		Params: mustJSON(operations.MailDeliveryParams{
			Mode:  "track",
			Query: r.URL.Query().Get("q"),
		}),
	})
	if err != nil {
		a.fail(w, r, 400, "VALIDATION", err.Error(), false)
		return
	}
	writeJSON(w, 200, raw)
}
