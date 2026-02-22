package rest

import (
	"encoding/json"
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
	PendingCount int            `json:"pending_entries"`
	RetryCount   int            `json:"retry_buffer_size"`
	RetryRunning bool           `json:"retry_running"`
	ClusterMode  bool           `json:"cluster_mode"`
	WriterReady  bool           `json:"writer_ready"`
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

	// Include archived logs if requested
	if includeArchive := r.URL.Query().Get("include_archive"); includeArchive == "true" || includeArchive == "1" {
		opts.IncludeArchive = true
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
		PendingCount: stats.PendingCount,
		RetryCount:   stats.RetryCount,
		RetryRunning: stats.RetryRunning,
		ClusterMode:  stats.ClusterMode,
		WriterReady:  stats.WriterReady,
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

	h.auditLog(r, "logs cleared")
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

	h.auditLog(r, "logs exported", "format", format)

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

// HandleGetLogArchives handles GET /api/v1/logs/archives
func (h *Handlers) HandleGetLogArchives(w http.ResponseWriter, r *http.Request) {
	store := h.getLogStore()
	if store == nil {
		writeError(w, http.StatusServiceUnavailable, "log_store_disabled", "log storage is not enabled")
		return
	}

	archives, err := store.ListArchives()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}

	if archives == nil {
		archives = []logging.ArchiveFile{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"archives": archives,
		"count":    len(archives),
	})
}

// HandleGetLogArchiveStats handles GET /api/v1/logs/archives/stats
func (h *Handlers) HandleGetLogArchiveStats(w http.ResponseWriter, r *http.Request) {
	store := h.getLogStore()
	if store == nil {
		writeError(w, http.StatusServiceUnavailable, "log_store_disabled", "log storage is not enabled")
		return
	}

	stats := store.GetArchiveStats()
	if stats == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled":        true,
		"archive_count":  stats.ArchiveCount,
		"total_size":     stats.TotalSize,
		"total_entries":  stats.TotalEntries,
		"oldest_archive": stats.OldestArchive,
		"newest_archive": stats.NewestArchive,
	})
}

// HandleArchiveLogsNow handles POST /api/v1/logs/archive
func (h *Handlers) HandleArchiveLogsNow(w http.ResponseWriter, r *http.Request) {
	store := h.getLogStore()
	if store == nil {
		writeError(w, http.StatusServiceUnavailable, "log_store_disabled", "log storage is not enabled")
		return
	}

	archiveFile, err := store.ArchiveNow()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "archive_failed", err.Error())
		return
	}

	h.auditLog(r, "logs archived manually")

	if archiveFile == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "no logs to archive",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"archive": archiveFile,
	})
}

// HandleCleanupArchives handles POST /api/v1/logs/archives/cleanup
func (h *Handlers) HandleCleanupArchives(w http.ResponseWriter, r *http.Request) {
	store := h.getLogStore()
	if store == nil {
		writeError(w, http.StatusServiceUnavailable, "log_store_disabled", "log storage is not enabled")
		return
	}

	deleted, err := store.CleanupOldArchives()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cleanup_failed", err.Error())
		return
	}

	h.auditLog(r, "old archives cleaned up", "deleted", deleted)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"deleted": deleted,
	})
}

// SetLogger sets the logger for log-related endpoints.
func (h *Handlers) SetLogger(logger logging.Logger) {
	h.logger = logger
}

// InternalLogRequest represents a log entry forwarded from another node.
type InternalLogRequest struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source,omitempty"`
	User      string                 `json:"user,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// HandleInternalLog handles POST /api/v1/internal/log
// This endpoint receives log entries forwarded from follower nodes.
func (h *Handlers) HandleInternalLog(w http.ResponseWriter, r *http.Request) {
	// Only accept from cluster nodes (no auth required for internal traffic)
	if h.clusterBackend == nil {
		writeError(w, http.StatusBadRequest, "not_cluster_mode", "server is not in cluster mode")
		return
	}

	// Only leader should accept forwarded logs
	if !h.clusterBackend.IsLeader() {
		writeError(w, http.StatusServiceUnavailable, "not_leader", "only leader accepts forwarded logs")
		return
	}

	var req InternalLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// Write log via logger (which will go through Raft)
	if h.logger != nil {
		logger := h.logger
		if req.Source != "" {
			logger = logger.WithSource(req.Source)
		}
		if req.User != "" {
			logger = logger.WithUser(req.User)
		}
		if req.RequestID != "" {
			logger = logger.WithRequestID(req.RequestID)
		}

		// Convert fields to key-value pairs
		var keyvals []interface{}
		for k, v := range req.Fields {
			keyvals = append(keyvals, k, v)
		}

		// Log based on level
		switch req.Level {
		case "debug":
			logger.Debug(req.Message, keyvals...)
		case "warn", "warning":
			logger.Warn(req.Message, keyvals...)
		case "error":
			logger.Error(req.Message, keyvals...)
		default:
			logger.Info(req.Message, keyvals...)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}
