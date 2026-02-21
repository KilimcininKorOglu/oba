// Package config provides configuration parsing and management for the Oba LDAP server.
package config

import (
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Parser errors.
var (
	ErrInvalidYAML       = errors.New("invalid YAML format")
	ErrInvalidIndent     = errors.New("invalid indentation")
	ErrUnexpectedToken   = errors.New("unexpected token")
	ErrInvalidDuration   = errors.New("invalid duration format")
	ErrInvalidNumber     = errors.New("invalid number format")
	ErrFileNotFound      = errors.New("configuration file not found")
	ErrInvalidListItem   = errors.New("invalid list item format")
	ErrMissingConfigFile = errors.New("config file path is required")
	ErrMissingOnChange   = errors.New("onChange callback is required")
)

// LoadConfig loads configuration from a file path.
// It reads the file, substitutes environment variables, parses YAML,
// and applies defaults for missing values.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}

	return ParseConfig(data)
}

// ParseConfig parses configuration from YAML data.
// It substitutes environment variables and applies defaults for missing values.
func ParseConfig(data []byte) (*Config, error) {
	// Substitute environment variables
	data = substituteEnvVars(data)

	// Start with defaults
	config := DefaultConfig()

	// Parse YAML and merge with defaults
	if err := parseYAML(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// substituteEnvVars replaces ${VAR} and ${VAR:-default} patterns with environment variable values.
func substituteEnvVars(data []byte) []byte {
	// Pattern matches ${VAR} or ${VAR:-default}
	re := regexp.MustCompile(`\$\{([^}]+)\}`)

	return re.ReplaceAllFunc(data, func(match []byte) []byte {
		// Extract content between ${ and }
		content := string(match[2 : len(match)-1])

		// Check for default value syntax: VAR:-default
		if idx := strings.Index(content, ":-"); idx != -1 {
			varName := content[:idx]
			defaultVal := content[idx+2:]
			if val := os.Getenv(varName); val != "" {
				return []byte(val)
			}
			return []byte(defaultVal)
		}

		// Simple variable substitution
		return []byte(os.Getenv(content))
	})
}

// yamlNode represents a parsed YAML node.
type yamlNode struct {
	key          string
	value        string
	indent       int
	children     []*yamlNode
	isList       bool
	isListObject bool // true when list item contains key: value (- key: value)
	listItems    []string
}

// parseYAML parses YAML data into the config struct.
func parseYAML(data []byte, config *Config) error {
	lines := strings.Split(string(data), "\n")
	root := &yamlNode{indent: -1}

	if err := buildTree(lines, root); err != nil {
		return err
	}

	return applyConfig(root, config)
}

// buildTree builds a tree structure from YAML lines.
func buildTree(lines []string, root *yamlNode) error {
	stack := []*yamlNode{root}

	for _, line := range lines {
		// Skip empty lines and comments
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Calculate indentation
		indent := countIndent(line)

		// Parse key-value or list item
		node, err := parseLine(trimmed, indent)
		if err != nil {
			return err
		}

		// Find parent based on indentation
		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}

		parent := stack[len(stack)-1]

		// Handle list items
		if node.isList {
			if node.isListObject {
				// List item that starts a new object (- key: value)
				// Create a container node for this list item
				listItemNode := &yamlNode{
					indent:   indent,
					children: []*yamlNode{},
				}
				// Add the first key-value as child
				firstChild := &yamlNode{
					key:    node.key,
					value:  node.value,
					indent: indent + 2,
				}
				listItemNode.children = append(listItemNode.children, firstChild)
				parent.children = append(parent.children, listItemNode)
				stack = append(stack, listItemNode)
				continue
			}

			// Simple list item (- value)
			if parent.listItems == nil {
				parent.listItems = []string{}
			}
			parent.listItems = append(parent.listItems, node.value)
			continue
		}

		parent.children = append(parent.children, node)
		stack = append(stack, node)
	}

	return nil
}

// countIndent counts the number of leading spaces.
func countIndent(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' {
			count++
		} else if ch == '\t' {
			count += 2 // Treat tab as 2 spaces
		} else {
			break
		}
	}
	return count
}

