// Package server provides the LDAP server implementation.
package server

import (
	"net"
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/ber"
	"github.com/KilimcininKorOglu/oba/internal/ldap"
)

// BenchmarkSearchBase benchmarks base-scope search operations.
// PRD target: 50,000+ search operations/sec (simple).
func BenchmarkSearchBase(b *testing.B) {
	handler := NewHandler()
	handler.SetSearchHandler(func(_ *Connection, req *ldap.SearchRequest) *SearchResult {
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode: ldap.ResultSuccess,
			},
			Entries: []*SearchEntry{
				{
					DN: req.BaseObject,
					Attributes: []ldap.Attribute{
						{Type: "objectClass", Values: [][]byte{[]byte("organization")}},
						{Type: "o", Values: [][]byte{[]byte("Example Inc")}},
					},
				},
			},
		}
	})

	// Create a mock connection
	conn := createBenchmarkConnection(handler)

	// Create a search request
	searchReq := createSearchRequest(1, "dc=test,dc=com", ldap.ScopeBaseObject)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = conn.handleSearch(searchReq)
	}
}

// BenchmarkSearchOneLevel benchmarks one-level scope search operations.
func BenchmarkSearchOneLevel(b *testing.B) {
	handler := NewHandler()
	handler.SetSearchHandler(func(_ *Connection, _ *ldap.SearchRequest) *SearchResult {
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode: ldap.ResultSuccess,
			},
			Entries: []*SearchEntry{
				{DN: "ou=users,dc=test,dc=com", Attributes: []ldap.Attribute{{Type: "ou", Values: [][]byte{[]byte("users")}}}},
				{DN: "ou=groups,dc=test,dc=com", Attributes: []ldap.Attribute{{Type: "ou", Values: [][]byte{[]byte("groups")}}}},
			},
		}
	})

	conn := createBenchmarkConnection(handler)
	searchReq := createSearchRequest(1, "dc=test,dc=com", ldap.ScopeSingleLevel)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = conn.handleSearch(searchReq)
	}
}

// BenchmarkSearchSubtree benchmarks subtree scope search operations.
func BenchmarkSearchSubtree(b *testing.B) {
	handler := NewHandler()
	handler.SetSearchHandler(func(_ *Connection, _ *ldap.SearchRequest) *SearchResult {
		entries := make([]*SearchEntry, 10)
		for i := 0; i < 10; i++ {
			entries[i] = &SearchEntry{
				DN: "uid=user" + string(rune('0'+i)) + ",ou=users,dc=test,dc=com",
				Attributes: []ldap.Attribute{
					{Type: "cn", Values: [][]byte{[]byte("User " + string(rune('0'+i)))}},
					{Type: "uid", Values: [][]byte{[]byte("user" + string(rune('0'+i)))}},
				},
			}
		}
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode: ldap.ResultSuccess,
			},
			Entries: entries,
		}
	})

	conn := createBenchmarkConnection(handler)
	searchReq := createSearchRequest(1, "dc=test,dc=com", ldap.ScopeWholeSubtree)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = conn.handleSearch(searchReq)
	}
}

// BenchmarkBind benchmarks bind operations.
func BenchmarkBind(b *testing.B) {
	handler := NewHandler()
	handler.SetBindHandler(func(_ *Connection, _ *ldap.BindRequest) *OperationResult {
		return &OperationResult{
			ResultCode: ldap.ResultSuccess,
		}
	})

	conn := createBenchmarkConnection(handler)
	bindReq := createBindRequest(1, "cn=admin,dc=test,dc=com", "password")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = conn.handleBind(bindReq)
	}
}

// BenchmarkAnonymousBind benchmarks anonymous bind operations.
func BenchmarkAnonymousBind(b *testing.B) {
	handler := NewHandler()

	conn := createBenchmarkConnection(handler)
	bindReq := createBindRequest(1, "", "")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = conn.handleBind(bindReq)
	}
}

// BenchmarkAdd benchmarks add operations.
// PRD target: 10,000+ write ops/s.
func BenchmarkAdd(b *testing.B) {
	handler := NewHandler()
	handler.SetAddHandler(func(_ *Connection, _ *ldap.AddRequest) *OperationResult {
		return &OperationResult{
			ResultCode: ldap.ResultSuccess,
		}
	})

	conn := createBenchmarkConnection(handler)
	addReq := createAddRequest(1, "uid=test,ou=users,dc=test,dc=com")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = conn.handleAdd(addReq)
	}
}

