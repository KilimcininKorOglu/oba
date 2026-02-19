// Package schema provides LDAP schema validation for the Oba LDAP server.
package schema

import (
	"fmt"
	"strings"
)

// Validation error codes
const (
	// ErrObjectClassViolation indicates an object class constraint violation.
	ErrObjectClassViolation = iota
	// ErrUndefinedAttributeType indicates an attribute type is not defined in the schema.
	ErrUndefinedAttributeType
	// ErrInvalidAttributeSyntax indicates an attribute value does not match its syntax.
	ErrInvalidAttributeSyntax
	// ErrMissingRequiredAttribute indicates a required (MUST) attribute is missing.
	ErrMissingRequiredAttribute
	// ErrSingleValueViolation indicates a single-value attribute has multiple values.
	ErrSingleValueViolation
	// ErrNoUserModification indicates an attempt to modify a read-only attribute.
	ErrNoUserModification
)

// ValidationError represents a schema validation error.
type ValidationError struct {
	Code    int
	Message string
	Attr    string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if e.Attr != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Attr)
	}
	return e.Message
}

// NewValidationError creates a new ValidationError with the given code and message.
func NewValidationError(code int, message string) *ValidationError {
	return &ValidationError{
		Code:    code,
		Message: message,
	}
}

// NewValidationErrorWithAttr creates a new ValidationError with the given code, message, and attribute.
func NewValidationErrorWithAttr(code int, message, attr string) *ValidationError {
	return &ValidationError{
		Code:    code,
		Message: message,
		Attr:    attr,
	}
}

// Entry represents an LDAP entry for validation.
// This is a simplified interface to avoid circular dependencies.
type Entry struct {
	DN         string
	Attributes map[string][][]byte
}

// NewEntry creates a new Entry with the given DN.
func NewEntry(dn string) *Entry {
	return &Entry{
		DN:         dn,
		Attributes: make(map[string][][]byte),
	}
}

// SetAttribute sets an attribute value on the entry.
func (e *Entry) SetAttribute(name string, values ...[]byte) {
	e.Attributes[name] = values
}

// SetStringAttribute sets a string attribute value on the entry.
func (e *Entry) SetStringAttribute(name string, values ...string) {
	byteValues := make([][]byte, len(values))
	for i, v := range values {
		byteValues[i] = []byte(v)
	}
	e.Attributes[name] = byteValues
}

// GetAttribute returns the values for an attribute.
func (e *Entry) GetAttribute(name string) [][]byte {
	return e.Attributes[name]
}

// GetAll returns all string values for an attribute.
func (e *Entry) GetAll(name string) []string {
	values := e.Attributes[name]
	if values == nil {
		return nil
	}
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = string(v)
	}
	return result
}

// Has checks if the entry has the given attribute.
func (e *Entry) Has(name string) bool {
	values, ok := e.Attributes[name]
	return ok && len(values) > 0
}

