package acl

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/logging"
	"github.com/KilimcininKorOglu/oba/internal/raft"
)

// Manager manages ACL configuration with hot reload support.
type Manager struct {
	mu        sync.RWMutex
	config    *Config
	evaluator *Evaluator
	filePath  string
	logger    logging.Logger

	// Metrics
	reloadCount   uint64
	lastReload    time.Time
	lastError     error
	lastErrorTime time.Time

	// Version for Raft sync
	version uint64
}

// ManagerConfig holds configuration for ACLManager.
type ManagerConfig struct {
	// FilePath is the path to the ACL YAML file.
	// If empty, uses embedded config.
	FilePath string

	// EmbeddedConfig is the ACL config from main config.yaml.
	// Used when FilePath is empty.
	EmbeddedConfig *Config

	// Logger for logging reload events.
	Logger logging.Logger
}

// NewManager creates a new ACL manager.
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	if cfg == nil {
		cfg = &ManagerConfig{}
	}

	m := &Manager{
		logger:     cfg.Logger,
		lastReload: time.Now(),
	}

	if cfg.FilePath != "" {
		// Load from file
		m.filePath = cfg.FilePath
		config, err := LoadFromFile(cfg.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load ACL file: %w", err)
		}
		m.config = config
		m.logInfo("ACL loaded from file",
			"file", cfg.FilePath,
			"rules", len(config.Rules),
			"defaultPolicy", config.DefaultPolicy,
		)
	} else if cfg.EmbeddedConfig != nil {
		// Use embedded config
		m.config = cfg.EmbeddedConfig
		m.logInfo("ACL loaded from embedded config",
			"rules", len(cfg.EmbeddedConfig.Rules),
			"defaultPolicy", cfg.EmbeddedConfig.DefaultPolicy,
		)
	} else {
		// Default config
		m.config = NewConfig()
		m.logInfo("ACL using default config", "defaultPolicy", m.config.DefaultPolicy)
	}

	m.evaluator = NewEvaluator(m.config)

	return m, nil
}

// Reload reloads ACL configuration from file.
// Returns error if reload fails; old config is preserved.
func (m *Manager) Reload() error {
	if m.filePath == "" {
		return fmt.Errorf("no ACL file configured; reload not supported")
	}

	m.logInfo("reloading ACL configuration", "file", m.filePath)

	// Load new config
	newConfig, err := LoadFromFile(m.filePath)
	if err != nil {
		m.mu.Lock()
		m.lastError = err
		m.lastErrorTime = time.Now()
		m.mu.Unlock()
		m.logError("ACL reload failed: load error", "error", err)
		return fmt.Errorf("failed to load ACL file: %w", err)
	}

	// Validate new config
	if errs := ValidateConfig(newConfig); len(errs) > 0 {
		m.mu.Lock()
		m.lastError = errs[0]
		m.lastErrorTime = time.Now()
		m.mu.Unlock()
		m.logError("ACL reload failed: validation error", "error", errs[0])
		return fmt.Errorf("ACL validation failed: %v", errs[0])
	}

	// Atomic swap
	m.mu.Lock()
	oldRuleCount := len(m.config.Rules)
	m.config = newConfig
	m.evaluator = NewEvaluator(newConfig)
	m.lastReload = time.Now()
	m.lastError = nil
	m.mu.Unlock()

	atomic.AddUint64(&m.reloadCount, 1)

	m.logInfo("ACL configuration reloaded",
		"oldRules", oldRuleCount,
		"newRules", len(newConfig.Rules),
		"defaultPolicy", newConfig.DefaultPolicy,
	)

	return nil
}

// GetEvaluator returns the current ACL evaluator.
// Thread-safe for concurrent access.
func (m *Manager) GetEvaluator() *Evaluator {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.evaluator
}

// GetConfig returns the current ACL configuration.
// Thread-safe for concurrent access.
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// CheckAccess checks if access is allowed.
// Thread-safe wrapper around evaluator.
func (m *Manager) CheckAccess(ctx *AccessContext) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.evaluator.CheckAccess(ctx)
}