// BenchmarkDelete benchmarks delete operations.
func BenchmarkDelete(b *testing.B) {
	handler := NewHandler()
	handler.SetDeleteHandler(func(_ *Connection, _ *ldap.DeleteRequest) *OperationResult {
		return &OperationResult{
			ResultCode: ldap.ResultSuccess,
		}
	})

	conn := createBenchmarkConnection(handler)
	deleteReq := createDeleteRequest(1, "uid=test,ou=users,dc=test,dc=com")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = conn.handleDelete(deleteReq)
	}
}

// BenchmarkModify benchmarks modify operations.
func BenchmarkModify(b *testing.B) {
	handler := NewHandler()
	handler.SetModifyHandler(func(_ *Connection, _ *ldap.ModifyRequest) *OperationResult {
		return &OperationResult{
			ResultCode: ldap.ResultSuccess,
		}
	})

	conn := createBenchmarkConnection(handler)
	modifyReq := createModifyRequest(1, "uid=test,ou=users,dc=test,dc=com")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = conn.handleModify(modifyReq)
	}
}

// BenchmarkCompare benchmarks compare operations.
func BenchmarkCompare(b *testing.B) {
	handler := NewHandler()
	handler.SetCompareHandler(func(_ *Connection, _ *ldap.CompareRequest) *OperationResult {
		return &OperationResult{
			ResultCode: ldap.ResultCompareTrue,
		}
	})

	conn := createBenchmarkConnection(handler)
	compareReq := createCompareRequest(1, "uid=test,ou=users,dc=test,dc=com", "uid", "test")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = conn.handleCompare(compareReq)
	}
}

// BenchmarkModifyDN benchmarks modifydn operations.
func BenchmarkModifyDN(b *testing.B) {
	handler := NewHandler()
	handler.SetModifyDNHandler(func(_ *Connection, _ *ldap.ModifyDNRequest) *OperationResult {
		return &OperationResult{
			ResultCode: ldap.ResultSuccess,
		}
	})

	conn := createBenchmarkConnection(handler)
	modifyDNReq := createModifyDNRequest(1, "uid=old,ou=users,dc=test,dc=com", "uid=new")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = conn.handleModifyDN(modifyDNReq)
	}
}

// BenchmarkMessageParsing benchmarks LDAP message parsing.
func BenchmarkMessageParsing(b *testing.B) {
	// Create a search request message
	enc := ber.NewBEREncoder(256)
	msgPos := enc.BeginSequence()
	_ = enc.WriteInteger(1) // messageID
	reqPos := enc.WriteApplicationTag(3, true)
	_ = enc.WriteOctetString([]byte("dc=test,dc=com"))
	_ = enc.WriteEnumerated(2) // scope: wholeSubtree
	_ = enc.WriteEnumerated(0) // derefAliases: never
	_ = enc.WriteInteger(0)    // sizeLimit
	_ = enc.WriteInteger(0)    // timeLimit
	_ = enc.WriteBoolean(false)
	_ = enc.WriteTaggedValue(7, false, []byte("objectClass"))
	attrPos := enc.BeginSequence()
	_ = enc.WriteOctetString([]byte("cn"))
	_ = enc.EndSequence(attrPos)
	_ = enc.EndApplicationTag(reqPos)
	_ = enc.EndSequence(msgPos)
	data := enc.Bytes()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = ldap.ParseLDAPMessage(data)
	}
}

// BenchmarkMessageEncoding benchmarks LDAP message encoding.
func BenchmarkMessageEncoding(b *testing.B) {
	enc := ber.NewBEREncoder(256)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
		msgPos := enc.BeginSequence()
		_ = enc.WriteInteger(int64(i))
		reqPos := enc.WriteApplicationTag(5, true) // SearchResultDone
		_ = enc.WriteEnumerated(0)                 // resultCode: success
		_ = enc.WriteOctetString([]byte(""))       // matchedDN
		_ = enc.WriteOctetString([]byte(""))       // diagnosticMessage
		_ = enc.EndApplicationTag(reqPos)
		_ = enc.EndSequence(msgPos)
	}
}

