package schema

import "testing"

func TestNewSyntax(t *testing.T) {
	syn := NewSyntax(SyntaxDirectoryString, "Directory String")

	if syn.OID != SyntaxDirectoryString {
		t.Errorf("expected OID '%s', got '%s'", SyntaxDirectoryString, syn.OID)
	}
	if syn.Description != "Directory String" {
		t.Errorf("expected Description 'Directory String', got '%s'", syn.Description)
	}
	if syn.Validator != nil {
		t.Error("Validator should be nil by default")
	}
}

func TestNewSyntaxWithValidator(t *testing.T) {
	validator := func(value []byte) bool { return true }
	syn := NewSyntaxWithValidator(SyntaxDirectoryString, "Directory String", validator)

	if syn.OID != SyntaxDirectoryString {
		t.Errorf("expected OID '%s', got '%s'", SyntaxDirectoryString, syn.OID)
	}
	if syn.Validator == nil {
		t.Error("Validator should not be nil")
	}
}

func TestSyntaxValidate(t *testing.T) {
	// Without validator, always returns true
	syn := NewSyntax(SyntaxDirectoryString, "Directory String")
	if !syn.Validate([]byte("test")) {
		t.Error("should return true when no validator is set")
	}

	// With validator
	syn.SetValidator(func(value []byte) bool {
		return len(value) > 0
	})
	if !syn.Validate([]byte("test")) {
		t.Error("should return true for non-empty value")
	}
	if syn.Validate([]byte("")) {
		t.Error("should return false for empty value")
	}
}

func TestSyntaxHasValidator(t *testing.T) {
	syn := NewSyntax(SyntaxDirectoryString, "Directory String")

	if syn.HasValidator() {
		t.Error("should not have validator initially")
	}

	syn.SetValidator(func(value []byte) bool { return true })
	if !syn.HasValidator() {
		t.Error("should have validator after setting")
	}
}

func TestValidateDirectoryString(t *testing.T) {
	tests := []struct {
		value    []byte
		expected bool
	}{
		{[]byte("hello"), true},
		{[]byte("Hello World"), true},
		{[]byte("日本語"), true},
		{[]byte(""), false},
		{[]byte{0xFF, 0xFE}, false}, // Invalid UTF-8
	}

	for _, tt := range tests {
		result := ValidateDirectoryString(tt.value)
		if result != tt.expected {
			t.Errorf("ValidateDirectoryString(%v) = %v, want %v", tt.value, result, tt.expected)
		}
	}
}

func TestValidateInteger(t *testing.T) {
	tests := []struct {
		value    []byte
		expected bool
	}{
		{[]byte("123"), true},
		{[]byte("-123"), true},
		{[]byte("+123"), true},
		{[]byte("0"), true},
		{[]byte(""), false},
		{[]byte("-"), false},
		{[]byte("+"), false},
		{[]byte("12.3"), false},
		{[]byte("abc"), false},
	}

	for _, tt := range tests {
		result := ValidateInteger(tt.value)
		if result != tt.expected {
			t.Errorf("ValidateInteger(%s) = %v, want %v", tt.value, result, tt.expected)
		}
	}
}

func TestValidateBoolean(t *testing.T) {
	tests := []struct {
		value    []byte
		expected bool
	}{
		{[]byte("TRUE"), true},
		{[]byte("FALSE"), true},
		{[]byte("true"), false},
		{[]byte("false"), false},
		{[]byte("1"), false},
		{[]byte("0"), false},
		{[]byte(""), false},
	}

	for _, tt := range tests {
		result := ValidateBoolean(tt.value)
		if result != tt.expected {
			t.Errorf("ValidateBoolean(%s) = %v, want %v", tt.value, result, tt.expected)
		}
	}
}

func TestValidateOctetString(t *testing.T) {
	// Octet string accepts any byte sequence
	tests := [][]byte{
		{},
		{0x00},
		{0xFF, 0xFE, 0x00},
		[]byte("hello"),
	}

	for _, value := range tests {
		if !ValidateOctetString(value) {
			t.Errorf("ValidateOctetString(%v) should return true", value)
		}
	}
}

func TestValidateIA5String(t *testing.T) {
	tests := []struct {
		value    []byte
		expected bool
	}{
		{[]byte("hello"), true},
		{[]byte("Hello World!"), true},
		{[]byte("test@example.com"), true},
		{[]byte(""), true},
		{[]byte{0x80}, false}, // Non-ASCII
		{[]byte("日本語"), false},
	}

	for _, tt := range tests {
		result := ValidateIA5String(tt.value)
		if result != tt.expected {
			t.Errorf("ValidateIA5String(%v) = %v, want %v", tt.value, result, tt.expected)
		}
	}
}

