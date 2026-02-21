package acl

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oba-ldap/oba/internal/logging"
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
