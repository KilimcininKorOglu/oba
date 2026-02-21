package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/acl"
	"github.com/KilimcininKorOglu/oba/internal/raft"
)

// ACLRuleJSON represents an ACL rule in JSON format.
type ACLRuleJSON struct {
	Target     string   `json:"target"`
	Subject    string   `json:"subject"`
	Scope      string   `json:"scope"`
	Rights     []string `json:"rights"`
	Attributes []string `json:"attributes,omitempty"`
	Deny       bool     `json:"deny"`
}

// ACLConfigJSON represents ACL configuration in JSON format.
type ACLConfigJSON struct {
	DefaultPolicy string        `json:"defaultPolicy"`
	Rules         []ACLRuleJSON `json:"rules"`
	Stats         *ACLStatsJSON `json:"stats,omitempty"`
}

// ACLStatsJSON represents ACL statistics.
type ACLStatsJSON struct {
	RuleCount   int       `json:"ruleCount"`
	LastReload  time.Time `json:"lastReload"`
	ReloadCount uint64    `json:"reloadCount"`
	FilePath    string    `json:"filePath,omitempty"`
}

// HandleGetACL handles GET /api/v1/acl
func (h *Handlers) HandleGetACL(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	if h.aclManager == nil {
		writeError(w, http.StatusServiceUnavailable, "acl_not_configured", "ACL manager not configured")
		return
	}

	config := h.aclManager.GetConfig()
	stats := h.aclManager.Stats()

	response := ACLConfigJSON{
		DefaultPolicy: config.DefaultPolicy,
		Rules:         aclRulesToJSON(config.Rules),
		Stats: &ACLStatsJSON{
			RuleCount:   stats.RuleCount,
			LastReload:  stats.LastReload,
			ReloadCount: stats.ReloadCount,
			FilePath:    stats.FilePath,
		},
	}

	writeJSON(w, http.StatusOK, response)
}

// HandleGetACLRules handles GET /api/v1/acl/rules
func (h *Handlers) HandleGetACLRules(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	if h.aclManager == nil {
		writeError(w, http.StatusServiceUnavailable, "acl_not_configured", "ACL manager not configured")
		return
	}

	rules := h.aclManager.GetRules()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rules": aclRulesToJSON(rules),
		"count": len(rules),
	})
}

// HandleGetACLRule handles GET /api/v1/acl/rules/{index}
func (h *Handlers) HandleGetACLRule(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	if h.aclManager == nil {
		writeError(w, http.StatusServiceUnavailable, "acl_not_configured", "ACL manager not configured")
		return
	}

	indexStr := extractPathParam(r.URL.Path, "/api/v1/acl/rules/")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_index", "invalid rule index")
		return
	}

	rule, err := h.aclManager.GetRule(index)
	if err != nil {
		writeError(w, http.StatusNotFound, "rule_not_found", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, aclRuleToJSON(rule))
}

// HandleAddACLRule handles POST /api/v1/acl/rules
func (h *Handlers) HandleAddACLRule(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	if h.aclManager == nil {
		writeError(w, http.StatusServiceUnavailable, "acl_not_configured", "ACL manager not configured")
		return
	}

	var req struct {
		Rule  ACLRuleJSON `json:"rule"`
		Index int         `json:"index"`
	}
	req.Index = -1

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	rule, err := jsonToACLRule(&req.Rule)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_rule", err.Error())
		return
	}

	// In cluster mode, route through Raft
	if h.clusterBackend != nil {
		if !h.clusterBackend.IsLeader() {
			writeError(w, http.StatusServiceUnavailable, "not_leader",
				"not leader, redirect to: "+h.clusterBackend.LeaderAddr())
			return
		}

		ruleData := aclRuleToRaftData(&req.Rule)
		version := h.aclManager.GetVersion() + 1

		if err := h.clusterBackend.ProposeACLAddRule(ruleData, req.Index, version); err != nil {
			writeError(w, http.StatusInternalServerError, "replication_failed", err.Error())
			return
		}

		h.auditLog(r, "ACL rule added via Raft", "target", rule.Target, "subject", rule.Subject)
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"message":    "rule added and replicated",
			"rule":       aclRuleToJSON(rule),
			"replicated": true,
		})
		return
	}

	// Standalone mode - direct add
	if err := h.aclManager.AddRule(rule, req.Index); err != nil {
		writeError(w, http.StatusBadRequest, "add_failed", err.Error())
		return
	}

	h.auditLog(r, "ACL rule added", "target", rule.Target, "subject", rule.Subject)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "rule added",
		"rule":    aclRuleToJSON(rule),
	})
}

