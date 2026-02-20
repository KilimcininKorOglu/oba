// Package backend provides the LDAP backend interface that wraps the storage engine
// and provides LDAP-specific operations including authentication, entry validation,
// and coordination with the storage layer.
package backend

import (
	"fmt"
	"time"
)

// Operational attribute constants per RFC 4512.
const (
	// AttrCreateTimestamp is the creation timestamp of an entry.
	AttrCreateTimestamp = "createTimestamp"
	// AttrModifyTimestamp is the last modification timestamp of an entry.
	AttrModifyTimestamp = "modifyTimestamp"
	// AttrCreatorsName is the DN of the entry creator.
	AttrCreatorsName = "creatorsName"
	// AttrModifiersName is the DN of the last modifier.
	AttrModifiersName = "modifiersName"
	// AttrEntryDN is the DN of the entry itself.
	AttrEntryDN = "entryDN"
	// AttrEntryUUID is the unique identifier of the entry (RFC 4530).
	AttrEntryUUID = "entryUUID"
	// AttrSubschemaSubentry is the DN of the applicable schema.
	AttrSubschemaSubentry = "subschemaSubentry"
	// AttrHasSubordinates indicates whether the entry has children.
	AttrHasSubordinates = "hasSubordinates"
	// AttrNumSubordinates is the count of immediate children.
	AttrNumSubordinates = "numSubordinates"
)

// OperationalAttrs holds operational attribute values for an entry.
type OperationalAttrs struct {
	// CreateTimestamp is the time when the entry was created.
	CreateTimestamp time.Time
	// ModifyTimestamp is the time when the entry was last modified.
	ModifyTimestamp time.Time
	// CreatorsName is the DN of the user who created the entry.
	CreatorsName string
	// ModifiersName is the DN of the user who last modified the entry.
	ModifiersName string
	// EntryUUID is the unique identifier for the entry.
	EntryUUID string
	// HasSubordinates indicates whether the entry has child entries.
	HasSubordinates bool
	// NumSubordinates is the number of immediate child entries.
	NumSubordinates int
}

// OperationType represents the type of operation being performed.
type OperationType string

// Operation type constants.
const (
	// OpAdd represents an add operation.
	OpAdd OperationType = "add"
	// OpModify represents a modify operation.
	OpModify OperationType = "modify"
)

// SetOperationalAttrs sets operational attributes on an entry based on the operation type.
// For add operations, it sets createTimestamp, creatorsName, and entryUUID.
// For both add and modify operations, it sets modifyTimestamp and modifiersName.
// The entryDN is always set to the entry's DN.
func SetOperationalAttrs(entry *Entry, op OperationType, bindDN string) {
	if entry == nil {
		return
	}

	now := time.Now().UTC()

	switch op {
	case OpAdd:
		// Set creation-specific attributes
		entry.SetAttribute(AttrCreateTimestamp, FormatTimestamp(now))
		entry.SetAttribute(AttrCreatorsName, bindDN)
		entry.SetAttribute(AttrEntryUUID, GenerateUUID())
		// Fall through to also set modification attributes
		fallthrough
	case OpModify:
		// Set modification attributes
		entry.SetAttribute(AttrModifyTimestamp, FormatTimestamp(now))
		entry.SetAttribute(AttrModifiersName, bindDN)
	}

	// Always set entryDN
	entry.SetAttribute(AttrEntryDN, entry.DN)
}

// SetSubordinateAttrs sets the hasSubordinates and numSubordinates attributes on an entry.
func SetSubordinateAttrs(entry *Entry, hasSubordinates bool, numSubordinates int) {
	if entry == nil {
		return
	}

	if hasSubordinates {
		entry.SetAttribute(AttrHasSubordinates, "TRUE")
	} else {
		entry.SetAttribute(AttrHasSubordinates, "FALSE")
	}

	entry.SetAttribute(AttrNumSubordinates, formatInt(numSubordinates))
}

// FormatTimestamp formats a time.Time as an LDAP GeneralizedTime string.
// The format is YYYYMMDDHHmmssZ (e.g., "20260218103000Z").
func FormatTimestamp(t time.Time) string {
	return t.UTC().Format("20060102150405Z")
}

// ParseTimestamp parses an LDAP GeneralizedTime string into a time.Time.
// Returns the zero time if parsing fails.
func ParseTimestamp(s string) time.Time {
	t, err := time.Parse("20060102150405Z", s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// formatInt formats an integer as a string.
func formatInt(n int) string {
	return fmt.Sprintf("%d", n)
}
