// Package server provides the LDAP server implementation.
package server

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/ber"
	"github.com/KilimcininKorOglu/oba/internal/ldap"
)

// mockConn implements net.Conn for testing
type mockConn struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
	closed   bool
	mu       sync.Mutex
}

func newMockConn() *mockConn {
	return &mockConn{
		readBuf:  new(bytes.Buffer),
		writeBuf: new(bytes.Buffer),
	}
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, net.ErrClosed
	}
	return m.readBuf.Read(b)
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, net.ErrClosed
	}
	return m.writeBuf.Write(b)
}

func (m *mockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 389}
}

func (m *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 54321}
}

func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func (m *mockConn) setReadData(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readBuf.Reset()
	m.readBuf.Write(data)
}

func (m *mockConn) getWrittenData() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeBuf.Bytes()
}

// Helper function to create a bind request message
func createBindRequestMessage(messageID int, version int, dn string, password string) []byte {
	// Create bind request content
	bindEncoder := ber.NewBEREncoder(128)
	bindEncoder.WriteInteger(int64(version))
	bindEncoder.WriteOctetString([]byte(dn))
	// Simple auth: context tag [0] with password
	bindEncoder.WriteTaggedValue(0, false, []byte(password))
	bindData := bindEncoder.Bytes()

	// Create the full message
	msgEncoder := ber.NewBEREncoder(256)
	seqPos := msgEncoder.BeginSequence()
	msgEncoder.WriteInteger(int64(messageID))
	appPos := msgEncoder.WriteApplicationTag(ldap.ApplicationBindRequest, true)
	msgEncoder.WriteRaw(bindData)
	msgEncoder.EndApplicationTag(appPos)
	msgEncoder.EndSequence(seqPos)

	return msgEncoder.Bytes()
}

// Helper function to create an unbind request message
func createUnbindRequestMessage(messageID int) []byte {
	msgEncoder := ber.NewBEREncoder(64)
	seqPos := msgEncoder.BeginSequence()
	msgEncoder.WriteInteger(int64(messageID))
	// UnbindRequest is APPLICATION 2 with NULL content
	appPos := msgEncoder.WriteApplicationTag(ldap.ApplicationUnbindRequest, false)
	msgEncoder.EndApplicationTag(appPos)
	msgEncoder.EndSequence(seqPos)

	return msgEncoder.Bytes()
}

// Helper function to create a search request message
func createSearchRequestMessage(messageID int, baseDN string) []byte {
	// Create search request content
	searchEncoder := ber.NewBEREncoder(256)
	searchEncoder.WriteOctetString([]byte(baseDN))       // baseObject
	searchEncoder.WriteEnumerated(0)                     // scope: baseObject
	searchEncoder.WriteEnumerated(0)                     // derefAliases: neverDerefAliases
	searchEncoder.WriteInteger(0)                        // sizeLimit
	searchEncoder.WriteInteger(0)                        // timeLimit
	searchEncoder.WriteBoolean(false)                    // typesOnly
	searchEncoder.WriteTaggedValue(7, false, []byte("objectClass")) // present filter
	attrSeqPos := searchEncoder.BeginSequence()          // attributes
	searchEncoder.EndSequence(attrSeqPos)
	searchData := searchEncoder.Bytes()

	// Create the full message
	msgEncoder := ber.NewBEREncoder(512)
	seqPos := msgEncoder.BeginSequence()
	msgEncoder.WriteInteger(int64(messageID))
	appPos := msgEncoder.WriteApplicationTag(ldap.ApplicationSearchRequest, true)
	msgEncoder.WriteRaw(searchData)
	msgEncoder.EndApplicationTag(appPos)
	msgEncoder.EndSequence(seqPos)

	return msgEncoder.Bytes()
}

