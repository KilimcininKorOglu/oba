// Package backend provides the LDAP backend interface that wraps the storage engine
// and provides LDAP-specific operations including authentication, entry validation,
// and coordination with the storage layer.
package backend

import (
	"strings"
)

// Entry represents an LDAP entry with multi-valued attributes.
// This is the backend's representation of an entry, using string values
// for easier manipulation in LDAP operations.
type Entry struct {
	// DN is the distinguished name of the entry.
	DN string

	// Attributes contains the entry's attribute values.
	// Key is the attribute name, value is a slice of string values.
	Attributes map[string][]string
}

// NewEntry creates a new Entry with the given DN.
func NewEntry(dn string) *Entry {
	return &Entry{
		DN:         dn,
		Attributes: make(map[string][]string),
	}
}

// GetAttribute returns the values for the given attribute name.
// Returns nil if the attribute does not exist.
func (e *Entry) GetAttribute(name string) []string {
	if e.Attributes == nil {
		return nil
	}
	return e.Attributes[strings.ToLower(name)]
}

// GetFirstAttribute returns the first value for the given attribute name.
// Returns an empty string if the attribute does not exist or has no values.
func (e *Entry) GetFirstAttribute(name string) string {
	values := e.GetAttribute(name)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// HasAttribute returns true if the entry has the given attribute.
func (e *Entry) HasAttribute(name string) bool {
	if e.Attributes == nil {
		return false
	}
	values, ok := e.Attributes[strings.ToLower(name)]
	return ok && len(values) > 0
}

// SetAttribute sets the values for the given attribute name.
// The attribute name is normalized to lowercase.
func (e *Entry) SetAttribute(name string, values ...string) {
	if e.Attributes == nil {
		e.Attributes = make(map[string][]string)
	}
	e.Attributes[strings.ToLower(name)] = values
}

// AddAttributeValue adds a value to the given attribute.
// The attribute name is normalized to lowercase.
func (e *Entry) AddAttributeValue(name string, value string) {
	if e.Attributes == nil {
		e.Attributes = make(map[string][]string)
	}
	name = strings.ToLower(name)
	e.Attributes[name] = append(e.Attributes[name], value)
}

// DeleteAttribute removes an attribute from the entry.
// The attribute name is normalized to lowercase.
func (e *Entry) DeleteAttribute(name string) {
	if e.Attributes == nil {
		return
	}
	delete(e.Attributes, strings.ToLower(name))
}

// DeleteAttributeValue removes a specific value from an attribute.
// If the attribute has no more values after removal, the attribute is deleted.
// The attribute name is normalized to lowercase.
func (e *Entry) DeleteAttributeValue(name string, value string) {
	if e.Attributes == nil {
		return
	}
	name = strings.ToLower(name)
	values := e.Attributes[name]
	if len(values) == 0 {
		return
	}

	newValues := make([]string, 0, len(values))
	for _, v := range values {
		if v != value {
			newValues = append(newValues, v)
		}
	}

	if len(newValues) == 0 {
		delete(e.Attributes, name)
	} else {
		e.Attributes[name] = newValues
	}
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

// AttributeNames returns a list of all attribute names in the entry.
func (e *Entry) AttributeNames() []string {
	if e.Attributes == nil {
		return nil
	}

	names := make([]string, 0, len(e.Attributes))
	for name := range e.Attributes {
		names = append(names, name)
	}
	return names
}

// Modification represents a single modification to an entry.
type Modification struct {
	// Type is the type of modification (add, delete, replace).
	Type ModificationType

	// Attribute is the name of the attribute to modify.
	Attribute string

	// Values are the values to add, delete, or replace.
	Values []string
}

// ModificationType represents the type of modification operation.
type ModificationType int

const (
	// ModAdd adds values to an attribute.
	ModAdd ModificationType = iota
	// ModDelete removes values from an attribute.
	ModDelete
	// ModReplace replaces all values of an attribute.
	ModReplace
)

// String returns the string representation of the modification type.
func (m ModificationType) String() string {
	switch m {
	case ModAdd:
		return "add"
	case ModDelete:
		return "delete"
	case ModReplace:
		return "replace"
	default:
		return "unknown"
	}
}

// NewModification creates a new Modification.
func NewModification(modType ModificationType, attr string, values ...string) *Modification {
	return &Modification{
		Type:      modType,
		Attribute: attr,
		Values:    values,
	}
}