// Clone creates a deep copy of the entry.
func (e *Entry) Clone() *Entry {
	if e == nil {
		return nil
	}

	clone := &Entry{
		DN:         e.DN,
		Attributes: make(map[string][][]byte, len(e.Attributes)),
	}

	for k, v := range e.Attributes {
		values := make([][]byte, len(v))
		for i, val := range v {
			values[i] = make([]byte, len(val))
			copy(values[i], val)
		}
		clone.Attributes[k] = values
	}

	return clone
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

// Modification represents a single modification to an entry.
type Modification struct {
	Type   ModificationType
	Attr   string
	Values [][]byte
}

// NewModification creates a new Modification.
func NewModification(modType ModificationType, attr string, values ...[]byte) *Modification {
	return &Modification{
		Type:   modType,
		Attr:   attr,
		Values: values,
	}
}

// NewStringModification creates a new Modification with string values.
func NewStringModification(modType ModificationType, attr string, values ...string) *Modification {
	byteValues := make([][]byte, len(values))
	for i, v := range values {
		byteValues[i] = []byte(v)
	}
	return &Modification{
		Type:   modType,
		Attr:   attr,
		Values: byteValues,
	}
}

// Validator validates LDAP entries against a schema.
type Validator struct {
	schema *Schema
}

// NewValidator creates a new Validator with the given schema.
func NewValidator(schema *Schema) *Validator {
	return &Validator{
		schema: schema,
	}
}

// ValidateEntry validates an entry against the schema.
// It checks:
// 1. Entry must have objectClass attribute
// 2. At least one structural object class required
// 3. All required (MUST) attributes present
// 4. All attributes allowed by MAY or MUST
// 5. Single-value attributes have at most one value
// 6. Attribute values match syntax
func (v *Validator) ValidateEntry(entry *Entry) error {
	if entry == nil {
		return NewValidationError(ErrObjectClassViolation, "entry is nil")
	}

	// 1. Get all object classes
	classes := entry.GetAll("objectClass")
	if len(classes) == 0 {
		return NewValidationError(ErrObjectClassViolation, "objectClass required")
	}

	// Collect all MUST and MAY attributes from all object classes
	must := make(map[string]bool)
	may := make(map[string]bool)
	hasStructural := false

	for _, className := range classes {
		oc := v.schema.GetObjectClass(className)
		if oc == nil {
			return NewValidationErrorWithAttr(ErrObjectClassViolation, "unknown objectClass", className)
		}

		// 2. Check for at least one structural object class
		if oc.IsStructural() {
			hasStructural = true
		}

		// Collect MUST attributes (including inherited)
		for _, attr := range v.schema.GetAllMustAttributes(className) {
			must[strings.ToLower(attr)] = true
		}

		// Collect MAY attributes (including inherited)
		for _, attr := range v.schema.GetAllMayAttributes(className) {
			may[strings.ToLower(attr)] = true
		}
	}

	// 2. At least one structural object class required
	if !hasStructural {
		return NewValidationError(ErrObjectClassViolation, "at least one structural objectClass required")
	}

	// 3. Check required attributes
	for attr := range must {
		if !v.hasAttributeCaseInsensitive(entry, attr) {
			return NewValidationErrorWithAttr(ErrMissingRequiredAttribute, "missing required attribute", attr)
		}
	}

	// 4. Check all attributes are allowed
	for attr := range entry.Attributes {
		attrLower := strings.ToLower(attr)

		// Skip objectClass - it's always allowed
		if attrLower == "objectclass" {
			continue
		}

		// Check if attribute is allowed by MUST or MAY
		if !must[attrLower] && !may[attrLower] {
			// Check if it's an operational attribute
			if !v.isOperational(attr) {
				return NewValidationErrorWithAttr(ErrUndefinedAttributeType, "attribute not allowed by objectClass", attr)
			}
		}
	}

	// 5. Check single-value constraints
	for attr, values := range entry.Attributes {
		at := v.schema.GetAttributeType(attr)
		if at != nil && at.SingleValue && len(values) > 1 {
			return NewValidationErrorWithAttr(ErrSingleValueViolation, "single-value attribute has multiple values", attr)
		}
	}

	// 6. Validate attribute syntax
	for attr, values := range entry.Attributes {
		if err := v.validateAttributeSyntax(attr, values); err != nil {
			return err
		}
	}

	return nil
}

// ValidateModification validates a modification against the schema.
// It applies the modifications to a copy of the entry and validates the result.
func (v *Validator) ValidateModification(entry *Entry, mods []Modification) error {
	if entry == nil {
		return NewValidationError(ErrObjectClassViolation, "entry is nil")
	}

	// Create a copy of the entry to apply modifications
	modified := entry.Clone()

	// Apply modifications
	for _, mod := range mods {
		// Check if attribute is read-only (NO-USER-MODIFICATION)
		at := v.schema.GetAttributeType(mod.Attr)
		if at != nil && at.NoUserMod {
			return NewValidationErrorWithAttr(ErrNoUserModification, "attribute is read-only", mod.Attr)
		}

		switch mod.Type {
		case ModAdd:
			// Add values to existing attribute
			existing := modified.GetAttribute(mod.Attr)
			modified.SetAttribute(mod.Attr, append(existing, mod.Values...)...)

		case ModDelete:
			if len(mod.Values) == 0 {
				// Delete entire attribute
				delete(modified.Attributes, mod.Attr)
			} else {
				// Delete specific values
				existing := modified.GetAttribute(mod.Attr)
				newValues := make([][]byte, 0, len(existing))
				for _, ev := range existing {
					keep := true
					for _, dv := range mod.Values {
						if bytesEqual(ev, dv) {
							keep = false
							break
						}
					}
					if keep {
						newValues = append(newValues, ev)
					}
				}
				if len(newValues) == 0 {
					delete(modified.Attributes, mod.Attr)
				} else {
					modified.SetAttribute(mod.Attr, newValues...)
				}
			}

		case ModReplace:
			if len(mod.Values) == 0 {
				// Replace with empty = delete
				delete(modified.Attributes, mod.Attr)
			} else {
				modified.SetAttribute(mod.Attr, mod.Values...)
			}
		}

		// Validate single-value constraint after modification
		if at != nil && at.SingleValue {
			values := modified.GetAttribute(mod.Attr)
			if len(values) > 1 {
				return NewValidationErrorWithAttr(ErrSingleValueViolation, "single-value attribute has multiple values", mod.Attr)
			}
		}

		// Validate syntax for added/replaced values
		if mod.Type == ModAdd || mod.Type == ModReplace {
			if err := v.validateAttributeSyntax(mod.Attr, mod.Values); err != nil {
				return err
			}
		}
	}

	// Validate the modified entry
	return v.ValidateEntry(modified)
}

// isOperational checks if an attribute is an operational attribute.
func (v *Validator) isOperational(attr string) bool {
	at := v.schema.GetAttributeType(attr)
	if at == nil {
		return false
	}
	return at.IsOperational()
}

// hasAttributeCaseInsensitive checks if the entry has an attribute (case-insensitive).
func (v *Validator) hasAttributeCaseInsensitive(entry *Entry, attrLower string) bool {
	for attr := range entry.Attributes {
		if strings.ToLower(attr) == attrLower {
			values := entry.Attributes[attr]
			if len(values) > 0 {
				return true
			}
		}
	}
	return false
}

// validateAttributeSyntax validates attribute values against their syntax.
func (v *Validator) validateAttributeSyntax(attr string, values [][]byte) error {
	// Get the effective syntax for this attribute
	syntaxOID := v.schema.GetEffectiveSyntax(attr)
	if syntaxOID == "" {
		// No syntax defined, skip validation
		return nil
	}

	// Get the syntax definition
	syntax := v.schema.GetSyntax(syntaxOID)
	if syntax == nil || !syntax.HasValidator() {
		// No validator defined, skip validation
		return nil
	}

	// Validate each value
	for _, value := range values {
		if !syntax.Validate(value) {
			return NewValidationErrorWithAttr(ErrInvalidAttributeSyntax, "invalid attribute syntax", attr)
		}
	}

	return nil
}

// bytesEqual compares two byte slices for equality.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// GetSchema returns the validator's schema.
func (v *Validator) GetSchema() *Schema {
	return v.schema
}