// BenchmarkSearchResultEncoding benchmarks search result entry encoding.
func BenchmarkSearchResultEncoding(b *testing.B) {
	enc := ber.NewBEREncoder(512)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
		msgPos := enc.BeginSequence()
		_ = enc.WriteInteger(int64(i))
		entryPos := enc.WriteApplicationTag(4, true) // SearchResultEntry
		_ = enc.WriteOctetString([]byte("uid=alice,ou=users,dc=test,dc=com"))
		attrsPos := enc.BeginSequence()
		// cn attribute
		attrPos := enc.BeginSequence()
		_ = enc.WriteOctetString([]byte("cn"))
		valsPos := enc.BeginSet()
		_ = enc.WriteOctetString([]byte("Alice Smith"))
		_ = enc.EndSet(valsPos)
		_ = enc.EndSequence(attrPos)
		// mail attribute
		attrPos = enc.BeginSequence()
		_ = enc.WriteOctetString([]byte("mail"))
		valsPos = enc.BeginSet()
		_ = enc.WriteOctetString([]byte("alice@example.com"))
		_ = enc.EndSet(valsPos)
		_ = enc.EndSequence(attrPos)
		_ = enc.EndSequence(attrsPos)
		_ = enc.EndApplicationTag(entryPos)
		_ = enc.EndSequence(msgPos)
	}
}

// BenchmarkConnectionCreate benchmarks connection creation.
func BenchmarkConnectionCreate(b *testing.B) {
	server := &Server{
		Handler: NewHandler(),
	}

	// Create a mock connection pair
	client, serverConn := net.Pipe()
	defer client.Close()
	defer serverConn.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		conn := NewConnection(serverConn, server)
		_ = conn
	}
}

// BenchmarkDispatchMessage benchmarks message dispatching.
func BenchmarkDispatchMessage(b *testing.B) {
	handler := NewHandler()
	handler.SetSearchHandler(func(_ *Connection, _ *ldap.SearchRequest) *SearchResult {
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode: ldap.ResultSuccess,
			},
		}
	})

	conn := createBenchmarkConnection(handler)
	msg := createSearchRequest(1, "dc=test,dc=com", ldap.ScopeBaseObject)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = conn.dispatchMessage(msg)
	}
}

// BenchmarkErrorResponse benchmarks error response creation.
func BenchmarkErrorResponse(b *testing.B) {
	conn := createBenchmarkConnection(NewHandler())

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = conn.createErrorResponse(i, ldap.ResultNoSuchObject, "entry not found")
	}
}

// BenchmarkFilterParsing benchmarks filter parsing.
func BenchmarkFilterParsing(b *testing.B) {
	// Create a complex filter: (&(objectClass=person)(|(cn=Alice*)(mail=*@example.com)))
	enc := ber.NewBEREncoder(256)

	// AND filter
	andPos := enc.WriteContextTag(0, true)
	// objectClass=person
	eqPos := enc.WriteContextTag(3, true)
	_ = enc.WriteOctetString([]byte("objectClass"))
	_ = enc.WriteOctetString([]byte("person"))
	_ = enc.EndContextTag(eqPos)
	// OR filter
	orPos := enc.WriteContextTag(1, true)
	// cn=Alice*
	subPos := enc.WriteContextTag(4, true)
	_ = enc.WriteOctetString([]byte("cn"))
	subSeqPos := enc.BeginSequence()
	_ = enc.WriteTaggedValue(0, false, []byte("Alice"))
	_ = enc.EndSequence(subSeqPos)
	_ = enc.EndContextTag(subPos)
	// mail=*@example.com
	subPos = enc.WriteContextTag(4, true)
	_ = enc.WriteOctetString([]byte("mail"))
	subSeqPos = enc.BeginSequence()
	_ = enc.WriteTaggedValue(2, false, []byte("@example.com"))
	_ = enc.EndSequence(subSeqPos)
	_ = enc.EndContextTag(subPos)
	_ = enc.EndContextTag(orPos)
	_ = enc.EndContextTag(andPos)

	filterData := enc.Bytes()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := ber.NewBERDecoder(filterData)
		_, _, _, _ = dec.ReadTaggedValue()
	}
}

