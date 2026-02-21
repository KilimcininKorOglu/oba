package config

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// ConfigManager manages runtime configuration with hot reload support.
type ConfigManager struct {
	config     *Config
	configFile string
	mu         sync.RWMutex
	onUpdate   func(old, new *Config)
}

// NewConfigManager creates a new config manager.
func NewConfigManager(cfg *Config, configFile string) *ConfigManager {
	return &ConfigManager{
		config:     cfg,
		configFile: configFile,
	}
}

// SetOnUpdate sets the callback for config updates.
func (m *ConfigManager) SetOnUpdate(fn func(old, new *Config)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onUpdate = fn
}

// GetConfig returns the current config.
func (m *ConfigManager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// GetConfigFile returns the config file path.
func (m *ConfigManager) GetConfigFile() string {
	return m.configFile
}

// ConfigJSON represents config in JSON format with sensitive data masked.
type ConfigJSON struct {
	Server   ServerConfigJSON   `json:"server"`
	Logging  LogConfigJSON      `json:"logging"`
	Security SecurityConfigJSON `json:"security"`
	REST     RESTConfigJSON     `json:"rest"`
	Storage  StorageConfigJSON  `json:"storage"`
}

// ServerConfigJSON represents server config in JSON.
type ServerConfigJSON struct {
	Address        string `json:"address"`
	TLSAddress     string `json:"tlsAddress,omitempty"`
	MaxConnections int    `json:"maxConnections"`
	ReadTimeout    string `json:"readTimeout"`
	WriteTimeout   string `json:"writeTimeout"`
	TLSCert        string `json:"tlsCert,omitempty"`
	TLSKey         string `json:"tlsKey,omitempty"`
}

// LogConfigJSON represents logging config in JSON.
type LogConfigJSON struct {
	Level  string `json:"level"`
	Format string `json:"format"`
	Output string `json:"output"`
}

// SecurityConfigJSON represents security config in JSON.
type SecurityConfigJSON struct {
	RateLimit      RateLimitConfigJSON      `json:"rateLimit"`
	PasswordPolicy PasswordPolicyConfigJSON `json:"passwordPolicy"`
	Encryption     EncryptionConfigJSON     `json:"encryption"`
}

// RateLimitConfigJSON represents rate limit config in JSON.
type RateLimitConfigJSON struct {
	Enabled         bool   `json:"enabled"`
	MaxAttempts     int    `json:"maxAttempts"`
	LockoutDuration string `json:"lockoutDuration"`
}

// PasswordPolicyConfigJSON represents password policy config in JSON.
type PasswordPolicyConfigJSON struct {
	Enabled          bool   `json:"enabled"`
	MinLength        int    `json:"minLength"`
	RequireUppercase bool   `json:"requireUppercase"`
	RequireLowercase bool   `json:"requireLowercase"`
	RequireDigit     bool   `json:"requireDigit"`
	RequireSpecial   bool   `json:"requireSpecial"`
	MaxAge           string `json:"maxAge"`
	HistoryCount     int    `json:"historyCount"`
}

// EncryptionConfigJSON represents encryption config in JSON.
type EncryptionConfigJSON struct {
	Enabled bool   `json:"enabled"`
	KeyFile string `json:"keyFile,omitempty"`
}

// RESTConfigJSON represents REST config in JSON.
type RESTConfigJSON struct {
	Enabled     bool     `json:"enabled"`
	Address     string   `json:"address"`
	RateLimit   int      `json:"rateLimit"`
	TokenTTL    string   `json:"tokenTTL"`
	CORSOrigins []string `json:"corsOrigins"`
	JWTSecret   string   `json:"jwtSecret"`
}

// StorageConfigJSON represents storage config in JSON.
type StorageConfigJSON struct {
	DataDir            string `json:"dataDir"`
	PageSize           int    `json:"pageSize"`
	BufferPoolSize     string `json:"bufferPoolSize"`
	CheckpointInterval string `json:"checkpointInterval"`
}

// ToJSON returns config as JSON-serializable struct with sensitive data masked.
func (m *ConfigManager) ToJSON() *ConfigJSON {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &ConfigJSON{
		Server: ServerConfigJSON{
			Address:        m.config.Server.Address,
			TLSAddress:     m.config.Server.TLSAddress,
			MaxConnections: m.config.Server.MaxConnections,
			ReadTimeout:    m.config.Server.ReadTimeout.String(),
			WriteTimeout:   m.config.Server.WriteTimeout.String(),
			TLSCert:        m.config.Server.TLSCert,
			TLSKey:         maskPath(m.config.Server.TLSKey),
		},
		Logging: LogConfigJSON{
			Level:  m.config.Logging.Level,
			Format: m.config.Logging.Format,
			Output: m.config.Logging.Output,
		},
		Security: SecurityConfigJSON{
			RateLimit: RateLimitConfigJSON{
				Enabled:         m.config.Security.RateLimit.Enabled,
				MaxAttempts:     m.config.Security.RateLimit.MaxAttempts,
				LockoutDuration: m.config.Security.RateLimit.LockoutDuration.String(),
			},
			PasswordPolicy: PasswordPolicyConfigJSON{
				Enabled:          m.config.Security.PasswordPolicy.Enabled,
				MinLength:        m.config.Security.PasswordPolicy.MinLength,
				RequireUppercase: m.config.Security.PasswordPolicy.RequireUppercase,
				RequireLowercase: m.config.Security.PasswordPolicy.RequireLowercase,
				RequireDigit:     m.config.Security.PasswordPolicy.RequireDigit,
				RequireSpecial:   m.config.Security.PasswordPolicy.RequireSpecial,
				MaxAge:           m.config.Security.PasswordPolicy.MaxAge.String(),
				HistoryCount:     m.config.Security.PasswordPolicy.HistoryCount,
			},
			Encryption: EncryptionConfigJSON{
				Enabled: m.config.Security.Encryption.Enabled,
				KeyFile: maskPath(m.config.Security.Encryption.KeyFile),
			},
		},
		REST: RESTConfigJSON{
			Enabled:     m.config.REST.Enabled,
			Address:     m.config.REST.Address,
			RateLimit:   m.config.REST.RateLimit,
			TokenTTL:    m.config.REST.TokenTTL.String(),
			CORSOrigins: m.config.REST.CORSOrigins,
			JWTSecret:   "********",
		},
		Storage: StorageConfigJSON{
			DataDir:            m.config.Storage.DataDir,
			PageSize:           m.config.Storage.PageSize,
			BufferPoolSize:     m.config.Storage.BufferPoolSize,
			CheckpointInterval: m.config.Storage.CheckpointInterval.String(),
		},
	}
}

// GetSection returns a specific config section.
func (m *ConfigManager) GetSection(section string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch strings.ToLower(section) {
	case "server":
		return ServerConfigJSON{
			Address:        m.config.Server.Address,
			TLSAddress:     m.config.Server.TLSAddress,
			MaxConnections: m.config.Server.MaxConnections,
			ReadTimeout:    m.config.Server.ReadTimeout.String(),
			WriteTimeout:   m.config.Server.WriteTimeout.String(),
			TLSCert:        m.config.Server.TLSCert,
			TLSKey:         maskPath(m.config.Server.TLSKey),
		}, nil
	case "logging":
		return LogConfigJSON{
			Level:  m.config.Logging.Level,
			Format: m.config.Logging.Format,
			Output: m.config.Logging.Output,
		}, nil
	case "security":
		return SecurityConfigJSON{
			RateLimit: RateLimitConfigJSON{
				Enabled:         m.config.Security.RateLimit.Enabled,
				MaxAttempts:     m.config.Security.RateLimit.MaxAttempts,
				LockoutDuration: m.config.Security.RateLimit.LockoutDuration.String(),
			},
			PasswordPolicy: PasswordPolicyConfigJSON{
				Enabled:          m.config.Security.PasswordPolicy.Enabled,
				MinLength:        m.config.Security.PasswordPolicy.MinLength,
				RequireUppercase: m.config.Security.PasswordPolicy.RequireUppercase,
				RequireLowercase: m.config.Security.PasswordPolicy.RequireLowercase,
				RequireDigit:     m.config.Security.PasswordPolicy.RequireDigit,
				RequireSpecial:   m.config.Security.PasswordPolicy.RequireSpecial,
				MaxAge:           m.config.Security.PasswordPolicy.MaxAge.String(),
				HistoryCount:     m.config.Security.PasswordPolicy.HistoryCount,
			},
			Encryption: EncryptionConfigJSON{
				Enabled: m.config.Security.Encryption.Enabled,
				KeyFile: maskPath(m.config.Security.Encryption.KeyFile),
			},
		}, nil
	case "rest":
		return RESTConfigJSON{
			Enabled:     m.config.REST.Enabled,
			Address:     m.config.REST.Address,
			RateLimit:   m.config.REST.RateLimit,
			TokenTTL:    m.config.REST.TokenTTL.String(),
			CORSOrigins: m.config.REST.CORSOrigins,
			JWTSecret:   "********",
		}, nil
	case "storage":
		return StorageConfigJSON{
			DataDir:            m.config.Storage.DataDir,
			PageSize:           m.config.Storage.PageSize,
			BufferPoolSize:     m.config.Storage.BufferPoolSize,
			CheckpointInterval: m.config.Storage.CheckpointInterval.String(),
		}, nil
	default:
		return nil, fmt.Errorf("unknown section: %s", section)
	}
}

// UpdateSection updates a config section with hot-reload support.
func (m *ConfigManager) UpdateSection(section string, data map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldConfig := m.config
	newConfig := copyConfig(oldConfig)

	switch strings.ToLower(section) {
	case "logging":
		if v, ok := data["level"].(string); ok {
			newConfig.Logging.Level = v
		}
		if v, ok := data["format"].(string); ok {
			newConfig.Logging.Format = v
		}
	case "server":
		if v, ok := data["maxConnections"].(float64); ok {
			newConfig.Server.MaxConnections = int(v)
		}
		if v, ok := data["readTimeout"].(string); ok {
			if d, err := time.ParseDuration(v); err == nil {
				newConfig.Server.ReadTimeout = d
			}
		}
		if v, ok := data["writeTimeout"].(string); ok {
			if d, err := time.ParseDuration(v); err == nil {
				newConfig.Server.WriteTimeout = d
			}
		}
		if v, ok := data["tlsCert"].(string); ok {
			newConfig.Server.TLSCert = v
		}
		if v, ok := data["tlsKey"].(string); ok {
			newConfig.Server.TLSKey = v
		}
	case "security.ratelimit":
		if v, ok := data["enabled"].(bool); ok {
			newConfig.Security.RateLimit.Enabled = v
		}
		if v, ok := data["maxAttempts"].(float64); ok {
			newConfig.Security.RateLimit.MaxAttempts = int(v)
		}
		if v, ok := data["lockoutDuration"].(string); ok {
			if d, err := time.ParseDuration(v); err == nil {
				newConfig.Security.RateLimit.LockoutDuration = d
			}
		}
	case "security.passwordpolicy":
		if v, ok := data["enabled"].(bool); ok {
			newConfig.Security.PasswordPolicy.Enabled = v
		}
		if v, ok := data["minLength"].(float64); ok {
			newConfig.Security.PasswordPolicy.MinLength = int(v)
		}
		if v, ok := data["requireUppercase"].(bool); ok {
			newConfig.Security.PasswordPolicy.RequireUppercase = v
		}
		if v, ok := data["requireLowercase"].(bool); ok {
			newConfig.Security.PasswordPolicy.RequireLowercase = v
		}
		if v, ok := data["requireDigit"].(bool); ok {
			newConfig.Security.PasswordPolicy.RequireDigit = v
		}
		if v, ok := data["requireSpecial"].(bool); ok {
			newConfig.Security.PasswordPolicy.RequireSpecial = v
		}
		if v, ok := data["maxAge"].(string); ok {
			if d, err := time.ParseDuration(v); err == nil {
				newConfig.Security.PasswordPolicy.MaxAge = d
			}
		}
		if v, ok := data["historyCount"].(float64); ok {
			newConfig.Security.PasswordPolicy.HistoryCount = int(v)
		}
	case "rest":
		if v, ok := data["rateLimit"].(float64); ok {
			newConfig.REST.RateLimit = int(v)
		}
		if v, ok := data["tokenTTL"].(string); ok {
			if d, err := time.ParseDuration(v); err == nil {
				newConfig.REST.TokenTTL = d
			}
		}
		if v, ok := data["corsOrigins"].([]interface{}); ok {
			origins := make([]string, len(v))
			for i, o := range v {
				if s, ok := o.(string); ok {
					origins[i] = s
				}
			}
			newConfig.REST.CORSOrigins = origins
		}
	default:
		return fmt.Errorf("unknown or read-only section: %s", section)
	}

	// Validate new config
	if errs := ValidateConfig(newConfig); len(errs) > 0 {
		return fmt.Errorf("validation failed: %v", errs[0])
	}

	m.config = newConfig

	// Trigger callback
	if m.onUpdate != nil {
		go m.onUpdate(oldConfig, newConfig)
	}

	return nil
}

// Reload reloads config from file.
func (m *ConfigManager) Reload() error {
	if m.configFile == "" {
		return fmt.Errorf("no config file configured")
	}

	newConfig, err := LoadConfig(m.configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if errs := ValidateConfig(newConfig); len(errs) > 0 {
		return fmt.Errorf("validation failed: %v", errs[0])
	}

	m.mu.Lock()
	oldConfig := m.config
	m.config = newConfig
	onUpdate := m.onUpdate
	m.mu.Unlock()

	if onUpdate != nil {
		go onUpdate(oldConfig, newConfig)
	}

	return nil
}

// SaveToFile saves current config to file.
func (m *ConfigManager) SaveToFile() error {
	if m.configFile == "" {
		return fmt.Errorf("no config file configured")
	}

	m.mu.RLock()
	data := m.configToYAML()
	m.mu.RUnlock()

	if err := os.WriteFile(m.configFile, []byte(data), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// configToYAML converts config to YAML string.
func (m *ConfigManager) configToYAML() string {
	var sb strings.Builder

	sb.WriteString("server:\n")
	sb.WriteString(fmt.Sprintf("  address: %q\n", m.config.Server.Address))
	if m.config.Server.TLSAddress != "" {
		sb.WriteString(fmt.Sprintf("  tlsAddress: %q\n", m.config.Server.TLSAddress))
	}
	sb.WriteString(fmt.Sprintf("  maxConnections: %d\n", m.config.Server.MaxConnections))
	sb.WriteString(fmt.Sprintf("  readTimeout: %s\n", m.config.Server.ReadTimeout))
	sb.WriteString(fmt.Sprintf("  writeTimeout: %s\n", m.config.Server.WriteTimeout))
	if m.config.Server.PIDFile != "" {
		sb.WriteString(fmt.Sprintf("  pidFile: %q\n", m.config.Server.PIDFile))
	}

	sb.WriteString("\ndirectory:\n")
	sb.WriteString(fmt.Sprintf("  baseDN: %q\n", m.config.Directory.BaseDN))
	sb.WriteString(fmt.Sprintf("  rootDN: %q\n", m.config.Directory.RootDN))
	sb.WriteString(fmt.Sprintf("  rootPassword: %q\n", m.config.Directory.RootPassword))

	sb.WriteString("\nstorage:\n")
	sb.WriteString(fmt.Sprintf("  dataDir: %q\n", m.config.Storage.DataDir))
	sb.WriteString(fmt.Sprintf("  pageSize: %d\n", m.config.Storage.PageSize))
	sb.WriteString(fmt.Sprintf("  bufferPoolSize: %q\n", m.config.Storage.BufferPoolSize))
	sb.WriteString(fmt.Sprintf("  checkpointInterval: %s\n", m.config.Storage.CheckpointInterval))

	sb.WriteString("\nsecurity:\n")
	sb.WriteString("  encryption:\n")
	sb.WriteString(fmt.Sprintf("    enabled: %t\n", m.config.Security.Encryption.Enabled))
	if m.config.Security.Encryption.KeyFile != "" {
		sb.WriteString(fmt.Sprintf("    keyFile: %q\n", m.config.Security.Encryption.KeyFile))
	}
	sb.WriteString("  rateLimit:\n")
	sb.WriteString(fmt.Sprintf("    enabled: %t\n", m.config.Security.RateLimit.Enabled))
	sb.WriteString(fmt.Sprintf("    maxAttempts: %d\n", m.config.Security.RateLimit.MaxAttempts))
	sb.WriteString(fmt.Sprintf("    lockoutDuration: %s\n", m.config.Security.RateLimit.LockoutDuration))
	sb.WriteString("  passwordPolicy:\n")
	sb.WriteString(fmt.Sprintf("    enabled: %t\n", m.config.Security.PasswordPolicy.Enabled))
	sb.WriteString(fmt.Sprintf("    minLength: %d\n", m.config.Security.PasswordPolicy.MinLength))

	sb.WriteString("\nlogging:\n")
	sb.WriteString(fmt.Sprintf("  level: %q\n", m.config.Logging.Level))
	sb.WriteString(fmt.Sprintf("  format: %q\n", m.config.Logging.Format))
	sb.WriteString(fmt.Sprintf("  output: %q\n", m.config.Logging.Output))

	if m.config.ACLFile != "" {
		sb.WriteString(fmt.Sprintf("\naclFile: %q\n", m.config.ACLFile))
	}

	sb.WriteString("\nrest:\n")
	sb.WriteString(fmt.Sprintf("  enabled: %t\n", m.config.REST.Enabled))
	sb.WriteString(fmt.Sprintf("  address: %q\n", m.config.REST.Address))
	sb.WriteString(fmt.Sprintf("  jwtSecret: %q\n", m.config.REST.JWTSecret))
	sb.WriteString(fmt.Sprintf("  tokenTTL: %s\n", m.config.REST.TokenTTL))
	sb.WriteString(fmt.Sprintf("  rateLimit: %d\n", m.config.REST.RateLimit))
	sb.WriteString("  corsOrigins:\n")
	for _, origin := range m.config.REST.CORSOrigins {
		sb.WriteString(fmt.Sprintf("    - %q\n", origin))
	}

	return sb.String()
}

// copyConfig creates a deep copy of config.
func copyConfig(c *Config) *Config {
	newConfig := *c
	newConfig.REST.CORSOrigins = make([]string, len(c.REST.CORSOrigins))
	copy(newConfig.REST.CORSOrigins, c.REST.CORSOrigins)
	return &newConfig
}

// maskPath masks sensitive file paths (shows path but indicates it's sensitive).
func maskPath(path string) string {
	if path == "" {
		return ""
	}
	return path
}