// HandleUpdateACLRule handles PUT /api/v1/acl/rules/{index}
func (h *Handlers) HandleUpdateACLRule(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	if h.aclManager == nil {
		writeError(w, http.StatusServiceUnavailable, "acl_not_configured", "ACL manager not configured")
		return
	}

	indexStr := extractPathParam(r.URL.Path, "/api/v1/acl/rules/")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_index", "invalid rule index")
		return
	}

	var req ACLRuleJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	rule, err := jsonToACLRule(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_rule", err.Error())
		return
	}

	// In cluster mode, route through Raft
	if h.clusterBackend != nil {
		if !h.clusterBackend.IsLeader() {
			writeError(w, http.StatusServiceUnavailable, "not_leader",
				"not leader, redirect to: "+h.clusterBackend.LeaderAddr())
			return
		}

		ruleData := aclRuleToRaftData(&req)
		version := h.aclManager.GetVersion() + 1

		if err := h.clusterBackend.ProposeACLUpdateRule(ruleData, index, version); err != nil {
			writeError(w, http.StatusInternalServerError, "replication_failed", err.Error())
			return
		}

		h.auditLog(r, "ACL rule updated via Raft", "index", index, "target", rule.Target)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message":    "rule updated and replicated",
			"rule":       aclRuleToJSON(rule),
			"replicated": true,
		})
		return
	}

	// Standalone mode - direct update
	if err := h.aclManager.UpdateRule(index, rule); err != nil {
		writeError(w, http.StatusBadRequest, "update_failed", err.Error())
		return
	}

	h.auditLog(r, "ACL rule updated", "index", index, "target", rule.Target, "subject", rule.Subject)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "rule updated",
		"rule":    aclRuleToJSON(rule),
	})
}

// HandleDeleteACLRule handles DELETE /api/v1/acl/rules/{index}
func (h *Handlers) HandleDeleteACLRule(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	if h.aclManager == nil {
		writeError(w, http.StatusServiceUnavailable, "acl_not_configured", "ACL manager not configured")
		return
	}

	indexStr := extractPathParam(r.URL.Path, "/api/v1/acl/rules/")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_index", "invalid rule index")
		return
	}

	// In cluster mode, route through Raft
	if h.clusterBackend != nil {
		if !h.clusterBackend.IsLeader() {
			writeError(w, http.StatusServiceUnavailable, "not_leader",
				"not leader, redirect to: "+h.clusterBackend.LeaderAddr())
			return
		}

		version := h.aclManager.GetVersion() + 1

		if err := h.clusterBackend.ProposeACLDeleteRule(index, version); err != nil {
			writeError(w, http.StatusInternalServerError, "replication_failed", err.Error())
			return
		}

		h.auditLog(r, "ACL rule deleted via Raft", "index", index)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message":    "rule deleted and replicated",
			"replicated": true,
		})
		return
	}

	// Standalone mode - direct delete
	if err := h.aclManager.DeleteRule(index); err != nil {
		writeError(w, http.StatusNotFound, "delete_failed", err.Error())
		return
	}

	h.auditLog(r, "ACL rule deleted", "index", index)
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "rule deleted",
	})
}