// CheckAttributeAccess checks if access to a specific attribute is allowed.
func (m *Manager) CheckAttributeAccess(ctx *AccessContext, attr string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.evaluator.CheckAttributeAccess(ctx, attr)
}

// CanRead checks if the user can read the target entry.
func (m *Manager) CanRead(bindDN, targetDN string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.evaluator.CanRead(bindDN, targetDN)
}

// CanWrite checks if the user can write to the target entry.
func (m *Manager) CanWrite(bindDN, targetDN string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.evaluator.CanWrite(bindDN, targetDN)
}

// CanAdd checks if the user can add entries under the target DN.
func (m *Manager) CanAdd(bindDN, targetDN string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.evaluator.CanAdd(bindDN, targetDN)
}

// CanDelete checks if the user can delete the target entry.
func (m *Manager) CanDelete(bindDN, targetDN string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.evaluator.CanDelete(bindDN, targetDN)
}

// CanSearch checks if the user can search the target entry.
func (m *Manager) CanSearch(bindDN, targetDN string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.evaluator.CanSearch(bindDN, targetDN)
}

// CanCompare checks if the user can compare attributes on the target entry.
func (m *Manager) CanCompare(bindDN, targetDN string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.evaluator.CanCompare(bindDN, targetDN)
}

// FilterAttributes filters entry attributes based on ACL.
func (m *Manager) FilterAttributes(ctx *AccessContext, entry *Entry) *Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.evaluator.FilterAttributes(ctx, entry)
}

// FilterAttributeList returns a list of attributes the user can access.
func (m *Manager) FilterAttributeList(ctx *AccessContext, attrs []string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.evaluator.FilterAttributeList(ctx, attrs)
}

// Stats returns reload statistics.
func (m *Manager) Stats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ManagerStats{
		FilePath:      m.filePath,
		RuleCount:     len(m.config.Rules),
		DefaultPolicy: m.config.DefaultPolicy,
		ReloadCount:   atomic.LoadUint64(&m.reloadCount),
		LastReload:    m.lastReload,
		LastError:     m.lastError,
		LastErrorTime: m.lastErrorTime,
	}
}

// ManagerStats holds ACL manager statistics.
type ManagerStats struct {
	FilePath      string
	RuleCount     int
	DefaultPolicy string
	ReloadCount   uint64
	LastReload    time.Time
	LastError     error
	LastErrorTime time.Time
}

// IsFileMode returns true if manager is using file-based config.
func (m *Manager) IsFileMode() bool {
	return m.filePath != ""
}

// FilePath returns the ACL file path (empty if using embedded config).
func (m *Manager) FilePath() string {
	return m.filePath
}

// logInfo logs an info message if logger is available.
func (m *Manager) logInfo(msg string, keysAndValues ...interface{}) {
	if m.logger != nil {
		m.logger.Info(msg, keysAndValues...)
	}
}

// logError logs an error message if logger is available.
func (m *Manager) logError(msg string, keysAndValues ...interface{}) {
	if m.logger != nil {
		m.logger.Error(msg, keysAndValues...)
	}
}

// AddRule adds a new ACL rule at the specified index.
// If index is -1 or >= len(rules), appends to the end.
func (m *Manager) AddRule(rule *ACL, index int) error {
	if rule == nil {
		return fmt.Errorf("rule cannot be nil")
	}
	if rule.Target == "" {
		return ErrMissingTarget
	}
	if rule.Subject == "" {
		return ErrMissingSubject
	}
	if rule.Rights == 0 {
		return ErrMissingRights
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.config.Rules) {
		m.config.Rules = append(m.config.Rules, rule)
	} else {
		m.config.Rules = append(m.config.Rules[:index], append([]*ACL{rule}, m.config.Rules[index:]...)...)
	}

	m.evaluator = NewEvaluator(m.config)
	m.logInfo("ACL rule added", "index", index, "target", rule.Target, "subject", rule.Subject)

	return nil
}

