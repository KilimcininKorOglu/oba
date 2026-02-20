// Package server provides the LDAP server implementation.
package server

import (
	"testing"

	"github.com/oba-ldap/oba/internal/ldap"
)

func TestNewHandler(t *testing.T) {
	handler := NewHandler()
	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}
	if handler.bindHandler == nil {
		t.Error("bindHandler should not be nil")
	}
	if handler.searchHandler == nil {
		t.Error("searchHandler should not be nil")
	}
	if handler.addHandler == nil {
		t.Error("addHandler should not be nil")
	}
	if handler.deleteHandler == nil {
		t.Error("deleteHandler should not be nil")
	}
	if handler.modifyHandler == nil {
		t.Error("modifyHandler should not be nil")
	}
}

func TestHandlerSetBindHandler(t *testing.T) {
	handler := NewHandler()
	called := false

	handler.SetBindHandler(func(conn *Connection, req *ldap.BindRequest) *OperationResult {
		called = true
		return &OperationResult{ResultCode: ldap.ResultSuccess}
	})

	result := handler.HandleBind(nil, &ldap.BindRequest{})
	if !called {
		t.Error("Custom bind handler was not called")
	}
	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Expected ResultSuccess, got %v", result.ResultCode)
	}
}

func TestHandlerSetSearchHandler(t *testing.T) {
	handler := NewHandler()
	called := false

	handler.SetSearchHandler(func(conn *Connection, req *ldap.SearchRequest) *SearchResult {
		called = true
		return &SearchResult{
			OperationResult: OperationResult{ResultCode: ldap.ResultSuccess},
			Entries: []*SearchEntry{
				{DN: "cn=test,dc=example,dc=com"},
			},
		}
	})

	result := handler.HandleSearch(nil, &ldap.SearchRequest{})
	if !called {
		t.Error("Custom search handler was not called")
	}
	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Expected ResultSuccess, got %v", result.ResultCode)
	}
	if len(result.Entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(result.Entries))
	}
}

func TestHandlerSetAddHandler(t *testing.T) {
	handler := NewHandler()
	called := false

	handler.SetAddHandler(func(conn *Connection, req *ldap.AddRequest) *OperationResult {
		called = true
		return &OperationResult{ResultCode: ldap.ResultSuccess}
	})

	result := handler.HandleAdd(nil, &ldap.AddRequest{})
	if !called {
		t.Error("Custom add handler was not called")
	}
	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Expected ResultSuccess, got %v", result.ResultCode)
	}
}

func TestHandlerSetDeleteHandler(t *testing.T) {
	handler := NewHandler()
	called := false

	handler.SetDeleteHandler(func(conn *Connection, req *ldap.DeleteRequest) *OperationResult {
		called = true
		return &OperationResult{ResultCode: ldap.ResultSuccess}
	})

	result := handler.HandleDelete(nil, &ldap.DeleteRequest{})
	if !called {
		t.Error("Custom delete handler was not called")
	}
	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Expected ResultSuccess, got %v", result.ResultCode)
	}
}

func TestHandlerSetModifyHandler(t *testing.T) {
	handler := NewHandler()
	called := false

	handler.SetModifyHandler(func(conn *Connection, req *ldap.ModifyRequest) *OperationResult {
		called = true
		return &OperationResult{ResultCode: ldap.ResultSuccess}
	})

	result := handler.HandleModify(nil, &ldap.ModifyRequest{})
	if !called {
		t.Error("Custom modify handler was not called")
	}
	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Expected ResultSuccess, got %v", result.ResultCode)
	}
}

func TestDefaultBindHandlerAnonymous(t *testing.T) {
	handler := NewHandler()

	// Anonymous bind should succeed
	req := &ldap.BindRequest{
		Version:    3,
		Name:       "",
		AuthMethod: ldap.AuthMethodSimple,
	}

	result := handler.HandleBind(nil, req)
	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Anonymous bind should succeed, got %v", result.ResultCode)
	}
}

func TestDefaultBindHandlerAuthenticated(t *testing.T) {
	handler := NewHandler()

	// Authenticated bind should fail with default handler
	req := &ldap.BindRequest{
		Version:        3,
		Name:           "cn=admin,dc=example,dc=com",
		AuthMethod:     ldap.AuthMethodSimple,
		SimplePassword: []byte("secret"),
	}

	result := handler.HandleBind(nil, req)
	if result.ResultCode != ldap.ResultInvalidCredentials {
		t.Errorf("Authenticated bind should fail with InvalidCredentials, got %v", result.ResultCode)
	}
}

func TestDefaultSearchHandler(t *testing.T) {
	handler := NewHandler()

	result := handler.HandleSearch(nil, &ldap.SearchRequest{})
	if result.ResultCode != ldap.ResultUnwillingToPerform {
		t.Errorf("Default search should return UnwillingToPerform, got %v", result.ResultCode)
	}
}

