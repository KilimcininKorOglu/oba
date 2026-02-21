package acl

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Loader errors.
var (
	ErrFileNotFound    = errors.New("acl: file not found")
	ErrInvalidYAML     = errors.New("acl: invalid YAML format")
	ErrInvalidVersion  = errors.New("acl: invalid version")
	ErrInvalidPolicy   = errors.New("acl: invalid default policy")
	ErrInvalidRight    = errors.New("acl: invalid right")
	ErrInvalidScope    = errors.New("acl: invalid scope")
	ErrMissingTarget   = errors.New("acl: missing target")
	ErrMissingSubject  = errors.New("acl: missing subject")
	ErrMissingRights   = errors.New("acl: missing rights")
)

// FileConfig represents the ACL file structure.
type FileConfig struct {
	Version       int              `yaml:"version"`
	DefaultPolicy string           `yaml:"defaultPolicy"`
	Rules         []FileRuleConfig `yaml:"rules"`
}

// FileRuleConfig represents a single rule in the ACL file.
type FileRuleConfig struct {
	Target     string   `yaml:"target"`
	Subject    string   `yaml:"subject"`
	Scope      string   `yaml:"scope"`
	Rights     []string `yaml:"rights"`
	Attributes []string `yaml:"attributes"`
	Deny       bool     `yaml:"deny"`
}

// LoadFromFile loads ACL configuration from a YAML file.
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("acl: failed to read file: %w", err)
	}

	return ParseACLYAML(data)
}

// ParseACLYAML parses ACL configuration from YAML bytes.
func ParseACLYAML(data []byte) (*Config, error) {
	// Substitute environment variables
	data = substituteEnvVars(data)

	fileConfig, err := parseACLYAMLToFileConfig(data)
	if err != nil {
		return nil, err
	}

	return convertFileConfig(fileConfig)
}

// substituteEnvVars replaces ${VAR} and ${VAR:-default} patterns.
func substituteEnvVars(data []byte) []byte {
	re := regexp.MustCompile(`\$\{([^}]+)\}`)

	return re.ReplaceAllFunc(data, func(match []byte) []byte {
		content := string(match[2 : len(match)-1])

		if idx := strings.Index(content, ":-"); idx != -1 {
			varName := content[:idx]
			defaultVal := content[idx+2:]
			if val := os.Getenv(varName); val != "" {
				return []byte(val)
			}
			return []byte(defaultVal)
		}

		return []byte(os.Getenv(content))
	})
}

// parseACLYAMLToFileConfig parses YAML into FileConfig.
func parseACLYAMLToFileConfig(data []byte) (*FileConfig, error) {
	fc := &FileConfig{
		Version:       1,
		DefaultPolicy: "deny",
		Rules:         make([]FileRuleConfig, 0),
	}

	lines := strings.Split(string(data), "\n")
	var currentRule *FileRuleConfig
	var inRules bool
	var inAttributes bool
	var inRights bool
	var ruleIndent int

	for _, line := range lines {
		// Skip empty lines and comments
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		// Top-level keys
		if indent == 0 {
			inRules = false
			inAttributes = false
			inRights = false

			if strings.HasPrefix(trimmed, "version:") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "version:"))
				v, err := strconv.Atoi(val)
				if err != nil {
					return nil, fmt.Errorf("%w: %s", ErrInvalidVersion, val)
				}
				fc.Version = v
			} else if strings.HasPrefix(trimmed, "defaultPolicy:") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "defaultPolicy:"))
				val = strings.Trim(val, "\"'")
				fc.DefaultPolicy = val
			} else if trimmed == "rules:" {
				inRules = true
			}
			continue
		}

		// Inside rules section
		if inRules {
			// New rule starts with "- " at rules level (indent 2)
			if strings.HasPrefix(trimmed, "- ") && indent <= 2 {
				// Check if this is a new rule or a list item
				rest := strings.TrimPrefix(trimmed, "- ")
				
				// If it contains ":" it's a new rule with inline key
				if strings.Contains(rest, ":") && !strings.HasPrefix(rest, "[") {
					// Save previous rule
					if currentRule != nil {
						fc.Rules = append(fc.Rules, *currentRule)
					}
					currentRule = &FileRuleConfig{}
					inAttributes = false
					inRights = false
					ruleIndent = indent

					parseRuleKeyValue(currentRule, rest, &inAttributes, &inRights)
					continue
				}
				
				// It's a new rule without inline key
				if currentRule != nil {
					fc.Rules = append(fc.Rules, *currentRule)
				}
				currentRule = &FileRuleConfig{}
				inAttributes = false
				inRights = false
				ruleIndent = indent
				continue
			}

			// Rule properties or list items
			if currentRule != nil {
				// List items for attributes or rights (deeper indent)
				if strings.HasPrefix(trimmed, "- ") && indent > ruleIndent {
					val := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
					val = strings.Trim(val, "\"'")
					if inAttributes {
						currentRule.Attributes = append(currentRule.Attributes, val)
					} else if inRights {
						currentRule.Rights = append(currentRule.Rights, val)
					}
					continue
				}

				// Key-value pairs for rule properties
				if strings.Contains(trimmed, ":") && !strings.HasPrefix(trimmed, "- ") {
					parseRuleKeyValue(currentRule, trimmed, &inAttributes, &inRights)
				}
			}
		}
	}

	// Save last rule
	if currentRule != nil {
		fc.Rules = append(fc.Rules, *currentRule)
	}

	return fc, nil
}

