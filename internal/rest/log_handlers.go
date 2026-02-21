package rest

import (
	"net/http"
	"strconv"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/logging"
)

// LogQueryRequest represents a log query request.
type LogQueryRequest struct {
	Level     string `json:"level,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
	Search    string `json:"search,omitempty"`
	Offset    int    `json:"offset,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

// LogQueryResponse represents a log query response.
type LogQueryResponse struct {
	Entries    []logging.LogEntry `json:"entries"`
	TotalCount int                `json:"total_count"`
	Offset     int                `json:"offset"`
	Limit      int                `json:"limit"`
	HasMore    bool               `json:"has_more"`
}

// LogStatsResponse represents log statistics response.
type LogStatsResponse struct {
	TotalEntries int            `json:"total_entries"`
	MaxEntries   int            `json:"max_entries"`
	ByLevel      map[string]int `json:"by_level"`
	OldestEntry  string         `json:"oldest_entry,omitempty"`
	NewestEntry  string         `json:"newest_entry,omitempty"`
}

// HandleGetLogs handles GET /api/v1/logs
func (h *Handlers) HandleGetLogs(w http.ResponseWriter, r *http.Request) {
	store := h.getLogStore()
	if store == nil {
		writeError(w, http.StatusServiceUnavailable, "log_store_disabled", "log storage is not enabled")
		return
	}

	opts := logging.QueryOptions{
		Level:     r.URL.Query().Get("level"),
		Source:    r.URL.Query().Get("source"),
		User:      r.URL.Query().Get("user"),
		RequestID: r.URL.Query().Get("request_id"),
		Search:    r.URL.Query().Get("search"),
	}

	if startTime := r.URL.Query().Get("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			opts.StartTime = t
		}
	}

	if endTime := r.URL.Query().Get("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			opts.EndTime = t
		}
	}

	if offset := r.URL.Query().Get("offset"); offset != "" {
		if n, err := strconv.Atoi(offset); err == nil {
			opts.Offset = n
		}
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		if n, err := strconv.Atoi(limit); err == nil {
			opts.Limit = n
		}
	} else {
		opts.Limit = 100
	}

	entries, total, err := store.Query(opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query_failed", err.Error())
		return
	}

	response := LogQueryResponse{
		Entries:    entries,
		TotalCount: total,
		Offset:     opts.Offset,
		Limit:      opts.Limit,
		HasMore:    opts.Offset+len(entries) < total,
	}

	writeJSON(w, http.StatusOK, response)
}

// HandleGetLogStats handles GET /api/v1/logs/stats
func (h *Handlers) HandleGetLogStats(w http.ResponseWriter, r *http.Request) {
	store := h.getLogStore()
	if store == nil {
		writeError(w, http.StatusServiceUnavailable, "log_store_disabled", "log storage is not enabled")
		return
	}

	stats := store.GetStats()

	response := LogStatsResponse{
		TotalEntries: stats.TotalEntries,
		MaxEntries:   stats.MaxEntries,
		ByLevel:      stats.ByLevel,
	}

	if !stats.OldestEntry.IsZero() {
		response.OldestEntry = stats.OldestEntry.Format(time.RFC3339)
	}
	if !stats.NewestEntry.IsZero() {
		response.NewestEntry = stats.NewestEntry.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, response)
}

// HandleClearLogs handles DELETE /api/v1/logs
func (h *Handlers) HandleClearLogs(w http.ResponseWriter, r *http.Request) {
	store := h.getLogStore()
	if store == nil {
		writeError(w, http.StatusServiceUnavailable, "log_store_disabled", "log storage is not enabled")
		return
	}

	if err := store.Clear(); err != nil {
		writeError(w, http.StatusInternalServerError, "clear_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "logs cleared"})
}

// HandleExportLogs handles GET /api/v1/logs/export
func (h *Handlers) HandleExportLogs(w http.ResponseWriter, r *http.Request) {
	store := h.getLogStore()
	if store == nil {
		writeError(w, http.StatusServiceUnavailable, "log_store_disabled", "log storage is not enabled")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	opts := logging.QueryOptions{
		Level:     r.URL.Query().Get("level"),
		Source:    r.URL.Query().Get("source"),
		User:      r.URL.Query().Get("user"),
		RequestID: r.URL.Query().Get("request_id"),
		Search:    r.URL.Query().Get("search"),
	}

	if startTime := r.URL.Query().Get("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			opts.StartTime = t
		}
	}

	if endTime := r.URL.Query().Get("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			opts.EndTime = t
		}
	}

	data, err := store.Export(format, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export_failed", err.Error())
		return
	}

	var contentType string
	switch format {
	case "csv":
		contentType = "text/csv"
	case "jsonl":
		contentType = "application/x-ndjson"
	default:
		contentType = "application/json"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=logs."+format)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// getLogStore returns the log store from the logger.
func (h *Handlers) getLogStore() *logging.LogStore {
	if h.logger == nil {
		return nil
	}
	return h.logger.GetStore()
}

// SetLogger sets the logger for log-related endpoints.
func (h *Handlers) SetLogger(logger logging.Logger) {
	h.logger = logger
}