// BenchmarkAttributeSelection benchmarks attribute selection parsing.
func BenchmarkAttributeSelection(b *testing.B) {
	// Create attribute list
	enc := ber.NewBEREncoder(128)
	pos := enc.BeginSequence()
	_ = enc.WriteOctetString([]byte("cn"))
	_ = enc.WriteOctetString([]byte("mail"))
	_ = enc.WriteOctetString([]byte("uid"))
	_ = enc.WriteOctetString([]byte("objectClass"))
	_ = enc.EndSequence(pos)
	data := enc.Bytes()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := ber.NewBERDecoder(data)
		_, _ = dec.ExpectSequence()
		for dec.Remaining() > 0 {
			_, _ = dec.ReadOctetString()
		}
	}
}

// Helper functions for benchmarks

func createBenchmarkConnection(handler *Handler) *Connection {
	// Create a mock connection using a pipe
	client, serverConn := net.Pipe()
	defer client.Close()

	server := &Server{
		Handler: handler,
	}

	conn := NewConnection(serverConn, server)
	return conn
}

func createSearchRequest(msgID int, baseDN string, scope ldap.SearchScope) *ldap.LDAPMessage {
	// Create search request data
	enc := ber.NewBEREncoder(256)
	_ = enc.WriteOctetString([]byte(baseDN))
	_ = enc.WriteEnumerated(int64(scope))
	_ = enc.WriteEnumerated(0) // derefAliases
	_ = enc.WriteInteger(0)    // sizeLimit
	_ = enc.WriteInteger(0)    // timeLimit
	_ = enc.WriteBoolean(false)
	_ = enc.WriteTaggedValue(7, false, []byte("objectClass"))
	attrPos := enc.BeginSequence()
	_ = enc.WriteOctetString([]byte("*"))
	_ = enc.EndSequence(attrPos)

	return &ldap.LDAPMessage{
		MessageID: msgID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationSearchRequest,
			Data: enc.Bytes(),
		},
	}
}

func createBindRequest(msgID int, dn, password string) *ldap.LDAPMessage {
	enc := ber.NewBEREncoder(128)
	_ = enc.WriteInteger(3) // version
	_ = enc.WriteOctetString([]byte(dn))
	_ = enc.WriteTaggedValue(0, false, []byte(password))

	return &ldap.LDAPMessage{
		MessageID: msgID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationBindRequest,
			Data: enc.Bytes(),
		},
	}
}

func createAddRequest(msgID int, dn string) *ldap.LDAPMessage {
	enc := ber.NewBEREncoder(256)
	_ = enc.WriteOctetString([]byte(dn))
	attrsPos := enc.BeginSequence()
	// objectClass attribute
	attrPos := enc.BeginSequence()
	_ = enc.WriteOctetString([]byte("objectClass"))
	valsPos := enc.BeginSet()
	_ = enc.WriteOctetString([]byte("person"))
	_ = enc.EndSet(valsPos)
	_ = enc.EndSequence(attrPos)
	// cn attribute
	attrPos = enc.BeginSequence()
	_ = enc.WriteOctetString([]byte("cn"))
	valsPos = enc.BeginSet()
	_ = enc.WriteOctetString([]byte("Test User"))
	_ = enc.EndSet(valsPos)
	_ = enc.EndSequence(attrPos)
	_ = enc.EndSequence(attrsPos)

	return &ldap.LDAPMessage{
		MessageID: msgID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationAddRequest,
			Data: enc.Bytes(),
		},
	}
}

func createDeleteRequest(msgID int, dn string) *ldap.LDAPMessage {
	return &ldap.LDAPMessage{
		MessageID: msgID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationDelRequest,
			Data: []byte(dn),
		},
	}
}

