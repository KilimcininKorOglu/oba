package rest

import (
	"net/http"
	"strconv"
	"sync/atomic"
)

// HandleRepairUIDUniqueness handles POST /api/v1/cluster/repair/uid
// Query params:
// - dryRun=true|false (default false)
func (h *Handlers) HandleRepairUIDUniqueness(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	dryRun := false
	if raw := r.URL.Query().Get("dryRun"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_dry_run", "dryRun must be true or false")
			return
		}
		dryRun = parsed
	}

	if h.clusterBackend != nil && !h.clusterBackend.IsLeader() {
		writeError(w, http.StatusServiceUnavailable, "not_leader",
			"not leader, redirect to: "+h.clusterBackend.LeaderAddr())
		return
	}

	bindDN := BindDN(r)
	report, err := h.backend.RepairUIDUniqueness(dryRun, bindDN)
	if err != nil {
		status, code, msg := mapBackendError(err)
		writeError(w, status, code, msg)
		return
	}

	if dryRun {
		h.auditLog(r, "uid repair dry-run", "actions", len(report.Actions), "duplicates", report.DuplicateUIDs)
	} else {
		h.auditLog(r, "uid repair executed", "fixed", report.Fixed, "failed", report.Failed, "duplicates", report.DuplicateUIDs)
	}
	writeJSON(w, http.StatusOK, report)
}