// Helper function to create an add request message
func createAddRequestMessage(messageID int, dn string) []byte {
	// Create add request content
	addEncoder := ber.NewBEREncoder(256)
	addEncoder.WriteOctetString([]byte(dn)) // entry DN

	// Attributes sequence
	attrListPos := addEncoder.BeginSequence()
	// Add one attribute
	attrPos := addEncoder.BeginSequence()
	addEncoder.WriteOctetString([]byte("objectClass"))
	valSetPos := addEncoder.BeginSet()
	addEncoder.WriteOctetString([]byte("top"))
	addEncoder.EndSet(valSetPos)
	addEncoder.EndSequence(attrPos)
	addEncoder.EndSequence(attrListPos)

	addData := addEncoder.Bytes()

	// Create the full message
	msgEncoder := ber.NewBEREncoder(512)
	seqPos := msgEncoder.BeginSequence()
	msgEncoder.WriteInteger(int64(messageID))
	appPos := msgEncoder.WriteApplicationTag(ldap.ApplicationAddRequest, true)
	msgEncoder.WriteRaw(addData)
	msgEncoder.EndApplicationTag(appPos)
	msgEncoder.EndSequence(seqPos)

	return msgEncoder.Bytes()
}

// Helper function to create a delete request message
func createDeleteRequestMessage(messageID int, dn string) []byte {
	// Create the full message
	msgEncoder := ber.NewBEREncoder(256)
	seqPos := msgEncoder.BeginSequence()
	msgEncoder.WriteInteger(int64(messageID))
	// DelRequest is APPLICATION 10 with DN as primitive content
	appPos := msgEncoder.WriteApplicationTag(ldap.ApplicationDelRequest, false)
	msgEncoder.WriteRaw([]byte(dn))
	msgEncoder.EndApplicationTag(appPos)
	msgEncoder.EndSequence(seqPos)

	return msgEncoder.Bytes()
}

// Helper function to create a modify request message
func createModifyRequestMessage(messageID int, dn string) []byte {
	// Create modify request content
	modEncoder := ber.NewBEREncoder(256)
	modEncoder.WriteOctetString([]byte(dn)) // object DN

	// Changes sequence
	changesPos := modEncoder.BeginSequence()
	// Add one change
	changePos := modEncoder.BeginSequence()
	modEncoder.WriteEnumerated(2) // replace operation
	// Modification (PartialAttribute)
	attrPos := modEncoder.BeginSequence()
	modEncoder.WriteOctetString([]byte("description"))
	valSetPos := modEncoder.BeginSet()
	modEncoder.WriteOctetString([]byte("test value"))
	modEncoder.EndSet(valSetPos)
	modEncoder.EndSequence(attrPos)
	modEncoder.EndSequence(changePos)
	modEncoder.EndSequence(changesPos)

	modData := modEncoder.Bytes()

	// Create the full message
	msgEncoder := ber.NewBEREncoder(512)
	seqPos := msgEncoder.BeginSequence()
	msgEncoder.WriteInteger(int64(messageID))
	appPos := msgEncoder.WriteApplicationTag(ldap.ApplicationModifyRequest, true)
	msgEncoder.WriteRaw(modData)
	msgEncoder.EndApplicationTag(appPos)
	msgEncoder.EndSequence(seqPos)

	return msgEncoder.Bytes()
}

func TestNewConnection(t *testing.T) {
	mockConn := newMockConn()
	server := &Server{}

	conn := NewConnection(mockConn, server)

	if conn == nil {
		t.Fatal("NewConnection returned nil")
	}
	if conn.conn != mockConn {
		t.Error("Connection.conn not set correctly")
	}
	if conn.server != server {
		t.Error("Connection.server not set correctly")
	}
	if conn.bindDN != "" {
		t.Error("Connection.bindDN should be empty initially")
	}
	if conn.authenticated {
		t.Error("Connection.authenticated should be false initially")
	}
	if conn.handler == nil {
		t.Error("Connection.handler should not be nil")
	}
}