func TestDefaultAddHandler(t *testing.T) {
	handler := NewHandler()

	result := handler.HandleAdd(nil, &ldap.AddRequest{})
	if result.ResultCode != ldap.ResultUnwillingToPerform {
		t.Errorf("Default add should return UnwillingToPerform, got %v", result.ResultCode)
	}
}

func TestDefaultDeleteHandler(t *testing.T) {
	handler := NewHandler()

	result := handler.HandleDelete(nil, &ldap.DeleteRequest{})
	if result.ResultCode != ldap.ResultUnwillingToPerform {
		t.Errorf("Default delete should return UnwillingToPerform, got %v", result.ResultCode)
	}
}

func TestDefaultModifyHandler(t *testing.T) {
	handler := NewHandler()

	result := handler.HandleModify(nil, &ldap.ModifyRequest{})
	if result.ResultCode != ldap.ResultUnwillingToPerform {
		t.Errorf("Default modify should return UnwillingToPerform, got %v", result.ResultCode)
	}
}

func TestHandlerNilHandler(t *testing.T) {
	handler := &Handler{}

	// All handlers should return UnwillingToPerform when nil
	bindResult := handler.HandleBind(nil, &ldap.BindRequest{})
	if bindResult.ResultCode != ldap.ResultUnwillingToPerform {
		t.Errorf("Nil bind handler should return UnwillingToPerform, got %v", bindResult.ResultCode)
	}

	searchResult := handler.HandleSearch(nil, &ldap.SearchRequest{})
	if searchResult.ResultCode != ldap.ResultUnwillingToPerform {
		t.Errorf("Nil search handler should return UnwillingToPerform, got %v", searchResult.ResultCode)
	}

	addResult := handler.HandleAdd(nil, &ldap.AddRequest{})
	if addResult.ResultCode != ldap.ResultUnwillingToPerform {
		t.Errorf("Nil add handler should return UnwillingToPerform, got %v", addResult.ResultCode)
	}

	deleteResult := handler.HandleDelete(nil, &ldap.DeleteRequest{})
	if deleteResult.ResultCode != ldap.ResultUnwillingToPerform {
		t.Errorf("Nil delete handler should return UnwillingToPerform, got %v", deleteResult.ResultCode)
	}

	modifyResult := handler.HandleModify(nil, &ldap.ModifyRequest{})
	if modifyResult.ResultCode != ldap.ResultUnwillingToPerform {
		t.Errorf("Nil modify handler should return UnwillingToPerform, got %v", modifyResult.ResultCode)
	}
}

func TestOperationResult(t *testing.T) {
	result := &OperationResult{
		ResultCode:        ldap.ResultNoSuchObject,
		MatchedDN:         "dc=example,dc=com",
		DiagnosticMessage: "Entry not found",
	}

	if result.ResultCode != ldap.ResultNoSuchObject {
		t.Errorf("Expected NoSuchObject, got %v", result.ResultCode)
	}
	if result.MatchedDN != "dc=example,dc=com" {
		t.Errorf("Expected matched DN, got %s", result.MatchedDN)
	}
	if result.DiagnosticMessage != "Entry not found" {
		t.Errorf("Expected diagnostic message, got %s", result.DiagnosticMessage)
	}
}

func TestSearchEntry(t *testing.T) {
	entry := &SearchEntry{
		DN: "cn=test,dc=example,dc=com",
		Attributes: []ldap.Attribute{
			{Type: "cn", Values: [][]byte{[]byte("test")}},
			{Type: "objectClass", Values: [][]byte{[]byte("top"), []byte("person")}},
		},
	}

	if entry.DN != "cn=test,dc=example,dc=com" {
		t.Errorf("Expected DN, got %s", entry.DN)
	}
	if len(entry.Attributes) != 2 {
		t.Errorf("Expected 2 attributes, got %d", len(entry.Attributes))
	}
	if entry.Attributes[0].Type != "cn" {
		t.Errorf("Expected cn attribute, got %s", entry.Attributes[0].Type)
	}
}

func TestSearchResult(t *testing.T) {
	result := &SearchResult{
		OperationResult: OperationResult{
			ResultCode: ldap.ResultSuccess,
		},
		Entries: []*SearchEntry{
			{DN: "cn=test1,dc=example,dc=com"},
			{DN: "cn=test2,dc=example,dc=com"},
		},
	}

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Expected Success, got %v", result.ResultCode)
	}
	if len(result.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(result.Entries))
	}
}

func TestResultCodeString(t *testing.T) {
	tests := []struct {
		code     ldap.ResultCode
		expected string
	}{
		{ldap.ResultSuccess, "Success"},
		{ldap.ResultOperationsError, "OperationsError"},
		{ldap.ResultProtocolError, "ProtocolError"},
		{ldap.ResultNoSuchObject, "NoSuchObject"},
		{ldap.ResultInvalidCredentials, "InvalidCredentials"},
		{ldap.ResultUnwillingToPerform, "UnwillingToPerform"},
		{ldap.ResultCode(999), "Unknown(999)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.code.String(); got != tt.expected {
				t.Errorf("ResultCode.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