// parseLine parses a single YAML line.
func parseLine(line string, indent int) (*yamlNode, error) {
	// Check for list item
	if strings.HasPrefix(line, "- ") {
		content := strings.TrimPrefix(line, "- ")

		// Check if list item contains key: value (nested object like "- target: *")
		if colonIdx := strings.Index(content, ":"); colonIdx != -1 {
			key := strings.TrimSpace(content[:colonIdx])
			value := ""
			if colonIdx+1 < len(content) {
				value = strings.TrimSpace(content[colonIdx+1:])
			}
			value = unquote(value)

			return &yamlNode{
				key:          key,
				value:        value,
				indent:       indent,
				isList:       true,
				isListObject: true,
			}, nil
		}

		// Simple list item (- value)
		return &yamlNode{
			value:  strings.TrimSpace(content),
			indent: indent,
			isList: true,
		}, nil
	}

	// Parse key: value
	colonIdx := strings.Index(line, ":")
	if colonIdx == -1 {
		return nil, ErrInvalidYAML
	}

	key := strings.TrimSpace(line[:colonIdx])
	value := ""
	if colonIdx+1 < len(line) {
		value = strings.TrimSpace(line[colonIdx+1:])
	}

	// Remove quotes from value
	value = unquote(value)

	return &yamlNode{
		key:    key,
		value:  value,
		indent: indent,
	}, nil
}

