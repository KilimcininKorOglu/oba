package ldap

import (
	"bytes"
	"testing"

	"github.com/oba-ldap/oba/internal/ber"
)

func TestResultCode_String(t *testing.T) {
	tests := []struct {
		code ResultCode
		want string
	}{
		{ResultSuccess, "success"},
		{ResultOperationsError, "operationsError"},
		{ResultProtocolError, "protocolError"},
		{ResultTimeLimitExceeded, "timeLimitExceeded"},
		{ResultSizeLimitExceeded, "sizeLimitExceeded"},
		{ResultCompareFalse, "compareFalse"},
		{ResultCompareTrue, "compareTrue"},
		{ResultAuthMethodNotSupported, "authMethodNotSupported"},
		{ResultStrongerAuthRequired, "strongerAuthRequired"},
		{ResultReferral, "referral"},
		{ResultAdminLimitExceeded, "adminLimitExceeded"},
		{ResultUnavailableCriticalExtension, "unavailableCriticalExtension"},
		{ResultConfidentialityRequired, "confidentialityRequired"},
		{ResultSASLBindInProgress, "saslBindInProgress"},
		{ResultNoSuchAttribute, "noSuchAttribute"},
		{ResultUndefinedAttributeType, "undefinedAttributeType"},
		{ResultInappropriateMatching, "inappropriateMatching"},
		{ResultConstraintViolation, "constraintViolation"},
		{ResultAttributeOrValueExists, "attributeOrValueExists"},
		{ResultInvalidAttributeSyntax, "invalidAttributeSyntax"},
		{ResultNoSuchObject, "noSuchObject"},
		{ResultAliasProblem, "aliasProblem"},
		{ResultInvalidDNSyntax, "invalidDNSyntax"},
		{ResultAliasDereferencingProblem, "aliasDereferencingProblem"},
		{ResultInappropriateAuthentication, "inappropriateAuthentication"},
		{ResultInvalidCredentials, "invalidCredentials"},
		{ResultInsufficientAccessRights, "insufficientAccessRights"},
		{ResultBusy, "busy"},
		{ResultUnavailable, "unavailable"},
		{ResultUnwillingToPerform, "unwillingToPerform"},
		{ResultLoopDetect, "loopDetect"},
		{ResultNamingViolation, "namingViolation"},
		{ResultObjectClassViolation, "objectClassViolation"},
		{ResultNotAllowedOnNonLeaf, "notAllowedOnNonLeaf"},
		{ResultNotAllowedOnRDN, "notAllowedOnRDN"},
		{ResultEntryAlreadyExists, "entryAlreadyExists"},
		{ResultObjectClassModsProhibited, "objectClassModsProhibited"},
		{ResultAffectsMultipleDSAs, "affectsMultipleDSAs"},
		{ResultOther, "other"},
		{ResultCode(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.code.String(); got != tt.want {
				t.Errorf("ResultCode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResultCode_IsSuccess(t *testing.T) {
	tests := []struct {
		code ResultCode
		want bool
	}{
		{ResultSuccess, true},
		{ResultOperationsError, false},
		{ResultInvalidCredentials, false},
		{ResultNoSuchObject, false},
	}

	for _, tt := range tests {
		t.Run(tt.code.String(), func(t *testing.T) {
			if got := tt.code.IsSuccess(); got != tt.want {
				t.Errorf("ResultCode.IsSuccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResultCode_IsError(t *testing.T) {
	tests := []struct {
		code ResultCode
		want bool
	}{
		{ResultSuccess, false},
		{ResultCompareFalse, false},
		{ResultCompareTrue, false},
		{ResultReferral, false},
		{ResultSASLBindInProgress, false},
		{ResultOperationsError, true},
		{ResultInvalidCredentials, true},
		{ResultNoSuchObject, true},
	}

	for _, tt := range tests {
		t.Run(tt.code.String(), func(t *testing.T) {
			if got := tt.code.IsError(); got != tt.want {
				t.Errorf("ResultCode.IsError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBindResponse_Encode(t *testing.T) {
	tests := []struct {
		name     string
		response BindResponse
	}{
		{
			name: "success without SASL",
			response: BindResponse{
				LDAPResult: LDAPResult{
					ResultCode:        ResultSuccess,
					MatchedDN:         "",
					DiagnosticMessage: "",
				},
			},
		},
		{
			name: "success with SASL credentials",
			response: BindResponse{
				LDAPResult: LDAPResult{
					ResultCode:        ResultSuccess,
					MatchedDN:         "",
					DiagnosticMessage: "",
				},
				ServerSASLCreds: []byte{0x01, 0x02, 0x03, 0x04},
			},
		},
		{
			name: "SASL bind in progress",
			response: BindResponse{
				LDAPResult: LDAPResult{
					ResultCode:        ResultSASLBindInProgress,
					MatchedDN:         "",
					DiagnosticMessage: "SASL challenge",
				},
				ServerSASLCreds: []byte("challenge-data"),
			},
		},
		{
			name: "invalid credentials",
			response: BindResponse{
				LDAPResult: LDAPResult{
					ResultCode:        ResultInvalidCredentials,
					MatchedDN:         "",
					DiagnosticMessage: "Invalid username or password",
				},
			},
		},
		{
			name: "with referral",
			response: BindResponse{
				LDAPResult: LDAPResult{
					ResultCode:        ResultReferral,
					MatchedDN:         "dc=example,dc=com",
					DiagnosticMessage: "Referral to another server",
					Referral:          []string{"ldap://server1.example.com", "ldap://server2.example.com"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := tt.response.Encode()
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}

			// Verify the encoded data starts with APPLICATION 1 tag
			if len(encoded) == 0 {
				t.Fatal("Encode() returned empty data")
			}

			// APPLICATION 1 = 0x61 (01 1 00001)
			if encoded[0] != 0x61 {
				t.Errorf("First byte = 0x%02x, want 0x61 (APPLICATION 1)", encoded[0])
			}

			// Verify it can be decoded
			decoder := ber.NewBERDecoder(encoded)
			class, _, tagNum, err := decoder.ReadTag()
			if err != nil {
				t.Fatalf("ReadTag() error = %v", err)
			}
			if class != ber.ClassApplication || tagNum != ApplicationBindResponse {
				t.Errorf("Tag = class %d, num %d, want APPLICATION %d", class, tagNum, ApplicationBindResponse)
			}
		})
	}
}

func TestSearchResultEntry_Encode(t *testing.T) {
	tests := []struct {
		name  string
		entry SearchResultEntry
	}{
		{
			name: "simple entry",
			entry: SearchResultEntry{
				ObjectName: "cn=user,dc=example,dc=com",
				Attributes: []PartialAttribute{
					{
						Type:   "cn",
						Values: [][]byte{[]byte("user")},
					},
					{
						Type:   "objectClass",
						Values: [][]byte{[]byte("top"), []byte("person")},
					},
				},
			},
		},
		{
			name: "entry with no attributes",
			entry: SearchResultEntry{
				ObjectName: "dc=example,dc=com",
				Attributes: []PartialAttribute{},
			},
		},
		{
			name: "entry with empty attribute values",
			entry: SearchResultEntry{
				ObjectName: "cn=test,dc=example,dc=com",
				Attributes: []PartialAttribute{
					{
						Type:   "description",
						Values: [][]byte{},
					},
				},
			},
		},
		{
			name: "entry with binary values",
			entry: SearchResultEntry{
				ObjectName: "cn=cert,dc=example,dc=com",
				Attributes: []PartialAttribute{
					{
						Type:   "userCertificate",
						Values: [][]byte{{0x30, 0x82, 0x01, 0x22}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := tt.entry.Encode()
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}

			// Verify the encoded data starts with APPLICATION 4 tag
			if len(encoded) == 0 {
				t.Fatal("Encode() returned empty data")
			}

			// APPLICATION 4 = 0x64 (01 1 00100)
			if encoded[0] != 0x64 {
				t.Errorf("First byte = 0x%02x, want 0x64 (APPLICATION 4)", encoded[0])
			}

			// Verify it can be decoded
			decoder := ber.NewBERDecoder(encoded)
			class, _, tagNum, err := decoder.ReadTag()
			if err != nil {
				t.Fatalf("ReadTag() error = %v", err)
			}
			if class != ber.ClassApplication || tagNum != ApplicationSearchResultEntry {
				t.Errorf("Tag = class %d, num %d, want APPLICATION %d", class, tagNum, ApplicationSearchResultEntry)
			}
		})
	}
}

func TestSearchResultDone_Encode(t *testing.T) {
	tests := []struct {
		name     string
		response SearchResultDone
	}{
		{
			name: "success",
			response: SearchResultDone{
				LDAPResult: LDAPResult{
					ResultCode:        ResultSuccess,
					MatchedDN:         "",
					DiagnosticMessage: "",
				},
			},
		},
		{
			name: "size limit exceeded",
			response: SearchResultDone{
				LDAPResult: LDAPResult{
					ResultCode:        ResultSizeLimitExceeded,
					MatchedDN:         "",
					DiagnosticMessage: "Size limit of 1000 exceeded",
				},
			},
		},
		{
			name: "no such object",
			response: SearchResultDone{
				LDAPResult: LDAPResult{
					ResultCode:        ResultNoSuchObject,
					MatchedDN:         "dc=example,dc=com",
					DiagnosticMessage: "Base DN not found",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := tt.response.Encode()
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}

			// APPLICATION 5 = 0x65 (01 1 00101)
			if encoded[0] != 0x65 {
				t.Errorf("First byte = 0x%02x, want 0x65 (APPLICATION 5)", encoded[0])
			}
		})
	}
}

func TestModifyResponse_Encode(t *testing.T) {
	response := ModifyResponse{
		LDAPResult: LDAPResult{
			ResultCode:        ResultSuccess,
			MatchedDN:         "",
			DiagnosticMessage: "",
		},
	}

	encoded, err := response.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	// APPLICATION 7 = 0x67 (01 1 00111)
	if encoded[0] != 0x67 {
		t.Errorf("First byte = 0x%02x, want 0x67 (APPLICATION 7)", encoded[0])
	}
}

func TestAddResponse_Encode(t *testing.T) {
	response := AddResponse{
		LDAPResult: LDAPResult{
			ResultCode:        ResultEntryAlreadyExists,
			MatchedDN:         "",
			DiagnosticMessage: "Entry cn=user,dc=example,dc=com already exists",
		},
	}

	encoded, err := response.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	// APPLICATION 9 = 0x69 (01 1 01001)
	if encoded[0] != 0x69 {
		t.Errorf("First byte = 0x%02x, want 0x69 (APPLICATION 9)", encoded[0])
	}
}

func TestDeleteResponse_Encode(t *testing.T) {
	response := DeleteResponse{
		LDAPResult: LDAPResult{
			ResultCode:        ResultNotAllowedOnNonLeaf,
			MatchedDN:         "",
			DiagnosticMessage: "Cannot delete non-leaf entry",
		},
	}

	encoded, err := response.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	// APPLICATION 11 = 0x6b (01 1 01011)
	if encoded[0] != 0x6b {
		t.Errorf("First byte = 0x%02x, want 0x6b (APPLICATION 11)", encoded[0])
	}
}

func TestModifyDNResponse_Encode(t *testing.T) {
	response := ModifyDNResponse{
		LDAPResult: LDAPResult{
			ResultCode:        ResultSuccess,
			MatchedDN:         "",
			DiagnosticMessage: "",
		},
	}

	encoded, err := response.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	// APPLICATION 13 = 0x6d (01 1 01101)
	if encoded[0] != 0x6d {
		t.Errorf("First byte = 0x%02x, want 0x6d (APPLICATION 13)", encoded[0])
	}
}

func TestCompareResponse_Encode(t *testing.T) {
	tests := []struct {
		name     string
		response CompareResponse
	}{
		{
			name: "compare true",
			response: CompareResponse{
				LDAPResult: LDAPResult{
					ResultCode:        ResultCompareTrue,
					MatchedDN:         "",
					DiagnosticMessage: "",
				},
			},
		},
		{
			name: "compare false",
			response: CompareResponse{
				LDAPResult: LDAPResult{
					ResultCode:        ResultCompareFalse,
					MatchedDN:         "",
					DiagnosticMessage: "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := tt.response.Encode()
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}

			// APPLICATION 15 = 0x6f (01 1 01111)
			if encoded[0] != 0x6f {
				t.Errorf("First byte = 0x%02x, want 0x6f (APPLICATION 15)", encoded[0])
			}
		})
	}
}

func TestNewSuccessResult(t *testing.T) {
	result := NewSuccessResult()

	if result.ResultCode != ResultSuccess {
		t.Errorf("ResultCode = %v, want %v", result.ResultCode, ResultSuccess)
	}
	if result.MatchedDN != "" {
		t.Errorf("MatchedDN = %q, want empty", result.MatchedDN)
	}
	if result.DiagnosticMessage != "" {
		t.Errorf("DiagnosticMessage = %q, want empty", result.DiagnosticMessage)
	}
}

func TestNewErrorResult(t *testing.T) {
	result := NewErrorResult(ResultInvalidCredentials, "Bad password")

	if result.ResultCode != ResultInvalidCredentials {
		t.Errorf("ResultCode = %v, want %v", result.ResultCode, ResultInvalidCredentials)
	}
	if result.DiagnosticMessage != "Bad password" {
		t.Errorf("DiagnosticMessage = %q, want %q", result.DiagnosticMessage, "Bad password")
	}
}

func TestNewErrorResultWithDN(t *testing.T) {
	result := NewErrorResultWithDN(ResultNoSuchObject, "dc=example,dc=com", "Object not found")

	if result.ResultCode != ResultNoSuchObject {
		t.Errorf("ResultCode = %v, want %v", result.ResultCode, ResultNoSuchObject)
	}
	if result.MatchedDN != "dc=example,dc=com" {
		t.Errorf("MatchedDN = %q, want %q", result.MatchedDN, "dc=example,dc=com")
	}
	if result.DiagnosticMessage != "Object not found" {
		t.Errorf("DiagnosticMessage = %q, want %q", result.DiagnosticMessage, "Object not found")
	}
}

func TestLDAPResult_EncodeWithReferral(t *testing.T) {
	result := LDAPResult{
		ResultCode:        ResultReferral,
		MatchedDN:         "",
		DiagnosticMessage: "Referral",
		Referral:          []string{"ldap://server1.example.com/dc=example,dc=com", "ldap://server2.example.com/dc=example,dc=com"},
	}

	encoder := ber.NewBEREncoder(128)
	seqPos := encoder.BeginSequence()
	if err := result.Encode(encoder); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if err := encoder.EndSequence(seqPos); err != nil {
		t.Fatalf("EndSequence() error = %v", err)
	}

	encoded := encoder.Bytes()
	if len(encoded) == 0 {
		t.Fatal("Encode() returned empty data")
	}

	// Verify the referral tag [3] is present
	found := false
	for i := 0; i < len(encoded)-1; i++ {
		// Context-specific tag [3] constructed = 0xa3
		if encoded[i] == 0xa3 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Referral tag [3] not found in encoded data")
	}
}

func TestBindResponse_EncodeWithSASLCreds(t *testing.T) {
	response := BindResponse{
		LDAPResult: LDAPResult{
			ResultCode:        ResultSASLBindInProgress,
			MatchedDN:         "",
			DiagnosticMessage: "",
		},
		ServerSASLCreds: []byte("server-challenge"),
	}

	encoded, err := response.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	// Verify the serverSaslCreds tag [7] is present
	found := false
	for i := 0; i < len(encoded)-1; i++ {
		// Context-specific tag [7] primitive = 0x87
		if encoded[i] == 0x87 {
			found = true
			break
		}
	}
	if !found {
		t.Error("ServerSASLCreds tag [7] not found in encoded data")
	}
}

func TestSearchResultEntry_EncodeRoundTrip(t *testing.T) {
	entry := SearchResultEntry{
		ObjectName: "cn=John Doe,ou=People,dc=example,dc=com",
		Attributes: []PartialAttribute{
			{
				Type:   "cn",
				Values: [][]byte{[]byte("John Doe")},
			},
			{
				Type:   "sn",
				Values: [][]byte{[]byte("Doe")},
			},
			{
				Type:   "mail",
				Values: [][]byte{[]byte("john.doe@example.com"), []byte("jdoe@example.com")},
			},
			{
				Type:   "objectClass",
				Values: [][]byte{[]byte("top"), []byte("person"), []byte("organizationalPerson"), []byte("inetOrgPerson")},
			},
		},
	}

	encoded, err := entry.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	// Verify the structure by decoding
	decoder := ber.NewBERDecoder(encoded)

	// Read APPLICATION 4 tag
	class, constructed, tagNum, err := decoder.ReadTag()
	if err != nil {
		t.Fatalf("ReadTag() error = %v", err)
	}
	if class != ber.ClassApplication || tagNum != ApplicationSearchResultEntry {
		t.Errorf("Tag = class %d, num %d, want APPLICATION %d", class, tagNum, ApplicationSearchResultEntry)
	}
	if constructed != ber.TypeConstructed {
		t.Error("Expected constructed tag")
	}

	// Read length
	_, err = decoder.ReadLength()
	if err != nil {
		t.Fatalf("ReadLength() error = %v", err)
	}

	// Read objectName
	objectName, err := decoder.ReadOctetString()
	if err != nil {
		t.Fatalf("ReadOctetString() error = %v", err)
	}
	if string(objectName) != entry.ObjectName {
		t.Errorf("ObjectName = %q, want %q", string(objectName), entry.ObjectName)
	}
}

func TestAllResponseTypes_CanBeWrappedInLDAPMessage(t *testing.T) {
	tests := []struct {
		name    string
		encode  func() ([]byte, error)
		appTag  int
	}{
		{
			name: "BindResponse",
			encode: func() ([]byte, error) {
				return (&BindResponse{LDAPResult: NewSuccessResult()}).Encode()
			},
			appTag: ApplicationBindResponse,
		},
		{
			name: "SearchResultEntry",
			encode: func() ([]byte, error) {
				return (&SearchResultEntry{ObjectName: "dc=test", Attributes: nil}).Encode()
			},
			appTag: ApplicationSearchResultEntry,
		},
		{
			name: "SearchResultDone",
			encode: func() ([]byte, error) {
				return (&SearchResultDone{LDAPResult: NewSuccessResult()}).Encode()
			},
			appTag: ApplicationSearchResultDone,
		},
		{
			name: "ModifyResponse",
			encode: func() ([]byte, error) {
				return (&ModifyResponse{LDAPResult: NewSuccessResult()}).Encode()
			},
			appTag: ApplicationModifyResponse,
		},
		{
			name: "AddResponse",
			encode: func() ([]byte, error) {
				return (&AddResponse{LDAPResult: NewSuccessResult()}).Encode()
			},
			appTag: ApplicationAddResponse,
		},
		{
			name: "DeleteResponse",
			encode: func() ([]byte, error) {
				return (&DeleteResponse{LDAPResult: NewSuccessResult()}).Encode()
			},
			appTag: ApplicationDelResponse,
		},
		{
			name: "ModifyDNResponse",
			encode: func() ([]byte, error) {
				return (&ModifyDNResponse{LDAPResult: NewSuccessResult()}).Encode()
			},
			appTag: ApplicationModifyDNResponse,
		},
		{
			name: "CompareResponse",
			encode: func() ([]byte, error) {
				return (&CompareResponse{LDAPResult: LDAPResult{ResultCode: ResultCompareTrue}}).Encode()
			},
			appTag: ApplicationCompareResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := tt.encode()
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}

			// Wrap in LDAPMessage
			msg := &LDAPMessage{
				MessageID: 1,
				Operation: &RawOperation{
					Tag:  tt.appTag,
					Data: extractOperationData(encoded),
				},
			}

			msgEncoded, err := msg.Encode()
			if err != nil {
				t.Fatalf("LDAPMessage.Encode() error = %v", err)
			}

			// Parse it back
			parsed, err := ParseLDAPMessage(msgEncoded)
			if err != nil {
				t.Fatalf("ParseLDAPMessage() error = %v", err)
			}

			if parsed.Operation.Tag != tt.appTag {
				t.Errorf("Operation.Tag = %d, want %d", parsed.Operation.Tag, tt.appTag)
			}
		})
	}
}

// extractOperationData extracts the content from an APPLICATION-tagged response
func extractOperationData(encoded []byte) []byte {
	if len(encoded) < 2 {
		return nil
	}

	decoder := ber.NewBERDecoder(encoded)
	_, _, _, err := decoder.ReadTag()
	if err != nil {
		return nil
	}

	length, err := decoder.ReadLength()
	if err != nil {
		return nil
	}

	offset := decoder.Offset()
	if offset+length > len(encoded) {
		return nil
	}

	return encoded[offset : offset+length]
}

func TestResultCodeValues(t *testing.T) {
	// Verify result code values match RFC 4511
	tests := []struct {
		code  ResultCode
		value int
	}{
		{ResultSuccess, 0},
		{ResultOperationsError, 1},
		{ResultProtocolError, 2},
		{ResultTimeLimitExceeded, 3},
		{ResultSizeLimitExceeded, 4},
		{ResultCompareFalse, 5},
		{ResultCompareTrue, 6},
		{ResultAuthMethodNotSupported, 7},
		{ResultStrongerAuthRequired, 8},
		{ResultReferral, 10},
		{ResultAdminLimitExceeded, 11},
		{ResultUnavailableCriticalExtension, 12},
		{ResultConfidentialityRequired, 13},
		{ResultSASLBindInProgress, 14},
		{ResultNoSuchAttribute, 16},
		{ResultUndefinedAttributeType, 17},
		{ResultInappropriateMatching, 18},
		{ResultConstraintViolation, 19},
		{ResultAttributeOrValueExists, 20},
		{ResultInvalidAttributeSyntax, 21},
		{ResultNoSuchObject, 32},
		{ResultAliasProblem, 33},
		{ResultInvalidDNSyntax, 34},
		{ResultAliasDereferencingProblem, 36},
		{ResultInappropriateAuthentication, 48},
		{ResultInvalidCredentials, 49},
		{ResultInsufficientAccessRights, 50},
		{ResultBusy, 51},
		{ResultUnavailable, 52},
		{ResultUnwillingToPerform, 53},
		{ResultLoopDetect, 54},
		{ResultNamingViolation, 64},
		{ResultObjectClassViolation, 65},
		{ResultNotAllowedOnNonLeaf, 66},
		{ResultNotAllowedOnRDN, 67},
		{ResultEntryAlreadyExists, 68},
		{ResultObjectClassModsProhibited, 69},
		{ResultAffectsMultipleDSAs, 71},
		{ResultOther, 80},
	}

	for _, tt := range tests {
		t.Run(tt.code.String(), func(t *testing.T) {
			if int(tt.code) != tt.value {
				t.Errorf("ResultCode %s = %d, want %d", tt.code.String(), int(tt.code), tt.value)
			}
		})
	}
}

func TestSearchResultEntry_LargeEntry(t *testing.T) {
	// Test encoding a large entry with many attributes
	entry := SearchResultEntry{
		ObjectName: "cn=large,dc=example,dc=com",
		Attributes: make([]PartialAttribute, 100),
	}

	for i := 0; i < 100; i++ {
		entry.Attributes[i] = PartialAttribute{
			Type:   "attr" + string(rune('A'+i%26)),
			Values: [][]byte{bytes.Repeat([]byte("value"), 10)},
		}
	}

	encoded, err := entry.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	if len(encoded) == 0 {
		t.Fatal("Encode() returned empty data")
	}

	// Verify it starts with APPLICATION 4
	if encoded[0] != 0x64 {
		t.Errorf("First byte = 0x%02x, want 0x64", encoded[0])
	}
}