func TestValidatePrintableString(t *testing.T) {
	tests := []struct {
		value    []byte
		expected bool
	}{
		{[]byte("Hello World"), true},
		{[]byte("Test123"), true},
		{[]byte("a-b.c/d:e=f?"), true},
		{[]byte("'test'"), true},
		{[]byte("(test)"), true},
		{[]byte("a+b,c"), true},
		{[]byte(""), true},
		{[]byte("test@example"), false}, // @ not allowed
		{[]byte("test#123"), false},     // # not allowed
		{[]byte("日本語"), false},
	}

	for _, tt := range tests {
		result := ValidatePrintableString(tt.value)
		if result != tt.expected {
			t.Errorf("ValidatePrintableString(%s) = %v, want %v", tt.value, result, tt.expected)
		}
	}
}

func TestValidateNumericString(t *testing.T) {
	tests := []struct {
		value    []byte
		expected bool
	}{
		{[]byte("123"), true},
		{[]byte("1 2 3"), true},
		{[]byte(""), true},
		{[]byte("   "), true},
		{[]byte("12a3"), false},
		{[]byte("-123"), false},
	}

	for _, tt := range tests {
		result := ValidateNumericString(tt.value)
		if result != tt.expected {
			t.Errorf("ValidateNumericString(%s) = %v, want %v", tt.value, result, tt.expected)
		}
	}
}

func TestValidateTelephoneNumber(t *testing.T) {
	tests := []struct {
		value    []byte
		expected bool
	}{
		{[]byte("+1-555-123-4567"), true},
		{[]byte("(555) 123-4567"), true},
		{[]byte("555.123.4567"), true},
		{[]byte("5551234567"), true},
		{[]byte(""), false},
		{[]byte("555-CALL"), false},
		{[]byte("phone: 555"), false},
	}

	for _, tt := range tests {
		result := ValidateTelephoneNumber(tt.value)
		if result != tt.expected {
			t.Errorf("ValidateTelephoneNumber(%s) = %v, want %v", tt.value, result, tt.expected)
		}
	}
}

func TestSyntaxConstants(t *testing.T) {
	// Verify syntax OID constants are defined correctly
	expectedOIDs := map[string]string{
		"SyntaxDirectoryString":   "1.3.6.1.4.1.1466.115.121.1.15",
		"SyntaxDN":                "1.3.6.1.4.1.1466.115.121.1.12",
		"SyntaxInteger":           "1.3.6.1.4.1.1466.115.121.1.27",
		"SyntaxBoolean":           "1.3.6.1.4.1.1466.115.121.1.7",
		"SyntaxOctetString":       "1.3.6.1.4.1.1466.115.121.1.40",
		"SyntaxGeneralizedTime":   "1.3.6.1.4.1.1466.115.121.1.24",
		"SyntaxOID":               "1.3.6.1.4.1.1466.115.121.1.38",
		"SyntaxTelephoneNumber":   "1.3.6.1.4.1.1466.115.121.1.50",
		"SyntaxIA5String":         "1.3.6.1.4.1.1466.115.121.1.26",
		"SyntaxPrintableString":   "1.3.6.1.4.1.1466.115.121.1.44",
		"SyntaxNumericString":     "1.3.6.1.4.1.1466.115.121.1.36",
		"SyntaxBitString":         "1.3.6.1.4.1.1466.115.121.1.6",
		"SyntaxUUID":              "1.3.6.1.1.16.1",
	}

	actualOIDs := map[string]string{
		"SyntaxDirectoryString":   SyntaxDirectoryString,
		"SyntaxDN":                SyntaxDN,
		"SyntaxInteger":           SyntaxInteger,
		"SyntaxBoolean":           SyntaxBoolean,
		"SyntaxOctetString":       SyntaxOctetString,
		"SyntaxGeneralizedTime":   SyntaxGeneralizedTime,
		"SyntaxOID":               SyntaxOID,
		"SyntaxTelephoneNumber":   SyntaxTelephoneNumber,
		"SyntaxIA5String":         SyntaxIA5String,
		"SyntaxPrintableString":   SyntaxPrintableString,
		"SyntaxNumericString":     SyntaxNumericString,
		"SyntaxBitString":         SyntaxBitString,
		"SyntaxUUID":              SyntaxUUID,
	}

	for name, expected := range expectedOIDs {
		actual := actualOIDs[name]
		if actual != expected {
			t.Errorf("%s = %s, want %s", name, actual, expected)
		}
	}
}