func TestConnectionReadMessage(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	// Create a valid bind request message
	msgData := createBindRequestMessage(1, 3, "cn=admin,dc=example,dc=com", "secret")
	mockConn.setReadData(msgData)

	msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	if msg.MessageID != 1 {
		t.Errorf("Expected message ID 1, got %d", msg.MessageID)
	}
	if msg.OperationType() != ldap.OperationType(ldap.ApplicationBindRequest) {
		t.Errorf("Expected BindRequest, got %v", msg.OperationType())
	}
}

func TestConnectionReadMessageEOF(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	// Empty buffer should return EOF
	_, err := conn.ReadMessage()
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
}

func TestConnectionReadMessageInvalidTag(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	// Write invalid tag (not a SEQUENCE)
	mockConn.setReadData([]byte{0x01, 0x01, 0x00})

	_, err := conn.ReadMessage()
	if err != ErrInvalidMessage {
		t.Errorf("Expected ErrInvalidMessage, got %v", err)
	}
}

func TestConnectionWriteMessage(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	// Create a simple response message
	msg := &ldap.LDAPMessage{
		MessageID: 1,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationBindResponse,
			Data: []byte{0x0a, 0x01, 0x00, 0x04, 0x00, 0x04, 0x00}, // resultCode=0, matchedDN="", diagnosticMessage=""
		},
	}

	err := conn.WriteMessage(msg)
	if err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	written := mockConn.getWrittenData()
	if len(written) == 0 {
		t.Error("No data written to connection")
	}
}

func TestConnectionWriteMessageClosed(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	// Close the connection
	conn.Close()

	msg := &ldap.LDAPMessage{
		MessageID: 1,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationBindResponse,
			Data: []byte{0x0a, 0x01, 0x00, 0x04, 0x00, 0x04, 0x00},
		},
	}

	err := conn.WriteMessage(msg)
	if err != ErrConnectionClosed {
		t.Errorf("Expected ErrConnectionClosed, got %v", err)
	}
}

func TestConnectionClose(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	err := conn.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !conn.isClosed() {
		t.Error("Connection should be marked as closed")
	}

	// Second close should be idempotent
	err = conn.Close()
	if err != nil {
		t.Errorf("Second Close should not return error, got %v", err)
	}
}

func TestConnectionBindDN(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	if conn.BindDN() != "" {
		t.Error("BindDN should be empty initially")
	}

	// Simulate successful bind
	conn.mu.Lock()
	conn.bindDN = "cn=admin,dc=example,dc=com"
	conn.authenticated = true
	conn.mu.Unlock()

	if conn.BindDN() != "cn=admin,dc=example,dc=com" {
		t.Error("BindDN not updated correctly")
	}
	if !conn.IsAuthenticated() {
		t.Error("IsAuthenticated should return true after bind")
	}
}

func TestConnectionRemoteAddr(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	addr := conn.RemoteAddr()
	if addr == nil {
		t.Error("RemoteAddr should not be nil")
	}
	if addr.String() != "192.168.1.100:54321" {
		t.Errorf("Unexpected remote address: %s", addr.String())
	}
}

func TestConnectionLocalAddr(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	addr := conn.LocalAddr()
	if addr == nil {
		t.Error("LocalAddr should not be nil")
	}
	if addr.String() != "127.0.0.1:389" {
		t.Errorf("Unexpected local address: %s", addr.String())
	}
}

func TestConnectionHandleUnbind(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	// Create an unbind request
	msgData := createUnbindRequestMessage(1)
	mockConn.setReadData(msgData)

	// Handle should return after unbind
	done := make(chan struct{})
	go func() {
		conn.Handle()
		close(done)
	}()

	select {
	case <-done:
		// Expected
	case <-time.After(time.Second):
		t.Error("Handle did not return after unbind")
	}
}