// UpdateRule updates an existing ACL rule at the specified index.
func (m *Manager) UpdateRule(index int, rule *ACL) error {
	if rule == nil {
		return fmt.Errorf("rule cannot be nil")
	}
	if rule.Target == "" {
		return ErrMissingTarget
	}
	if rule.Subject == "" {
		return ErrMissingSubject
	}
	if rule.Rights == 0 {
		return ErrMissingRights
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.config.Rules) {
		return fmt.Errorf("rule index %d out of range (0-%d)", index, len(m.config.Rules)-1)
	}

	m.config.Rules[index] = rule
	m.evaluator = NewEvaluator(m.config)
	m.logInfo("ACL rule updated", "index", index, "target", rule.Target, "subject", rule.Subject)

	return nil
}

// DeleteRule removes an ACL rule at the specified index.
func (m *Manager) DeleteRule(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.config.Rules) {
		return fmt.Errorf("rule index %d out of range (0-%d)", index, len(m.config.Rules)-1)
	}

	m.config.Rules = append(m.config.Rules[:index], m.config.Rules[index+1:]...)
	m.evaluator = NewEvaluator(m.config)
	m.logInfo("ACL rule deleted", "index", index)

	return nil
}

// SetDefaultPolicy updates the default policy.
func (m *Manager) SetDefaultPolicy(policy string) error {
	policy = strings.ToLower(policy)
	if policy != "allow" && policy != "deny" {
		return fmt.Errorf("invalid policy: %s (must be allow or deny)", policy)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.DefaultPolicy = policy
	m.evaluator = NewEvaluator(m.config)
	m.logInfo("ACL default policy changed", "policy", policy)

	return nil
}

// GetRule returns a rule at the specified index.
func (m *Manager) GetRule(index int) (*ACL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if index < 0 || index >= len(m.config.Rules) {
		return nil, fmt.Errorf("rule index %d out of range (0-%d)", index, len(m.config.Rules)-1)
	}

	return m.config.Rules[index], nil
}

// GetRules returns all rules.
func (m *Manager) GetRules() []*ACL {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*ACL, len(m.config.Rules))
	copy(rules, m.config.Rules)
	return rules
}

// SaveToFile saves current ACL config to file.
func (m *Manager) SaveToFile() error {
	if m.filePath == "" {
		return fmt.Errorf("no ACL file configured")
	}

	m.mu.RLock()
	data := m.configToYAML()
	m.mu.RUnlock()

	if err := os.WriteFile(m.filePath, []byte(data), 0644); err != nil {
		return fmt.Errorf("failed to write ACL file: %w", err)
	}

	m.logInfo("ACL saved to file", "file", m.filePath)
	return nil
}

// configToYAML converts current config to YAML string.
func (m *Manager) configToYAML() string {
	var sb strings.Builder

	sb.WriteString("version: 1\n")
	sb.WriteString(fmt.Sprintf("defaultPolicy: %s\n", m.config.DefaultPolicy))
	sb.WriteString("rules:\n")

	for _, rule := range m.config.Rules {
		sb.WriteString(fmt.Sprintf("  - target: %q\n", rule.Target))
		sb.WriteString(fmt.Sprintf("    subject: %q\n", rule.Subject))
		sb.WriteString(fmt.Sprintf("    scope: %s\n", rule.Scope.String()))
		sb.WriteString("    rights:\n")
		for _, r := range rightsToStrings(rule.Rights) {
			sb.WriteString(fmt.Sprintf("      - %s\n", r))
		}
		if len(rule.Attributes) > 0 {
			sb.WriteString("    attributes:\n")
			for _, a := range rule.Attributes {
				sb.WriteString(fmt.Sprintf("      - %q\n", a))
			}
		}
		if rule.Deny {
			sb.WriteString("    deny: true\n")
		}
	}

	return sb.String()
}

// rightsToStrings converts Right flags to string slice.
func rightsToStrings(r Right) []string {
	if r == All {
		return []string{"all"}
	}

	var rights []string
	if r.Has(Read) {
		rights = append(rights, "read")
	}
	if r.Has(Write) {
		rights = append(rights, "write")
	}
	if r.Has(Add) {
		rights = append(rights, "add")
	}
	if r.Has(Delete) {
		rights = append(rights, "delete")
	}
	if r.Has(Search) {
		rights = append(rights, "search")
	}
	if r.Has(Compare) {
		rights = append(rights, "compare")
	}
	return rights
}

