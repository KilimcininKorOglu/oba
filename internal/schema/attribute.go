package schema

// AttributeUsage defines how an attribute is used in the directory.
// This determines whether the attribute is user-modifiable and its scope.
type AttributeUsage int

const (
	// UserApplications indicates a user attribute that applications can read and write.
	// This is the default usage for most attributes.
	UserApplications AttributeUsage = iota

	// DirectoryOperation indicates an operational attribute used by the directory
	// for its own purposes. These are typically read-only for users.
	DirectoryOperation

	// DistributedOperation indicates an operational attribute that is shared
	// across multiple directory servers in a distributed environment.
	DistributedOperation

	// DSAOperation indicates an operational attribute specific to a single
	// Directory System Agent (DSA). These are local to each server.
	DSAOperation
)

// String returns the string representation of the AttributeUsage.
func (u AttributeUsage) String() string {
	switch u {
	case UserApplications:
		return "userApplications"
	case DirectoryOperation:
		return "directoryOperation"
	case DistributedOperation:
		return "distributedOperation"
	case DSAOperation:
		return "dSAOperation"
	default:
		return "unknown"
	}
}

// IsOperational returns true if this usage indicates an operational attribute.
func (u AttributeUsage) IsOperational() bool {
	return u != UserApplications
}

// AttributeType represents an LDAP attribute type definition.
// Attribute types define the syntax and constraints for attribute values.
type AttributeType struct {
	OID         string         // Object Identifier (e.g., "2.5.4.3")
	Name        string         // Primary name (e.g., "cn")
	Names       []string       // All names including aliases (e.g., ["cn", "commonName"])
	Desc        string         // Human-readable description
	Obsolete    bool           // Whether this attribute type is obsolete
	Superior    string         // Parent attribute type name or OID
	Equality    string         // Matching rule OID/name for equality matching
	Ordering    string         // Matching rule OID/name for ordering matching
	Substring   string         // Matching rule OID/name for substring matching
	Syntax      string         // Syntax OID (e.g., "1.3.6.1.4.1.1466.115.121.1.15")
	SingleValue bool           // If true, attribute can have only one value
	Collective  bool           // If true, attribute is collective
	NoUserMod   bool           // If true, attribute cannot be modified by users
	Usage       AttributeUsage // How the attribute is used
}

// NewAttributeType creates a new AttributeType with the given OID and name.
// The default usage is UserApplications.
func NewAttributeType(oid, name string) *AttributeType {
	return &AttributeType{
		OID:   oid,
		Name:  name,
		Names: []string{name},
		Usage: UserApplications,
	}
}

// IsUserAttribute returns true if this is a user-modifiable attribute.
func (at *AttributeType) IsUserAttribute() bool {
	return at.Usage == UserApplications && !at.NoUserMod
}

// IsOperational returns true if this is an operational attribute.
func (at *AttributeType) IsOperational() bool {
	return at.Usage.IsOperational()
}

// IsReadOnly returns true if users cannot modify this attribute.
func (at *AttributeType) IsReadOnly() bool {
	return at.NoUserMod
}

// IsSingleValued returns true if this attribute can have only one value.
func (at *AttributeType) IsSingleValued() bool {
	return at.SingleValue
}

// IsMultiValued returns true if this attribute can have multiple values.
func (at *AttributeType) IsMultiValued() bool {
	return !at.SingleValue
}

// HasEqualityMatching returns true if this attribute supports equality matching.
func (at *AttributeType) HasEqualityMatching() bool {
	return at.Equality != ""
}

// HasOrderingMatching returns true if this attribute supports ordering matching.
func (at *AttributeType) HasOrderingMatching() bool {
	return at.Ordering != ""
}

// HasSubstringMatching returns true if this attribute supports substring matching.
func (at *AttributeType) HasSubstringMatching() bool {
	return at.Substring != ""
}

// AddName adds an alias name to this attribute type.
func (at *AttributeType) AddName(name string) {
	for _, n := range at.Names {
		if n == name {
			return
		}
	}
	at.Names = append(at.Names, name)
}

// SetSyntax sets the syntax OID for this attribute type.
func (at *AttributeType) SetSyntax(syntaxOID string) {
	at.Syntax = syntaxOID
}

// SetMatchingRules sets the matching rules for this attribute type.
func (at *AttributeType) SetMatchingRules(equality, ordering, substring string) {
	at.Equality = equality
	at.Ordering = ordering
	at.Substring = substring
}