func createModifyRequest(msgID int, dn string) *ldap.LDAPMessage {
	enc := ber.NewBEREncoder(256)
	_ = enc.WriteOctetString([]byte(dn))
	changesPos := enc.BeginSequence()
	// Replace operation
	changePos := enc.BeginSequence()
	_ = enc.WriteEnumerated(2) // replace
	attrPos := enc.BeginSequence()
	_ = enc.WriteOctetString([]byte("description"))
	valsPos := enc.BeginSet()
	_ = enc.WriteOctetString([]byte("Updated description"))
	_ = enc.EndSet(valsPos)
	_ = enc.EndSequence(attrPos)
	_ = enc.EndSequence(changePos)
	_ = enc.EndSequence(changesPos)

	return &ldap.LDAPMessage{
		MessageID: msgID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationModifyRequest,
			Data: enc.Bytes(),
		},
	}
}

func createCompareRequest(msgID int, dn, attr, value string) *ldap.LDAPMessage {
	enc := ber.NewBEREncoder(128)
	_ = enc.WriteOctetString([]byte(dn))
	avaPos := enc.BeginSequence()
	_ = enc.WriteOctetString([]byte(attr))
	_ = enc.WriteOctetString([]byte(value))
	_ = enc.EndSequence(avaPos)

	return &ldap.LDAPMessage{
		MessageID: msgID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationCompareRequest,
			Data: enc.Bytes(),
		},
	}
}

func createModifyDNRequest(msgID int, dn, newRDN string) *ldap.LDAPMessage {
	enc := ber.NewBEREncoder(128)
	_ = enc.WriteOctetString([]byte(dn))
	_ = enc.WriteOctetString([]byte(newRDN))
	_ = enc.WriteBoolean(true) // deleteOldRDN

	return &ldap.LDAPMessage{
		MessageID: msgID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationModifyDNRequest,
			Data: enc.Bytes(),
		},
	}
}

// BenchmarkReadMessage benchmarks reading LDAP messages from a connection.
func BenchmarkReadMessage(b *testing.B) {
	// Create a search request message
	enc := ber.NewBEREncoder(256)
	msgPos := enc.BeginSequence()
	_ = enc.WriteInteger(1)
	reqPos := enc.WriteApplicationTag(3, true)
	_ = enc.WriteOctetString([]byte("dc=test,dc=com"))
	_ = enc.WriteEnumerated(0)
	_ = enc.WriteEnumerated(0)
	_ = enc.WriteInteger(0)
	_ = enc.WriteInteger(0)
	_ = enc.WriteBoolean(false)
	_ = enc.WriteTaggedValue(7, false, []byte("objectClass"))
	attrPos := enc.BeginSequence()
	_ = enc.EndSequence(attrPos)
	_ = enc.EndApplicationTag(reqPos)
	_ = enc.EndSequence(msgPos)
	msgData := enc.Bytes()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = ldap.ParseLDAPMessage(msgData)
	}
}

// BenchmarkWriteMessage benchmarks writing LDAP messages.
func BenchmarkWriteMessage(b *testing.B) {
	enc := ber.NewBEREncoder(256)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
		// SearchResultDone
		msgPos := enc.BeginSequence()
		_ = enc.WriteInteger(int64(i))
		resPos := enc.WriteApplicationTag(5, true)
		_ = enc.WriteEnumerated(0)
		_ = enc.WriteOctetString([]byte(""))
		_ = enc.WriteOctetString([]byte(""))
		_ = enc.EndApplicationTag(resPos)
		_ = enc.EndSequence(msgPos)
		_ = enc.Bytes()
	}
}

// BenchmarkConcurrentSearches benchmarks concurrent search handling.
func BenchmarkConcurrentSearches(b *testing.B) {
	handler := NewHandler()
	handler.SetSearchHandler(func(_ *Connection, _ *ldap.SearchRequest) *SearchResult {
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode: ldap.ResultSuccess,
			},
			Entries: []*SearchEntry{
				{DN: "dc=test,dc=com", Attributes: []ldap.Attribute{}},
			},
		}
	})

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		conn := createBenchmarkConnection(handler)
		searchReq := createSearchRequest(1, "dc=test,dc=com", ldap.ScopeBaseObject)

		for pb.Next() {
			_ = conn.handleSearch(searchReq)
		}
	})
}

// BenchmarkConnectionTimeout benchmarks connection timeout handling.
func BenchmarkConnectionTimeout(b *testing.B) {
	client, serverConn := net.Pipe()
	defer client.Close()
	defer serverConn.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = serverConn.SetReadDeadline(time.Now().Add(30 * time.Second))
	}
}