// unquote removes surrounding quotes from a string.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// parseInlineArray parses inline array format like ["a", "b", "c"]
func parseInlineArray(s string) []string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil
	}

	// Remove brackets
	s = s[1 : len(s)-1]
	if s == "" {
		return []string{}
	}

	var result []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		item = unquote(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

// applyConfig applies parsed YAML nodes to the config struct.
func applyConfig(root *yamlNode, config *Config) error {
	for _, node := range root.children {
		switch node.key {
		case "server":
			if err := applyServerConfig(node, &config.Server); err != nil {
				return err
			}
		case "directory":
			if err := applyDirectoryConfig(node, &config.Directory); err != nil {
				return err
			}
		case "storage":
			if err := applyStorageConfig(node, &config.Storage); err != nil {
				return err
			}
		case "logging":
			if err := applyLogConfig(node, &config.Logging); err != nil {
				return err
			}
		case "security":
			if err := applySecurityConfig(node, &config.Security); err != nil {
				return err
			}
		case "acl":
			if err := applyACLConfig(node, &config.ACL); err != nil {
				return err
			}
		case "aclFile":
			if node.value != "" {
				config.ACLFile = node.value
			}
		case "rest":
			if err := applyRESTConfig(node, &config.REST); err != nil {
				return err
			}
		case "cluster":
			if err := applyClusterConfig(node, &config.Cluster); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyServerConfig applies server configuration.
func applyServerConfig(node *yamlNode, config *ServerConfig) error {
	for _, child := range node.children {
		switch child.key {
		case "address":
			if child.value != "" {
				config.Address = child.value
			}
		case "tlsAddress":
			if child.value != "" {
				config.TLSAddress = child.value
			}
		case "tlsCert":
			if child.value != "" {
				config.TLSCert = child.value
			}
		case "tlsKey":
			if child.value != "" {
				config.TLSKey = child.value
			}
		case "maxConnections":
			if child.value != "" {
				val, err := strconv.Atoi(child.value)
				if err != nil {
					return ErrInvalidNumber
				}
				config.MaxConnections = val
			}
		case "readTimeout":
			if child.value != "" {
				dur, err := parseDuration(child.value)
				if err != nil {
					return err
				}
				config.ReadTimeout = dur
			}
		case "writeTimeout":
			if child.value != "" {
				dur, err := parseDuration(child.value)
				if err != nil {
					return err
				}
				config.WriteTimeout = dur
			}
		case "pidFile":
			if child.value != "" {
				config.PIDFile = child.value
			}
		}
	}
	return nil
}

// applyDirectoryConfig applies directory configuration.
func applyDirectoryConfig(node *yamlNode, config *DirectoryConfig) error {
	for _, child := range node.children {
		switch child.key {
		case "baseDN":
			if child.value != "" {
				config.BaseDN = child.value
			}
		case "rootDN":
			if child.value != "" {
				config.RootDN = child.value
			}
		case "rootPassword":
			if child.value != "" {
				config.RootPassword = child.value
			}
		}
	}
	return nil
}

// applyStorageConfig applies storage configuration.
func applyStorageConfig(node *yamlNode, config *StorageConfig) error {
	for _, child := range node.children {
		switch child.key {
		case "dataDir":
			if child.value != "" {
				config.DataDir = child.value
			}
		case "walDir":
			if child.value != "" {
				config.WALDir = child.value
			}
		case "pageSize":
			if child.value != "" {
				val, err := strconv.Atoi(child.value)
				if err != nil {
					return ErrInvalidNumber
				}
				config.PageSize = val
			}
		case "bufferPoolSize":
			if child.value != "" {
				config.BufferPoolSize = child.value
			}
		case "checkpointInterval":
			if child.value != "" {
				dur, err := parseDuration(child.value)
				if err != nil {
					return err
				}
				config.CheckpointInterval = dur
			}
		}
	}
	return nil
}

// applyLogConfig applies logging configuration.
func applyLogConfig(node *yamlNode, config *LogConfig) error {
	for _, child := range node.children {
		switch child.key {
		case "level":
			if child.value != "" {
				config.Level = child.value
			}
		case "format":
			if child.value != "" {
				config.Format = child.value
			}
		case "output":
			if child.value != "" {
				config.Output = child.value
			}
		case "store":
			if err := applyLogStoreConfig(child, &config.Store); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyLogStoreConfig applies log store configuration.
func applyLogStoreConfig(node *yamlNode, config *LogStoreConfig) error {
	for _, child := range node.children {
		switch child.key {
		case "enabled":
			config.Enabled = parseBool(child.value)
		case "dbPath":
			if child.value != "" {
				config.DBPath = child.value
			}
		case "maxEntries":
			if child.value != "" {
				n, err := strconv.Atoi(child.value)
				if err != nil {
					return err
				}
				config.MaxEntries = n
			}
		}
	}
	return nil
}

// applySecurityConfig applies security configuration.
func applySecurityConfig(node *yamlNode, config *SecurityConfig) error {
	for _, child := range node.children {
		switch child.key {
		case "passwordPolicy":
			if err := applyPasswordPolicyConfig(child, &config.PasswordPolicy); err != nil {
				return err
			}
		case "rateLimit":
			if err := applyRateLimitConfig(child, &config.RateLimit); err != nil {
				return err
			}
		case "encryption":
			if err := applyEncryptionConfig(child, &config.Encryption); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyPasswordPolicyConfig applies password policy configuration.
func applyPasswordPolicyConfig(node *yamlNode, config *PasswordPolicyConfig) error {
	for _, child := range node.children {
		switch child.key {
		case "enabled":
			config.Enabled = parseBool(child.value)
		case "minLength":
			if child.value != "" {
				val, err := strconv.Atoi(child.value)
				if err != nil {
					return ErrInvalidNumber
				}
				config.MinLength = val
			}
		case "requireUppercase":
			config.RequireUppercase = parseBool(child.value)
		case "requireLowercase":
			config.RequireLowercase = parseBool(child.value)
		case "requireDigit":
			config.RequireDigit = parseBool(child.value)
		case "requireSpecial":
			config.RequireSpecial = parseBool(child.value)
		case "maxAge":
			if child.value != "" {
				dur, err := parseDuration(child.value)
				if err != nil {
					return err
				}
				config.MaxAge = dur
			}
		case "historyCount":
			if child.value != "" {
				val, err := strconv.Atoi(child.value)
				if err != nil {
					return ErrInvalidNumber
				}
				config.HistoryCount = val
			}
		}
	}
	return nil
}

// applyRateLimitConfig applies rate limit configuration.
func applyRateLimitConfig(node *yamlNode, config *RateLimitConfig) error {
	for _, child := range node.children {
		switch child.key {
		case "enabled":
			config.Enabled = parseBool(child.value)
		case "maxAttempts":
			if child.value != "" {
				val, err := strconv.Atoi(child.value)
				if err != nil {
					return ErrInvalidNumber
				}
				config.MaxAttempts = val
			}
		case "lockoutDuration":
			if child.value != "" {
				dur, err := parseDuration(child.value)
				if err != nil {
					return err
				}
				config.LockoutDuration = dur
			}
		}
	}
	return nil
}

// applyEncryptionConfig applies encryption configuration.
func applyEncryptionConfig(node *yamlNode, config *EncryptionConfig) error {
	for _, child := range node.children {
		switch child.key {
		case "enabled":
			config.Enabled = parseBool(child.value)
		case "keyFile":
			if child.value != "" {
				config.KeyFile = child.value
			}
		}
	}
	return nil
}

// applyACLConfig applies ACL configuration.
func applyACLConfig(node *yamlNode, config *ACLConfig) error {
	for _, child := range node.children {
		switch child.key {
		case "defaultPolicy":
			if child.value != "" {
				config.DefaultPolicy = child.value
			}
		case "rules":
			rules, err := parseACLRules(child)
			if err != nil {
				return err
			}
			config.Rules = rules
		}
	}
	return nil
}

// parseACLRules parses ACL rules from a YAML node.
func parseACLRules(node *yamlNode) ([]ACLRuleConfig, error) {
	var rules []ACLRuleConfig

	for _, child := range node.children {
		rule := ACLRuleConfig{}

		for _, ruleChild := range child.children {
			switch ruleChild.key {
			case "target":
				rule.Target = ruleChild.value
			case "subject":
				rule.Subject = ruleChild.value
			case "rights":
				// Try inline array first, then list items
				if inlineArr := parseInlineArray(ruleChild.value); inlineArr != nil {
					rule.Rights = inlineArr
				} else if len(ruleChild.listItems) > 0 {
					rule.Rights = ruleChild.listItems
				}
			case "attributes":
				// Try inline array first, then list items
				if inlineArr := parseInlineArray(ruleChild.value); inlineArr != nil {
					rule.Attributes = inlineArr
				} else if len(ruleChild.listItems) > 0 {
					rule.Attributes = ruleChild.listItems
				}
			}
		}

		rules = append(rules, rule)
	}

	return rules, nil
}

// applyRESTConfig applies REST API configuration.
func applyRESTConfig(node *yamlNode, config *RESTConfig) error {
	for _, child := range node.children {
		switch child.key {
		case "enabled":
			config.Enabled = parseBool(child.value)
		case "address":
			if child.value != "" {
				config.Address = child.value
			}
		case "tlsAddress":
			if child.value != "" {
				config.TLSAddress = child.value
			}
		case "jwtSecret":
			if child.value != "" {
				config.JWTSecret = child.value
			}
		case "tokenTTL":
			if child.value != "" {
				dur, err := parseDuration(child.value)
				if err != nil {
					return err
				}
				config.TokenTTL = dur
			}
		case "rateLimit":
			if child.value != "" {
				val, err := strconv.Atoi(child.value)
				if err != nil {
					return ErrInvalidNumber
				}
				config.RateLimit = val
			}
		case "corsOrigins":
			if inlineArr := parseInlineArray(child.value); inlineArr != nil {
				config.CORSOrigins = inlineArr
			} else if len(child.listItems) > 0 {
				config.CORSOrigins = child.listItems
			}
		}
	}
	return nil
}

// parseDuration parses a duration string supporting formats like "30s", "5m", "1h", "90d".
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	// Check for day suffix (not supported by time.ParseDuration)
	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		days, err := strconv.Atoi(numStr)
		if err != nil {
			return 0, ErrInvalidDuration
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	// Use standard library for other formats
	dur, err := time.ParseDuration(s)
	if err != nil {
		return 0, ErrInvalidDuration
	}
	return dur, nil
}

// parseBool parses a boolean string.
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "yes" || s == "1" || s == "on"
}

// applyClusterConfig applies cluster configuration.
func applyClusterConfig(node *yamlNode, config *ClusterConfig) error {
	for _, child := range node.children {
		switch child.key {
		case "enabled":
			config.Enabled = parseBool(child.value)
		case "nodeID":
			if child.value != "" {
				val, err := strconv.ParseUint(child.value, 10, 64)
				if err != nil {
					return ErrInvalidNumber
				}
				config.NodeID = val
			}
		case "raftAddr":
			if child.value != "" {
				config.RaftAddr = child.value
			}
		case "peers":
			peers, err := parseClusterPeers(child)
			if err != nil {
				return err
			}
			config.Peers = peers
		case "electionTimeout":
			if child.value != "" {
				dur, err := parseDuration(child.value)
				if err != nil {
					return err
				}
				config.ElectionTimeout = dur
			}
		case "heartbeatTimeout":
			if child.value != "" {
				dur, err := parseDuration(child.value)
				if err != nil {
					return err
				}
				config.HeartbeatTimeout = dur
			}
		case "snapshotInterval":
			if child.value != "" {
				val, err := strconv.ParseUint(child.value, 10, 64)
				if err != nil {
					return ErrInvalidNumber
				}
				config.SnapshotInterval = val
			}
		case "dataDir":
			if child.value != "" {
				config.DataDir = child.value
			}
		}
	}
	return nil
}

// parseClusterPeers parses cluster peer configurations.
func parseClusterPeers(node *yamlNode) ([]PeerConfig, error) {
	var peers []PeerConfig
	for _, child := range node.children {
		peer := PeerConfig{}
		for _, peerChild := range child.children {
			switch peerChild.key {
			case "id":
				val, err := strconv.ParseUint(peerChild.value, 10, 64)
				if err != nil {
					return nil, ErrInvalidNumber
				}
				peer.ID = val
			case "addr":
				peer.Addr = peerChild.value
			}
		}
		if peer.ID > 0 || peer.Addr != "" {
			peers = append(peers, peer)
		}
	}
	return peers, nil
}
