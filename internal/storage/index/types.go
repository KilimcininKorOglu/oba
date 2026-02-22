// Package index provides the Index Manager for coordinating multiple B+ Tree indexes
// for different attributes in ObaDB.
package index

import (
	"strings"

	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/btree"
)

// IndexType represents the type of index for attribute searching.
type IndexType int

const (
	// IndexEquality supports equality searches like (uid=alice).
	IndexEquality IndexType = iota
	// IndexPresence supports presence searches like (mail=*).
	IndexPresence
	// IndexSubstring supports substring searches like (cn=*admin*).
	IndexSubstring
)

// String returns the string representation of an IndexType.
func (t IndexType) String() string {
	switch t {
	case IndexEquality:
		return "equality"
	case IndexPresence:
		return "presence"
	case IndexSubstring:
		return "substring"
	default:
		return "unknown"
	}
}

// Index represents a B+ Tree index for a specific attribute.
type Index struct {
	// Attribute is the name of the indexed attribute (e.g., "uid", "mail", "cn").
	Attribute string

	// Type is the type of index (equality, presence, substring).
	Type IndexType

	// Tree is the underlying B+ Tree for this index.
	Tree *btree.BPlusTree

	// RootPageID is the root page ID of the B+ Tree (for persistence).
	RootPageID storage.PageID
}

// Entry represents an LDAP entry for index maintenance.
// This is a simplified interface to avoid circular dependencies.
type Entry struct {
	// DN is the distinguished name of the entry.
	DN string

	// Attributes contains the entry's attribute values.
	// Key is the attribute name (lowercase), value is a slice of attribute values.
	Attributes map[string][][]byte

	// PageID is the page where this entry is stored.
	PageID storage.PageID

	// SlotID is the slot within the page where this entry is stored.
	SlotID uint16
}

// NewEntry creates a new Entry with the given DN.
func NewEntry(dn string) *Entry {
	return &Entry{
		DN:         dn,
		Attributes: make(map[string][][]byte),
	}
}

// GetAttribute returns the values for the given attribute name (case-insensitive).
// Returns nil if the attribute doesn't exist.
func (e *Entry) GetAttribute(name string) [][]byte {
	if e.Attributes == nil {
		return nil
	}
	name = strings.ToLower(name)
	for k, v := range e.Attributes {
		if strings.ToLower(k) == name {
			return v
		}
	}
	return nil
}

// HasAttribute returns true if the entry has the given attribute.
func (e *Entry) HasAttribute(name string) bool {
	if e.Attributes == nil {
		return false
	}
	_, ok := e.Attributes[name]
	return ok
}

// SetAttribute sets the values for the given attribute name.
func (e *Entry) SetAttribute(name string, values [][]byte) {
	if e.Attributes == nil {
		e.Attributes = make(map[string][][]byte)
	}
	e.Attributes[name] = values
}

// AddAttributeValue adds a value to the given attribute.
func (e *Entry) AddAttributeValue(name string, value []byte) {
	if e.Attributes == nil {
		e.Attributes = make(map[string][][]byte)
	}
	e.Attributes[name] = append(e.Attributes[name], value)
}

// EntryRef returns the btree.EntryRef for this entry.
func (e *Entry) EntryRef() btree.EntryRef {
	return btree.EntryRef{
		PageID: e.PageID,
		SlotID: e.SlotID,
		DN:     e.DN,
	}
}

// IndexMetadata contains metadata about an index for persistence.
type IndexMetadata struct {
	// Attribute is the name of the indexed attribute.
	Attribute string

	// Type is the type of index.
	Type IndexType

	// RootPageID is the root page ID of the B+ Tree.
	RootPageID storage.PageID
}

// DefaultIndexedAttributes returns the list of attributes that should be indexed by default.
// These are commonly searched attributes in LDAP.
func DefaultIndexedAttributes() []string {
	return []string{
		"objectclass",
		"uid",
		"cn",
		"sn",
		"mail",
		"memberof",
	}
}

// PresenceMarker is a special value used for presence indexes.
// When an attribute exists, we index this marker value.
var PresenceMarker = []byte{0x01}
