// Package acl provides Access Control List (ACL) data structures and evaluation
// for the Oba LDAP server.
package acl

// Right represents an LDAP access control right.
// Rights are bit flags that can be combined using bitwise OR.
type Right int

const (
	// Read allows reading entry attributes
	Read Right = 1 << iota

	// Write allows modifying entry attributes
	Write

	// Add allows creating new entries
	Add

	// Delete allows removing entries
	Delete

	// Search allows searching for entries
	Search

	// Compare allows comparing attribute values
	Compare

	// All combines all rights
	All = Read | Write | Add | Delete | Search | Compare
)

// String returns a human-readable representation of the right.
func (r Right) String() string {
	switch r {
	case Read:
		return "read"
	case Write:
		return "write"
	case Add:
		return "add"
	case Delete:
		return "delete"
	case Search:
		return "search"
	case Compare:
		return "compare"
	case All:
		return "all"
	default:
		return "unknown"
	}
}

// Has checks if the right includes the specified right.
func (r Right) Has(other Right) bool {
	return r&other != 0
}

// Scope represents the scope of an ACL rule.
type Scope int

const (
	// ScopeBase applies only to the target entry itself
	ScopeBase Scope = iota

	// ScopeOne applies to immediate children of the target
	ScopeOne

	// ScopeSubtree applies to the target and all descendants
	ScopeSubtree
)

// String returns a human-readable representation of the scope.
func (s Scope) String() string {
	switch s {
	case ScopeBase:
		return "base"
	case ScopeOne:
		return "one"
	case ScopeSubtree:
		return "subtree"
	default:
		return "unknown"
	}
}

// ACL represents a single access control rule.
type ACL struct {
	// Target is the DN pattern this rule applies to.
	// Can be a specific DN, a pattern with wildcards, or "*" for all entries.
	Target string

	// Scope defines how the target DN is interpreted.
	Scope Scope

	// Subject defines who this rule applies to.
	// Can be "anonymous", "authenticated", "self", a specific DN, or "*" for everyone.
	Subject string

	// Rights defines what operations are allowed or denied.
	Rights Right

	// Attributes specifies which attributes this rule applies to.
	// Empty slice means all attributes.
	Attributes []string

	// Deny indicates this is a deny rule (true) or allow rule (false).
	Deny bool
}

// NewACL creates a new ACL rule with the given parameters.
func NewACL(target, subject string, rights Right) *ACL {
	return &ACL{
		Target:     target,
		Scope:      ScopeSubtree,
		Subject:    subject,
		Rights:     rights,
		Attributes: nil,
		Deny:       false,
	}
}

// WithScope sets the scope and returns the ACL for chaining.
func (a *ACL) WithScope(scope Scope) *ACL {
	a.Scope = scope
	return a
}

// WithAttributes sets the attributes and returns the ACL for chaining.
func (a *ACL) WithAttributes(attrs ...string) *ACL {
	a.Attributes = attrs
	return a
}

// WithDeny sets the deny flag and returns the ACL for chaining.
func (a *ACL) WithDeny(deny bool) *ACL {
	a.Deny = deny
	return a
}

// AppliesToAttribute checks if this ACL applies to the given attribute.
// Returns true if Attributes is empty (applies to all) or if the attribute is in the list.
func (a *ACL) AppliesToAttribute(attr string) bool {
	if len(a.Attributes) == 0 {
		return true
	}
	for _, allowed := range a.Attributes {
		if allowed == attr || allowed == "*" {
			return true
		}
	}
	return false
}

// Config holds the ACL configuration including default policy and rules.
type Config struct {
	// DefaultPolicy is applied when no rules match.
	// Can be "allow" or "deny". Default is "deny".
	DefaultPolicy string

	// Rules is the ordered list of ACL rules.
	// Rules are evaluated in order; first match wins.
	Rules []*ACL
}

// NewConfig creates a new ACL configuration with default deny policy.
func NewConfig() *Config {
	return &Config{
		DefaultPolicy: "deny",
		Rules:         make([]*ACL, 0),
	}
}

// AddRule appends a rule to the configuration.
func (c *Config) AddRule(rule *ACL) {
	c.Rules = append(c.Rules, rule)
}

// SetDefaultPolicy sets the default policy.
// Valid values are "allow" and "deny".
func (c *Config) SetDefaultPolicy(policy string) {
	c.DefaultPolicy = policy
}

// IsDefaultAllow returns true if the default policy is "allow".
func (c *Config) IsDefaultAllow() bool {
	return c.DefaultPolicy == "allow"
}

// AccessContext provides context for an access control check.
type AccessContext struct {
	// BindDN is the DN of the authenticated user making the request.
	// Empty string indicates anonymous access.
	BindDN string

	// TargetDN is the DN of the entry being accessed.
	TargetDN string

	// Operation is the type of operation being performed.
	Operation Right

	// Attributes is the list of attributes being accessed.
	// Used for read/write operations to check attribute-level permissions.
	Attributes []string
}

// NewAccessContext creates a new access context.
func NewAccessContext(bindDN, targetDN string, operation Right) *AccessContext {
	return &AccessContext{
		BindDN:     bindDN,
		TargetDN:   targetDN,
		Operation:  operation,
		Attributes: nil,
	}
}

// WithAttributes sets the attributes and returns the context for chaining.
func (c *AccessContext) WithAttributes(attrs ...string) *AccessContext {
	c.Attributes = attrs
	return c
}

// IsAnonymous returns true if the request is from an unauthenticated user.
func (c *AccessContext) IsAnonymous() bool {
	return c.BindDN == ""
}

// IsSelf returns true if the bind DN matches the target DN.
func (c *AccessContext) IsSelf() bool {
	return c.BindDN != "" && c.BindDN == c.TargetDN
}