// parseRuleKeyValue parses a key: value pair for a rule.
func parseRuleKeyValue(rule *FileRuleConfig, line string, inAttributes, inRights *bool) {
	*inAttributes = false
	*inRights = false

	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return
	}

	key := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])
	val = strings.Trim(val, "\"'")

	switch key {
	case "target":
		rule.Target = val
	case "subject":
		rule.Subject = val
	case "scope":
		rule.Scope = val
	case "deny":
		rule.Deny = val == "true" || val == "yes"
	case "rights":
		*inRights = true
		// Check for inline array: [read, write]
		if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
			inner := val[1 : len(val)-1]
			for _, r := range strings.Split(inner, ",") {
				r = strings.TrimSpace(r)
				r = strings.Trim(r, "\"'")
				if r != "" {
					rule.Rights = append(rule.Rights, r)
				}
			}
			*inRights = false
		}
	case "attributes":
		*inAttributes = true
		// Check for inline array
		if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
			inner := val[1 : len(val)-1]
			for _, a := range strings.Split(inner, ",") {
				a = strings.TrimSpace(a)
				a = strings.Trim(a, "\"'")
				if a != "" {
					rule.Attributes = append(rule.Attributes, a)
				}
			}
			*inAttributes = false
		}
	}
}

// convertFileConfig converts FileConfig to acl.Config.
func convertFileConfig(fc *FileConfig) (*Config, error) {
	// Validate version
	if fc.Version < 1 {
		return nil, fmt.Errorf("%w: must be >= 1", ErrInvalidVersion)
	}

	// Validate default policy
	policy := strings.ToLower(fc.DefaultPolicy)
	if policy != "allow" && policy != "deny" && policy != "" {
		return nil, fmt.Errorf("%w: %s (must be allow or deny)", ErrInvalidPolicy, fc.DefaultPolicy)
	}

	config := NewConfig()
	if policy != "" {
		config.SetDefaultPolicy(policy)
	}

	for i, rule := range fc.Rules {
		aclRule, err := convertRule(&rule, i)
		if err != nil {
			return nil, err
		}
		config.AddRule(aclRule)
	}

	return config, nil
}

// convertRule converts FileRuleConfig to ACL.
func convertRule(r *FileRuleConfig, index int) (*ACL, error) {
	// Validate required fields
	if r.Target == "" {
		return nil, fmt.Errorf("rule %d: %w", index, ErrMissingTarget)
	}
	if r.Subject == "" {
		return nil, fmt.Errorf("rule %d: %w", index, ErrMissingSubject)
	}
	if len(r.Rights) == 0 {
		return nil, fmt.Errorf("rule %d: %w", index, ErrMissingRights)
	}

	rights, err := ParseRights(r.Rights)
	if err != nil {
		return nil, fmt.Errorf("rule %d: %w", index, err)
	}

	acl := NewACL(r.Target, r.Subject, rights)

	// Scope
	if r.Scope != "" {
		scope, err := ParseScope(r.Scope)
		if err != nil {
			return nil, fmt.Errorf("rule %d: %w", index, err)
		}
		acl.WithScope(scope)
	}

	// Attributes
	if len(r.Attributes) > 0 {
		acl.WithAttributes(r.Attributes...)
	}

	// Deny
	acl.WithDeny(r.Deny)

	return acl, nil
}

// ParseRights converts string rights to Right flags.
func ParseRights(rights []string) (Right, error) {
	var result Right

	for _, r := range rights {
		switch strings.ToLower(strings.TrimSpace(r)) {
		case "read":
			result |= Read
		case "write":
			result |= Write
		case "add":
			result |= Add
		case "delete":
			result |= Delete
		case "search":
			result |= Search
		case "compare":
			result |= Compare
		case "all":
			result |= All
		default:
			return 0, fmt.Errorf("%w: %s", ErrInvalidRight, r)
		}
	}

	return result, nil
}

// ParseScope converts string scope to Scope.
func ParseScope(s string) (Scope, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "base":
		return ScopeBase, nil
	case "one", "onelevel":
		return ScopeOne, nil
	case "sub", "subtree", "":
		return ScopeSubtree, nil
	default:
		return 0, fmt.Errorf("%w: %s", ErrInvalidScope, s)
	}
}