// HandleSetDefaultPolicy handles PUT /api/v1/acl/default
func (h *Handlers) HandleSetDefaultPolicy(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	if h.aclManager == nil {
		writeError(w, http.StatusServiceUnavailable, "acl_not_configured", "ACL manager not configured")
		return
	}

	var req struct {
		Policy string `json:"policy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	// In cluster mode, route through Raft
	if h.clusterBackend != nil {
		if !h.clusterBackend.IsLeader() {
			writeError(w, http.StatusServiceUnavailable, "not_leader",
				"not leader, redirect to: "+h.clusterBackend.LeaderAddr())
			return
		}

		version := h.aclManager.GetVersion() + 1

		if err := h.clusterBackend.ProposeACLSetDefault(req.Policy, version); err != nil {
			writeError(w, http.StatusInternalServerError, "replication_failed", err.Error())
			return
		}

		h.auditLog(r, "ACL default policy changed via Raft", "policy", req.Policy)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message":    "default policy updated and replicated",
			"policy":     req.Policy,
			"replicated": true,
		})
		return
	}

	// Standalone mode - direct update
	if err := h.aclManager.SetDefaultPolicy(req.Policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_policy", err.Error())
		return
	}

	h.auditLog(r, "ACL default policy changed", "policy", req.Policy)
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "default policy updated",
		"policy":  req.Policy,
	})
}

// HandleReloadACL handles POST /api/v1/acl/reload
func (h *Handlers) HandleReloadACL(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	if h.aclManager == nil {
		writeError(w, http.StatusServiceUnavailable, "acl_not_configured", "ACL manager not configured")
		return
	}

	if !h.aclManager.IsFileMode() {
		writeError(w, http.StatusBadRequest, "reload_not_supported", "ACL reload requires file-based configuration")
		return
	}

	if err := h.aclManager.Reload(); err != nil {
		writeError(w, http.StatusInternalServerError, "reload_failed", err.Error())
		return
	}

	stats := h.aclManager.Stats()
	h.auditLog(r, "ACL reloaded", "ruleCount", stats.RuleCount)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":     "ACL reloaded",
		"ruleCount":   stats.RuleCount,
		"reloadCount": stats.ReloadCount,
	})
}

// HandleSaveACL handles POST /api/v1/acl/save
func (h *Handlers) HandleSaveACL(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	if h.aclManager == nil {
		writeError(w, http.StatusServiceUnavailable, "acl_not_configured", "ACL manager not configured")
		return
	}

	if !h.aclManager.IsFileMode() {
		writeError(w, http.StatusBadRequest, "save_not_supported", "ACL save requires file-based configuration")
		return
	}

	if err := h.aclManager.SaveToFile(); err != nil {
		writeError(w, http.StatusInternalServerError, "save_failed", err.Error())
		return
	}

	h.auditLog(r, "ACL saved to file", "filePath", h.aclManager.FilePath())
	writeJSON(w, http.StatusOK, map[string]string{
		"message":  "ACL saved",
		"filePath": h.aclManager.FilePath(),
	})
}

// HandleValidateACL handles POST /api/v1/acl/validate
func (h *Handlers) HandleValidateACL(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	var req ACLConfigJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	config := acl.NewConfig()
	config.SetDefaultPolicy(req.DefaultPolicy)

	var errors []string
	for i, ruleJSON := range req.Rules {
		rule, err := jsonToACLRule(&ruleJSON)
		if err != nil {
			errors = append(errors, "rule "+strconv.Itoa(i)+": "+err.Error())
			continue
		}
		if err := acl.ValidateRule(rule); err != nil {
			errors = append(errors, "rule "+strconv.Itoa(i)+": "+err.Error())
		}
		config.AddRule(rule)
	}

	if validationErrs := acl.ValidateConfig(config); len(validationErrs) > 0 {
		for _, e := range validationErrs {
			errors = append(errors, e.Error())
		}
	}

	if len(errors) > 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"valid":  false,
			"errors": errors,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":   true,
		"message": "ACL configuration is valid",
	})
}

// aclRulesToJSON converts ACL rules to JSON format.
func aclRulesToJSON(rules []*acl.ACL) []ACLRuleJSON {
	result := make([]ACLRuleJSON, len(rules))
	for i, rule := range rules {
		result[i] = *aclRuleToJSON(rule)
	}
	return result
}

// aclRuleToJSON converts a single ACL rule to JSON format.
func aclRuleToJSON(rule *acl.ACL) *ACLRuleJSON {
	return &ACLRuleJSON{
		Target:     rule.Target,
		Subject:    rule.Subject,
		Scope:      rule.Scope.String(),
		Rights:     rightsToStrings(rule.Rights),
		Attributes: rule.Attributes,
		Deny:       rule.Deny,
	}
}

// jsonToACLRule converts JSON to ACL rule.
func jsonToACLRule(j *ACLRuleJSON) (*acl.ACL, error) {
	rights, err := acl.ParseRights(j.Rights)
	if err != nil {
		return nil, err
	}

	rule := acl.NewACL(j.Target, j.Subject, rights)

	if j.Scope != "" {
		scope, err := acl.ParseScope(j.Scope)
		if err != nil {
			return nil, err
		}
		rule.WithScope(scope)
	}

	if len(j.Attributes) > 0 {
		rule.WithAttributes(j.Attributes...)
	}

	rule.WithDeny(j.Deny)

	return rule, nil
}

// rightsToStrings converts Right flags to string slice.
func rightsToStrings(r acl.Right) []string {
	if r == acl.All {
		return []string{"all"}
	}

	var rights []string
	if r.Has(acl.Read) {
		rights = append(rights, "read")
	}
	if r.Has(acl.Write) {
		rights = append(rights, "write")
	}
	if r.Has(acl.Add) {
		rights = append(rights, "add")
	}
	if r.Has(acl.Delete) {
		rights = append(rights, "delete")
	}
	if r.Has(acl.Search) {
		rights = append(rights, "search")
	}
	if r.Has(acl.Compare) {
		rights = append(rights, "compare")
	}
	return rights
}

// extractPathParam extracts a path parameter after the given prefix.
func extractPathParam(path, prefix string) string {
	if strings.HasPrefix(path, prefix) {
		return strings.TrimPrefix(path, prefix)
	}
	return ""
}

// aclRuleToRaftData converts ACLRuleJSON to raft.ACLRuleData for Raft replication.
func aclRuleToRaftData(j *ACLRuleJSON) *raft.ACLRuleData {
	return &raft.ACLRuleData{
		Target:     j.Target,
		Subject:    j.Subject,
		Scope:      j.Scope,
		Rights:     j.Rights,
		Attributes: j.Attributes,
		Deny:       j.Deny,
	}
}