// SetFilePath sets the ACL file path.
func (m *Manager) SetFilePath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filePath = path
}

// ValidateRule validates a single ACL rule.
func ValidateRule(rule *ACL) error {
	if rule == nil {
		return fmt.Errorf("rule cannot be nil")
	}
	if rule.Target == "" {
		return ErrMissingTarget
	}
	if rule.Subject == "" {
		return ErrMissingSubject
	}
	if rule.Rights == 0 {
		return ErrMissingRights
	}
	return nil
}

// GetVersion returns the current ACL version for Raft sync.
func (m *Manager) GetVersion() uint64 {
	return atomic.LoadUint64(&m.version)
}

// IncrementVersion increments and returns the new ACL version.
func (m *Manager) IncrementVersion() uint64 {
	return atomic.AddUint64(&m.version, 1)
}

// ApplyFullConfigFromRaft applies a full ACL config from Raft replication.
func (m *Manager) ApplyFullConfigFromRaft(rules []raft.ACLRuleData, defaultPolicy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Convert ACLRuleData to ACL
	aclRules := make([]*ACL, len(rules))
	for i, ruleData := range rules {
		rule, err := ruleDataToACL(&ruleData)
		if err != nil {
			return fmt.Errorf("failed to convert rule %d: %w", i, err)
		}
		aclRules[i] = rule
	}

	m.config.Rules = aclRules
	m.config.DefaultPolicy = defaultPolicy
	m.evaluator = NewEvaluator(m.config)
	m.lastReload = time.Now()
	atomic.AddUint64(&m.reloadCount, 1)
	atomic.AddUint64(&m.version, 1)

	m.logInfo("ACL config applied from Raft",
		"rules", len(rules),
		"defaultPolicy", defaultPolicy,
	)

	return nil
}

// AddRuleFromRaft adds a rule from Raft replication.
func (m *Manager) AddRuleFromRaft(ruleData *raft.ACLRuleData, index int) error {
	if ruleData == nil {
		return fmt.Errorf("rule data cannot be nil")
	}

	rule, err := ruleDataToACL(ruleData)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.config.Rules) {
		m.config.Rules = append(m.config.Rules, rule)
	} else {
		m.config.Rules = append(m.config.Rules[:index],
			append([]*ACL{rule}, m.config.Rules[index:]...)...)
	}

	m.evaluator = NewEvaluator(m.config)
	atomic.AddUint64(&m.version, 1)

	m.logInfo("ACL rule added from Raft", "index", index, "target", rule.Target)

	return nil
}

// UpdateRuleFromRaft updates a rule from Raft replication.
func (m *Manager) UpdateRuleFromRaft(ruleData *raft.ACLRuleData, index int) error {
	if ruleData == nil {
		return fmt.Errorf("rule data cannot be nil")
	}

	rule, err := ruleDataToACL(ruleData)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.config.Rules) {
		return fmt.Errorf("rule index %d out of range", index)
	}

	m.config.Rules[index] = rule
	m.evaluator = NewEvaluator(m.config)
	atomic.AddUint64(&m.version, 1)

	m.logInfo("ACL rule updated from Raft", "index", index, "target", rule.Target)

	return nil
}

// DeleteRuleFromRaft deletes a rule from Raft replication.
func (m *Manager) DeleteRuleFromRaft(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.config.Rules) {
		return nil // Already deleted or invalid, ignore
	}

	m.config.Rules = append(m.config.Rules[:index], m.config.Rules[index+1:]...)
	m.evaluator = NewEvaluator(m.config)
	atomic.AddUint64(&m.version, 1)

	m.logInfo("ACL rule deleted from Raft", "index", index)

	return nil
}

