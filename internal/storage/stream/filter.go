package stream

import "strings"

// Scope constants for watch filters.
const (
	ScopeBase     = 0 // Match only the base DN
	ScopeOneLevel = 1 // Match direct children of base DN
	ScopeSubtree  = 2 // Match base DN and all descendants
)

// WatchFilter defines criteria for filtering change events.
type WatchFilter struct {
	// BaseDN is the base distinguished name to watch.
	// Empty string matches all DNs.
	BaseDN string
	// Scope determines which entries relative to BaseDN are matched.
	// 0 = base (exact match), 1 = one level, 2 = subtree
	Scope int
	// Operations filters by operation type. Empty slice matches all operations.
	Operations []OperationType
}

// Matches returns true if the event matches the filter criteria.
func (f *WatchFilter) Matches(event *ChangeEvent) bool {
	if event == nil {
		return false
	}

	// Check operation filter
	if len(f.Operations) > 0 {
		matched := false
		for _, op := range f.Operations {
			if event.Operation == op {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check DN filter
	if f.BaseDN == "" {
		return true
	}

	dn := strings.ToLower(event.DN)
	baseDN := strings.ToLower(f.BaseDN)

	switch f.Scope {
	case ScopeBase:
		return dn == baseDN
	case ScopeOneLevel:
		return isDirectChild(dn, baseDN)
	case ScopeSubtree:
		return dn == baseDN || strings.HasSuffix(dn, ","+baseDN)
	}

	return false
}

// isDirectChild returns true if dn is a direct child of baseDN.
func isDirectChild(dn, baseDN string) bool {
	suffix := "," + baseDN
	if !strings.HasSuffix(dn, suffix) {
		return false
	}
	prefix := strings.TrimSuffix(dn, suffix)
	return prefix != "" && !strings.Contains(prefix, ",")
}

// MatchAll returns a filter that matches all events.
func MatchAll() WatchFilter {
	return WatchFilter{}
}

// MatchDN returns a filter that matches events for a specific DN.
func MatchDN(dn string) WatchFilter {
	return WatchFilter{
		BaseDN: dn,
		Scope:  ScopeBase,
	}
}

// MatchSubtree returns a filter that matches events under a base DN.
func MatchSubtree(baseDN string) WatchFilter {
	return WatchFilter{
		BaseDN: baseDN,
		Scope:  ScopeSubtree,
	}
}
