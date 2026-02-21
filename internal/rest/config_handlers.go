package rest

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/KilimcininKorOglu/oba/internal/config"
)

// HandleGetPublicConfig handles GET /api/v1/config/public (no auth required)
func (h *Handlers) HandleGetPublicConfig(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	if h.configManager == nil {
		writeError(w, http.StatusServiceUnavailable, "config_not_configured", "config manager not configured")
		return
	}

	cfg := h.configManager.GetConfig()
	publicConfig := map[string]interface{}{
		"baseDN": cfg.Directory.BaseDN,
	}

	writeJSON(w, http.StatusOK, publicConfig)
}

// HandleGetConfig handles GET /api/v1/config
func (h *Handlers) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	if h.configManager == nil {
		writeError(w, http.StatusServiceUnavailable, "config_not_configured", "config manager not configured")
		return
	}

	configJSON := h.configManager.ToJSON()
	writeJSON(w, http.StatusOK, configJSON)
}

// HandleGetConfigSection handles GET /api/v1/config/{section}
func (h *Handlers) HandleGetConfigSection(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	if h.configManager == nil {
		writeError(w, http.StatusServiceUnavailable, "config_not_configured", "config manager not configured")
		return
	}

	section := extractConfigSection(r.URL.Path)
	if section == "" {
		writeError(w, http.StatusBadRequest, "invalid_section", "section name required")
		return
	}

	data, err := h.configManager.GetSection(section)
	if err != nil {
		writeError(w, http.StatusNotFound, "section_not_found", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, data)
}

// HandleUpdateConfigSection handles PATCH /api/v1/config/{section}
func (h *Handlers) HandleUpdateConfigSection(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	if h.configManager == nil {
		writeError(w, http.StatusServiceUnavailable, "config_not_configured", "config manager not configured")
		return
	}

	section := extractConfigSection(r.URL.Path)
	if section == "" {
		writeError(w, http.StatusBadRequest, "invalid_section", "section name required")
		return
	}

	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if err := h.configManager.UpdateSection(section, data); err != nil {
		writeError(w, http.StatusBadRequest, "update_failed", err.Error())
		return
	}

	// Get updated section
	updated, _ := h.configManager.GetSection(section)

	h.auditLog(r, "config updated", "section", section)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "config section updated",
		"section": section,
		"config":  updated,
	})
}

// HandleReloadConfig handles POST /api/v1/config/reload
func (h *Handlers) HandleReloadConfig(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	if h.configManager == nil {
		writeError(w, http.StatusServiceUnavailable, "config_not_configured", "config manager not configured")
		return
	}

	if h.configManager.GetConfigFile() == "" {
		writeError(w, http.StatusBadRequest, "reload_not_supported", "config reload requires file-based configuration")
		return
	}

	if err := h.configManager.Reload(); err != nil {
		writeError(w, http.StatusInternalServerError, "reload_failed", err.Error())
		return
	}

	h.auditLog(r, "config reloaded")
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "config reloaded",
	})
}

// HandleSaveConfig handles POST /api/v1/config/save
func (h *Handlers) HandleSaveConfig(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	if h.configManager == nil {
		writeError(w, http.StatusServiceUnavailable, "config_not_configured", "config manager not configured")
		return
	}

	if h.configManager.GetConfigFile() == "" {
		writeError(w, http.StatusBadRequest, "save_not_supported", "config save requires file-based configuration")
		return
	}

	if err := h.configManager.SaveToFile(); err != nil {
		writeError(w, http.StatusInternalServerError, "save_failed", err.Error())
		return
	}

	h.auditLog(r, "config saved to file", "filePath", h.configManager.GetConfigFile())
	writeJSON(w, http.StatusOK, map[string]string{
		"message":  "config saved",
		"filePath": h.configManager.GetConfigFile(),
	})
}

// HandleValidateConfig handles POST /api/v1/config/validate
func (h *Handlers) HandleValidateConfig(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	var req config.ConfigJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	// For validation, we just check if the structure is valid
	// Full validation would require converting JSON back to Config
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":   true,
		"message": "config structure is valid",
	})
}

// extractConfigSection extracts section name from path.
func extractConfigSection(path string) string {
	prefix := "/api/v1/config/"
	if strings.HasPrefix(path, prefix) {
		section := strings.TrimPrefix(path, prefix)
		// Remove trailing slash if any
		section = strings.TrimSuffix(section, "/")
		return section
	}
	return ""
}