// SetDefaultPolicyFromRaft sets default policy from Raft replication.
func (m *Manager) SetDefaultPolicyFromRaft(policy string) error {
	policy = strings.ToLower(policy)
	if policy != "allow" && policy != "deny" {
		return fmt.Errorf("invalid policy: %s", policy)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.DefaultPolicy = policy
	m.evaluator = NewEvaluator(m.config)
	atomic.AddUint64(&m.version, 1)

	m.logInfo("ACL default policy set from Raft", "policy", policy)

	return nil
}

// ACLSnapshot represents ACL data for Raft snapshot.
type ACLSnapshot struct {
	Version       uint64              `json:"version"`
	DefaultPolicy string              `json:"defaultPolicy"`
	Rules         []raft.ACLRuleData  `json:"rules"`
}

// GetACLSnapshot returns the current ACL config as a snapshot for Raft.
func (m *Manager) GetACLSnapshot() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := &ACLSnapshot{
		Version:       atomic.LoadUint64(&m.version),
		DefaultPolicy: m.config.DefaultPolicy,
		Rules:         make([]raft.ACLRuleData, len(m.config.Rules)),
	}

	for i, rule := range m.config.Rules {
		snapshot.Rules[i] = aclToRuleData(rule)
	}

	return json.Marshal(snapshot)
}

// RestoreACLSnapshot restores ACL from a Raft snapshot.
func (m *Manager) RestoreACLSnapshot(data []byte) error {
	var snapshot ACLSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("failed to unmarshal ACL snapshot: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Convert rules
	rules := make([]*ACL, len(snapshot.Rules))
	for i, ruleData := range snapshot.Rules {
		rule, err := ruleDataToACL(&ruleData)
		if err != nil {
			return fmt.Errorf("failed to convert rule %d: %w", i, err)
		}
		rules[i] = rule
	}

	m.config.Rules = rules
	m.config.DefaultPolicy = snapshot.DefaultPolicy
	m.evaluator = NewEvaluator(m.config)
	atomic.StoreUint64(&m.version, snapshot.Version)

	m.logInfo("ACL restored from Raft snapshot",
		"rules", len(rules),
		"defaultPolicy", snapshot.DefaultPolicy,
	)

	return nil
}

// ruleDataToACL converts raft.ACLRuleData to ACL.
func ruleDataToACL(data *raft.ACLRuleData) (*ACL, error) {
	rule := &ACL{
		Target:     data.Target,
		Subject:    data.Subject,
		Attributes: data.Attributes,
		Deny:       data.Deny,
	}

	// Parse scope
	switch strings.ToLower(data.Scope) {
	case "base", "":
		rule.Scope = ScopeBase
	case "one", "onelevel":
		rule.Scope = ScopeOne
	case "sub", "subtree":
		rule.Scope = ScopeSubtree
	default:
		return nil, fmt.Errorf("invalid scope: %s", data.Scope)
	}

	// Parse rights
	for _, r := range data.Rights {
		switch strings.ToLower(r) {
		case "read":
			rule.Rights |= Read
		case "write":
			rule.Rights |= Write
		case "add":
			rule.Rights |= Add
		case "delete":
			rule.Rights |= Delete
		case "search":
			rule.Rights |= Search
		case "compare":
			rule.Rights |= Compare
		case "all":
			rule.Rights = All
		default:
			return nil, fmt.Errorf("invalid right: %s", r)
		}
	}

	return rule, nil
}

// aclToRuleData converts ACL to raft.ACLRuleData.
func aclToRuleData(rule *ACL) raft.ACLRuleData {
	return raft.ACLRuleData{
		Target:     rule.Target,
		Subject:    rule.Subject,
		Scope:      rule.Scope.String(),
		Rights:     rightsToStrings(rule.Rights),
		Attributes: rule.Attributes,
		Deny:       rule.Deny,
	}
}

// ToRuleDataSlice converts ACL rules to raft.ACLRuleData slice for Raft commands.
func (m *Manager) ToRuleDataSlice() []raft.ACLRuleData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]raft.ACLRuleData, len(m.config.Rules))
	for i, rule := range m.config.Rules {
		result[i] = aclToRuleData(rule)
	}
	return result
}

// GetDefaultPolicy returns the current default policy.
func (m *Manager) GetDefaultPolicy() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.DefaultPolicy
}
