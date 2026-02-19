package schema

// Syntax represents an LDAP syntax definition.
// Syntaxes define the format and validation rules for attribute values.
type Syntax struct {
	OID         string            // Object Identifier (e.g., "1.3.6.1.4.1.1466.115.121.1.15")
	Description string            // Human-readable description (e.g., "Directory String")
	Validator   func([]byte) bool // Function to validate values against this syntax
}

// NewSyntax creates a new Syntax with the given OID and description.
// The validator is initially nil and should be set separately.
func NewSyntax(oid, description string) *Syntax {
	return &Syntax{
		OID:         oid,
		Description: description,
	}
}

// NewSyntaxWithValidator creates a new Syntax with the given OID, description, and validator.
func NewSyntaxWithValidator(oid, description string, validator func([]byte) bool) *Syntax {
	return &Syntax{
		OID:         oid,
		Description: description,
		Validator:   validator,
	}
}

// Validate checks if the given value conforms to this syntax.
// Returns true if the value is valid or if no validator is defined.
func (s *Syntax) Validate(value []byte) bool {
	if s.Validator == nil {
		return true
	}
	return s.Validator(value)
}

// HasValidator returns true if this syntax has a validator function defined.
func (s *Syntax) HasValidator() bool {
	return s.Validator != nil
}

// SetValidator sets the validator function for this syntax.
func (s *Syntax) SetValidator(validator func([]byte) bool) {
	s.Validator = validator
}

// Common LDAP Syntax OIDs as constants for convenience.
const (
	// SyntaxDirectoryString is the OID for Directory String syntax (UTF-8 string).
	SyntaxDirectoryString = "1.3.6.1.4.1.1466.115.121.1.15"

	// SyntaxDN is the OID for Distinguished Name syntax.
	SyntaxDN = "1.3.6.1.4.1.1466.115.121.1.12"

	// SyntaxInteger is the OID for Integer syntax.
	SyntaxInteger = "1.3.6.1.4.1.1466.115.121.1.27"

	// SyntaxBoolean is the OID for Boolean syntax.
	SyntaxBoolean = "1.3.6.1.4.1.1466.115.121.1.7"

	// SyntaxOctetString is the OID for Octet String syntax (binary data).
	SyntaxOctetString = "1.3.6.1.4.1.1466.115.121.1.40"

	// SyntaxGeneralizedTime is the OID for Generalized Time syntax.
	SyntaxGeneralizedTime = "1.3.6.1.4.1.1466.115.121.1.24"

	// SyntaxOID is the OID for OID syntax.
	SyntaxOID = "1.3.6.1.4.1.1466.115.121.1.38"

	// SyntaxTelephoneNumber is the OID for Telephone Number syntax.
	SyntaxTelephoneNumber = "1.3.6.1.4.1.1466.115.121.1.50"

	// SyntaxIA5String is the OID for IA5 String syntax (ASCII).
	SyntaxIA5String = "1.3.6.1.4.1.1466.115.121.1.26"

	// SyntaxPrintableString is the OID for Printable String syntax.
	SyntaxPrintableString = "1.3.6.1.4.1.1466.115.121.1.44"

	// SyntaxNumericString is the OID for Numeric String syntax.
	SyntaxNumericString = "1.3.6.1.4.1.1466.115.121.1.36"

	// SyntaxBitString is the OID for Bit String syntax.
	SyntaxBitString = "1.3.6.1.4.1.1466.115.121.1.6"

	// SyntaxUUID is the OID for UUID syntax.
	SyntaxUUID = "1.3.6.1.1.16.1"
)

// Common syntax validators that can be used with Syntax.SetValidator.

// ValidateDirectoryString validates a Directory String (UTF-8 string).
// Returns true if the value is a valid UTF-8 string with at least one character.
func ValidateDirectoryString(value []byte) bool {
	if len(value) == 0 {
		return false
	}
	// Check for valid UTF-8
	for i := 0; i < len(value); {
		if value[i] < 0x80 {
			i++
			continue
		}
		// Multi-byte UTF-8 sequence
		var size int
		if value[i]&0xE0 == 0xC0 {
			size = 2
		} else if value[i]&0xF0 == 0xE0 {
			size = 3
		} else if value[i]&0xF8 == 0xF0 {
			size = 4
		} else {
			return false
		}
		if i+size > len(value) {
			return false
		}
		for j := 1; j < size; j++ {
			if value[i+j]&0xC0 != 0x80 {
				return false
			}
		}
		i += size
	}
	return true
}

// ValidateInteger validates an Integer value.
// Returns true if the value represents a valid integer string.
func ValidateInteger(value []byte) bool {
	if len(value) == 0 {
		return false
	}
	start := 0
	if value[0] == '-' || value[0] == '+' {
		start = 1
		if len(value) == 1 {
			return false
		}
	}
	for i := start; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

// ValidateBoolean validates a Boolean value.
// Returns true if the value is "TRUE" or "FALSE".
func ValidateBoolean(value []byte) bool {
	s := string(value)
	return s == "TRUE" || s == "FALSE"
}

// ValidateOctetString validates an Octet String (any binary data).
// Always returns true as any byte sequence is valid.
func ValidateOctetString(value []byte) bool {
	return true
}

// ValidateIA5String validates an IA5 String (ASCII).
// Returns true if all bytes are in the ASCII range (0-127).
func ValidateIA5String(value []byte) bool {
	for _, b := range value {
		if b > 127 {
			return false
		}
	}
	return true
}

// ValidatePrintableString validates a Printable String.
// Returns true if all characters are in the printable string character set.
func ValidatePrintableString(value []byte) bool {
	for _, b := range value {
		if !isPrintableChar(b) {
			return false
		}
	}
	return true
}

// isPrintableChar checks if a byte is a valid printable string character.
func isPrintableChar(b byte) bool {
	// A-Z, a-z, 0-9, space, and special characters: '()+,-./:=?
	if b >= 'A' && b <= 'Z' {
		return true
	}
	if b >= 'a' && b <= 'z' {
		return true
	}
	if b >= '0' && b <= '9' {
		return true
	}
	switch b {
	case ' ', '\'', '(', ')', '+', ',', '-', '.', '/', ':', '=', '?':
		return true
	}
	return false
}

// ValidateNumericString validates a Numeric String.
// Returns true if all characters are digits or spaces.
func ValidateNumericString(value []byte) bool {
	for _, b := range value {
		if b != ' ' && (b < '0' || b > '9') {
			return false
		}
	}
	return true
}

// ValidateTelephoneNumber validates a Telephone Number.
// Returns true if the value contains only valid telephone number characters.
func ValidateTelephoneNumber(value []byte) bool {
	if len(value) == 0 {
		return false
	}
	for _, b := range value {
		if !isTelephoneChar(b) {
			return false
		}
	}
	return true
}

// isTelephoneChar checks if a byte is a valid telephone number character.
func isTelephoneChar(b byte) bool {
	if b >= '0' && b <= '9' {
		return true
	}
	switch b {
	case ' ', '-', '(', ')', '+', '.':
		return true
	}
	return false
}