func TestConnectionHandleBindRequest(t *testing.T) {
	mockConn := newMockConn()
	handler := NewHandler()
	server := &Server{Handler: handler}
	conn := NewConnection(mockConn, server)

	// Create an anonymous bind request followed by unbind
	bindMsg := createBindRequestMessage(1, 3, "", "")
	unbindMsg := createUnbindRequestMessage(2)
	mockConn.setReadData(append(bindMsg, unbindMsg...))

	done := make(chan struct{})
	go func() {
		conn.Handle()
		close(done)
	}()

	select {
	case <-done:
		// Check that a response was written
		written := mockConn.getWrittenData()
		if len(written) == 0 {
			t.Error("No response written for bind request")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}

func TestConnectionHandleSearchRequest(t *testing.T) {
	mockConn := newMockConn()
	handler := NewHandler()
	server := &Server{Handler: handler}
	conn := NewConnection(mockConn, server)

	// Create a search request followed by unbind
	searchMsg := createSearchRequestMessage(1, "dc=example,dc=com")
	unbindMsg := createUnbindRequestMessage(2)
	mockConn.setReadData(append(searchMsg, unbindMsg...))

	done := make(chan struct{})
	go func() {
		conn.Handle()
		close(done)
	}()

	select {
	case <-done:
		// Check that a response was written
		written := mockConn.getWrittenData()
		if len(written) == 0 {
			t.Error("No response written for search request")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}

func TestConnectionHandleAddRequest(t *testing.T) {
	mockConn := newMockConn()
	handler := NewHandler()
	server := &Server{Handler: handler}
	conn := NewConnection(mockConn, server)

	// Create an add request followed by unbind
	addMsg := createAddRequestMessage(1, "cn=test,dc=example,dc=com")
	unbindMsg := createUnbindRequestMessage(2)
	mockConn.setReadData(append(addMsg, unbindMsg...))

	done := make(chan struct{})
	go func() {
		conn.Handle()
		close(done)
	}()

	select {
	case <-done:
		written := mockConn.getWrittenData()
		if len(written) == 0 {
			t.Error("No response written for add request")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}

func TestConnectionHandleDeleteRequest(t *testing.T) {
	mockConn := newMockConn()
	handler := NewHandler()
	server := &Server{Handler: handler}
	conn := NewConnection(mockConn, server)

	// Create a delete request followed by unbind
	deleteMsg := createDeleteRequestMessage(1, "cn=test,dc=example,dc=com")
	unbindMsg := createUnbindRequestMessage(2)
	mockConn.setReadData(append(deleteMsg, unbindMsg...))

	done := make(chan struct{})
	go func() {
		conn.Handle()
		close(done)
	}()

	select {
	case <-done:
		written := mockConn.getWrittenData()
		if len(written) == 0 {
			t.Error("No response written for delete request")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}

func TestConnectionHandleModifyRequest(t *testing.T) {
	mockConn := newMockConn()
	handler := NewHandler()
	server := &Server{Handler: handler}
	conn := NewConnection(mockConn, server)

	// Create a modify request followed by unbind
	modifyMsg := createModifyRequestMessage(1, "cn=test,dc=example,dc=com")
	unbindMsg := createUnbindRequestMessage(2)
	mockConn.setReadData(append(modifyMsg, unbindMsg...))

	done := make(chan struct{})
	go func() {
		conn.Handle()
		close(done)
	}()

	select {
	case <-done:
		written := mockConn.getWrittenData()
		if len(written) == 0 {
			t.Error("No response written for modify request")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}

func TestConnectionStateTracking(t *testing.T) {
	mockConn := newMockConn()
	handler := NewHandler()

	// Set up a custom bind handler that succeeds
	handler.SetBindHandler(func(conn *Connection, req *ldap.BindRequest) *OperationResult {
		return &OperationResult{
			ResultCode: ldap.ResultSuccess,
		}
	})

	server := &Server{Handler: handler}
	conn := NewConnection(mockConn, server)

	// Create a bind request with DN followed by unbind
	bindMsg := createBindRequestMessage(1, 3, "cn=admin,dc=example,dc=com", "secret")
	unbindMsg := createUnbindRequestMessage(2)
	mockConn.setReadData(append(bindMsg, unbindMsg...))

	done := make(chan struct{})
	go func() {
		conn.Handle()
		close(done)
	}()

	select {
	case <-done:
		// After successful bind, state should be updated
		if conn.BindDN() != "cn=admin,dc=example,dc=com" {
			t.Errorf("BindDN not updated, got: %s", conn.BindDN())
		}
		if !conn.IsAuthenticated() {
			t.Error("Should be authenticated after successful bind")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}

func TestConnectionErrorRecovery(t *testing.T) {
	mockConn := newMockConn()
	handler := NewHandler()
	server := &Server{Handler: handler}
	conn := NewConnection(mockConn, server)

	// Write invalid data followed by valid unbind
	invalidData := []byte{0x30, 0x05, 0x02, 0x01, 0x01, 0xFF, 0xFF} // Invalid operation
	mockConn.setReadData(invalidData)

	done := make(chan struct{})
	go func() {
		conn.Handle()
		close(done)
	}()

	select {
	case <-done:
		// Handle should exit gracefully on error
	case <-time.After(time.Second):
		t.Error("Handle did not exit on error")
	}
}

func TestConnectionConcurrentAccess(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	// Test concurrent access to connection state
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = conn.BindDN()
			_ = conn.IsAuthenticated()
			_ = conn.isClosed()
		}()
	}
	wg.Wait()
}

func TestCreateBindResponse(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	response := conn.createBindResponse(1, ldap.ResultSuccess, "", "")
	if response == nil {
		t.Fatal("createBindResponse returned nil")
	}
	if response.MessageID != 1 {
		t.Errorf("Expected message ID 1, got %d", response.MessageID)
	}
	if response.Operation.Tag != ldap.ApplicationBindResponse {
		t.Errorf("Expected BindResponse tag, got %d", response.Operation.Tag)
	}
}

func TestCreateSearchDoneResponse(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	response := conn.createSearchDoneResponse(1, ldap.ResultSuccess, "", "")
	if response == nil {
		t.Fatal("createSearchDoneResponse returned nil")
	}
	if response.Operation.Tag != ldap.ApplicationSearchResultDone {
		t.Errorf("Expected SearchResultDone tag, got %d", response.Operation.Tag)
	}
}

func TestCreateSearchEntryResponse(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	entry := &SearchEntry{
		DN: "cn=test,dc=example,dc=com",
		Attributes: []ldap.Attribute{
			{Type: "cn", Values: [][]byte{[]byte("test")}},
			{Type: "objectClass", Values: [][]byte{[]byte("top"), []byte("person")}},
		},
	}

	response := conn.createSearchEntryResponse(1, entry)
	if response == nil {
		t.Fatal("createSearchEntryResponse returned nil")
	}
	if response.Operation.Tag != ldap.ApplicationSearchResultEntry {
		t.Errorf("Expected SearchResultEntry tag, got %d", response.Operation.Tag)
	}
}

func TestCreateAddResponse(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	response := conn.createAddResponse(1, ldap.ResultSuccess, "", "")
	if response == nil {
		t.Fatal("createAddResponse returned nil")
	}
	if response.Operation.Tag != ldap.ApplicationAddResponse {
		t.Errorf("Expected AddResponse tag, got %d", response.Operation.Tag)
	}
}

func TestCreateDeleteResponse(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	response := conn.createDeleteResponse(1, ldap.ResultSuccess, "", "")
	if response == nil {
		t.Fatal("createDeleteResponse returned nil")
	}
	if response.Operation.Tag != ldap.ApplicationDelResponse {
		t.Errorf("Expected DelResponse tag, got %d", response.Operation.Tag)
	}
}

func TestCreateModifyResponse(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	response := conn.createModifyResponse(1, ldap.ResultSuccess, "", "")
	if response == nil {
		t.Fatal("createModifyResponse returned nil")
	}
	if response.Operation.Tag != ldap.ApplicationModifyResponse {
		t.Errorf("Expected ModifyResponse tag, got %d", response.Operation.Tag)
	}
}
