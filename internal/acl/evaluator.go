// Package acl provides Access Control List (ACL) data structures and evaluation
// for the Oba LDAP server.
package acl

// Entry represents an LDAP entry for attribute filtering.
// This is a simplified interface to avoid circular dependencies.
type Entry struct {
	DN         string
	Attributes map[string][]string
}

// NewEntry creates a new Entry with the given DN.
func NewEntry(dn string) *Entry {
	return &Entry{
		DN:         dn,
		Attributes: make(map[string][]string),
	}
}

// SetAttribute sets an attribute value on the entry.
func (e *Entry) SetAttribute(name string, values ...string) {
	e.Attributes[name] = values
}

// GetAttribute returns the values for an attribute.
func (e *Entry) GetAttribute(name string) []string {
	return e.Attributes[name]
}

// HasAttribute checks if the entry has the given attribute.
func (e *Entry) HasAttribute(name string) bool {
	_, ok := e.Attributes[name]
	return ok
}

// Clone creates a deep copy of the entry.
func (e *Entry) Clone() *Entry {
	if e == nil {
		return nil
	}

	clone := &Entry{
		DN:         e.DN,
		Attributes: make(map[string][]string, len(e.Attributes)),
	}

	for k, v := range e.Attributes {
		values := make([]string, len(v))
		copy(values, v)
		clone.Attributes[k] = values
	}

	return clone
}

// Evaluator evaluates ACL rules to determine access permissions.
type Evaluator struct {
	config  *Config
	matcher *Matcher
}

// NewEvaluator creates a new ACL evaluator with the given configuration.
func NewEvaluator(config *Config) *Evaluator {
	if config == nil {
		config = NewConfig()
	}

	return &Evaluator{
		config:  config,
		matcher: NewMatcher(),
	}
}

// CheckAccess determines if the operation is allowed based on ACL rules.
// Uses first-match-wins semantics: the first matching rule determines access.
// If no rules match, the default policy is applied.
func (e *Evaluator) CheckAccess(ctx *AccessContext) bool {
	if ctx == nil {
		return e.config.IsDefaultAllow()
	}

	for _, rule := range e.config.Rules {
		// Check if the rule matches the target DN
		if !e.matcher.MatchesTarget(rule, ctx.TargetDN) {
			continue
		}

		// Check if the rule matches the subject (bind DN)
		if !e.matcher.MatchesSubject(rule, ctx.BindDN, ctx.TargetDN) {
			continue
		}

		// Check if the rule applies to the requested operation
		if !rule.Rights.Has(ctx.Operation) {
			continue
		}

		// Rule matches target, subject, and operation
		// First-match-wins: return based on deny/allow
		if rule.Deny {
			return false
		}

		return true
	}

	// No matching rule found - apply default policy
	return e.config.IsDefaultAllow()
}

// CheckAttributeAccess checks if access to a specific attribute is allowed.
func (e *Evaluator) CheckAttributeAccess(ctx *AccessContext, attr string) bool {
	if ctx == nil {
		return e.config.IsDefaultAllow()
	}

	for _, rule := range e.config.Rules {
		// Check if the rule matches the target DN
		if !e.matcher.MatchesTarget(rule, ctx.TargetDN) {
			continue
		}

		// Check if the rule matches the subject (bind DN)
		if !e.matcher.MatchesSubject(rule, ctx.BindDN, ctx.TargetDN) {
			continue
		}

		// Check if the rule applies to this attribute
		if !rule.AppliesToAttribute(attr) {
			continue
		}

		// Check if the rule applies to the requested operation
		if !rule.Rights.Has(ctx.Operation) {
			continue
		}

		// Rule matches target, subject, attribute, and operation
		// First-match-wins: return based on deny/allow
		if rule.Deny {
			return false
		}

		return true
	}

	// No matching rule found - apply default policy
	return e.config.IsDefaultAllow()
}

// FilterAttributes returns a new entry with only the attributes the user can read.
// This is used to filter search results based on read permissions.
func (e *Evaluator) FilterAttributes(ctx *AccessContext, entry *Entry) *Entry {
	if entry == nil {
		return nil
	}

	if ctx == nil {
		if e.config.IsDefaultAllow() {
			return entry.Clone()
		}
		return NewEntry(entry.DN)
	}

	// Create a read context for attribute filtering
	readCtx := &AccessContext{
		BindDN:    ctx.BindDN,
		TargetDN:  entry.DN,
		Operation: Read,
	}

	filtered := NewEntry(entry.DN)

	for attrName, values := range entry.Attributes {
		if e.CheckAttributeAccess(readCtx, attrName) {
			// Copy the values
			filteredValues := make([]string, len(values))
			copy(filteredValues, values)
			filtered.Attributes[attrName] = filteredValues
		}
	}

	return filtered
}

// FilterAttributeList returns a list of attributes the user can access.
// This is used to filter requested attributes in search operations.
func (e *Evaluator) FilterAttributeList(ctx *AccessContext, attrs []string) []string {
	if ctx == nil {
		if e.config.IsDefaultAllow() {
			return attrs
		}
		return nil
	}

	readCtx := &AccessContext{
		BindDN:    ctx.BindDN,
		TargetDN:  ctx.TargetDN,
		Operation: Read,
	}

	filtered := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		if e.CheckAttributeAccess(readCtx, attr) {
			filtered = append(filtered, attr)
		}
	}

	return filtered
}

// GetConfig returns the evaluator's configuration.
func (e *Evaluator) GetConfig() *Config {
	return e.config
}

// SetConfig updates the evaluator's configuration.
func (e *Evaluator) SetConfig(config *Config) {
	if config == nil {
		config = NewConfig()
	}
	e.config = config
}

// CanRead checks if the user can read the target entry.
func (e *Evaluator) CanRead(bindDN, targetDN string) bool {
	return e.CheckAccess(&AccessContext{
		BindDN:    bindDN,
		TargetDN:  targetDN,
		Operation: Read,
	})
}

// CanWrite checks if the user can write to the target entry.
func (e *Evaluator) CanWrite(bindDN, targetDN string) bool {
	return e.CheckAccess(&AccessContext{
		BindDN:    bindDN,
		TargetDN:  targetDN,
		Operation: Write,
	})
}

// CanAdd checks if the user can add entries under the target DN.
func (e *Evaluator) CanAdd(bindDN, targetDN string) bool {
	return e.CheckAccess(&AccessContext{
		BindDN:    bindDN,
		TargetDN:  targetDN,
		Operation: Add,
	})
}

// CanDelete checks if the user can delete the target entry.
func (e *Evaluator) CanDelete(bindDN, targetDN string) bool {
	return e.CheckAccess(&AccessContext{
		BindDN:    bindDN,
		TargetDN:  targetDN,
		Operation: Delete,
	})
}

// CanSearch checks if the user can search the target entry.
func (e *Evaluator) CanSearch(bindDN, targetDN string) bool {
	return e.CheckAccess(&AccessContext{
		BindDN:    bindDN,
		TargetDN:  targetDN,
		Operation: Search,
	})
}

// CanCompare checks if the user can compare attributes on the target entry.
func (e *Evaluator) CanCompare(bindDN, targetDN string) bool {
	return e.CheckAccess(&AccessContext{
		BindDN:    bindDN,
		TargetDN:  targetDN,
		Operation: Compare,
	})
}
