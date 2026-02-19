package schema

// ObjectClassKind represents the type of an LDAP object class.
// LDAP defines three kinds of object classes: abstract, structural, and auxiliary.
type ObjectClassKind int

const (
	// ObjectClassAbstract represents an abstract object class.
	// Abstract classes cannot be instantiated directly and serve as
	// templates for other object classes.
	ObjectClassAbstract ObjectClassKind = iota

	// ObjectClassStructural represents a structural object class.
	// Every entry must have exactly one structural object class.
	// Structural classes define the core identity of an entry.
	ObjectClassStructural

	// ObjectClassAuxiliary represents an auxiliary object class.
	// Auxiliary classes provide additional attributes to entries
	// and can be combined with structural classes.
	ObjectClassAuxiliary
)

// String returns the string representation of the ObjectClassKind.
func (k ObjectClassKind) String() string {
	switch k {
	case ObjectClassAbstract:
		return "ABSTRACT"
	case ObjectClassStructural:
		return "STRUCTURAL"
	case ObjectClassAuxiliary:
		return "AUXILIARY"
	default:
		return "UNKNOWN"
	}
}

// ObjectClass represents an LDAP object class definition.
// Object classes define the set of attributes that entries of that class
// must have (MUST) and may have (MAY).
type ObjectClass struct {
	OID      string          // Object Identifier (e.g., "2.5.6.6")
	Name     string          // Primary name (e.g., "person")
	Names    []string        // All names including aliases
	Desc     string          // Human-readable description
	Obsolete bool            // Whether this object class is obsolete
	Superior string          // Parent object class name or OID
	Kind     ObjectClassKind // Abstract, Structural, or Auxiliary
	Must     []string        // Required attribute names
	May      []string        // Optional attribute names
}

// NewObjectClass creates a new ObjectClass with the given OID and name.
// The default kind is ObjectClassStructural.
func NewObjectClass(oid, name string) *ObjectClass {
	return &ObjectClass{
		OID:   oid,
		Name:  name,
		Names: []string{name},
		Kind:  ObjectClassStructural,
		Must:  []string{},
		May:   []string{},
	}
}

// IsAbstract returns true if this is an abstract object class.
func (oc *ObjectClass) IsAbstract() bool {
	return oc.Kind == ObjectClassAbstract
}

// IsStructural returns true if this is a structural object class.
func (oc *ObjectClass) IsStructural() bool {
	return oc.Kind == ObjectClassStructural
}

// IsAuxiliary returns true if this is an auxiliary object class.
func (oc *ObjectClass) IsAuxiliary() bool {
	return oc.Kind == ObjectClassAuxiliary
}

// HasMustAttribute checks if the given attribute is required by this object class.
func (oc *ObjectClass) HasMustAttribute(attr string) bool {
	for _, must := range oc.Must {
		if must == attr {
			return true
		}
	}
	return false
}

// HasMayAttribute checks if the given attribute is optional for this object class.
func (oc *ObjectClass) HasMayAttribute(attr string) bool {
	for _, may := range oc.May {
		if may == attr {
			return true
		}
	}
	return false
}

// AllowsAttribute checks if the given attribute is allowed (either MUST or MAY)
// by this object class.
func (oc *ObjectClass) AllowsAttribute(attr string) bool {
	return oc.HasMustAttribute(attr) || oc.HasMayAttribute(attr)
}

// AddMustAttribute adds a required attribute to this object class.
func (oc *ObjectClass) AddMustAttribute(attr string) {
	if !oc.HasMustAttribute(attr) {
		oc.Must = append(oc.Must, attr)
	}
}

// AddMayAttribute adds an optional attribute to this object class.
func (oc *ObjectClass) AddMayAttribute(attr string) {
	if !oc.HasMayAttribute(attr) {
		oc.May = append(oc.May, attr)
	}
}

// AddName adds an alias name to this object class.
func (oc *ObjectClass) AddName(name string) {
	for _, n := range oc.Names {
		if n == name {
			return
		}
	}
	oc.Names = append(oc.Names, name)
}
