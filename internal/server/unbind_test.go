// Package server provides the LDAP server implementation.
package server

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/ber"
	"github.com/KilimcininKorOglu/oba/internal/ldap"
)

func TestParseUnbindRequest(t *testing.T) {
	// UnbindRequest has no content, so any data should work
	testCases := []struct {
		name string
		data []byte
	}{
		{"empty data", []byte{}},
		{"null data", nil},
		{"some data", []byte{0x00}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := ParseUnbindRequest(tc.data)
			if err != nil {
				t.Errorf("ParseUnbindRequest failed: %v", err)
			}
			if req == nil {
				t.Error("ParseUnbindRequest returned nil request")
			}
		})
	}
}

func TestNewUnbindProcessor(t *testing.T) {
	processor := NewUnbindProcessor()
	if processor == nil {
		t.Fatal("NewUnbindProcessor returned nil")
	}
	if processor.handler == nil {
		t.Error("handler should not be nil")
	}
}

func TestUnbindProcessorSetHandler(t *testing.T) {
	processor := NewUnbindProcessor()
	called := false

	processor.SetHandler(func(conn *Connection, req *UnbindRequest) error {
		called = true
		return nil
	})

	err := processor.Handle(nil, &UnbindRequest{})
	if err != nil {
		t.Errorf("Handle returned error: %v", err)
	}
	if !called {
		t.Error("Custom handler was not called")
	}
}

func TestUnbindProcessorHandleNilHandler(t *testing.T) {
	processor := &UnbindProcessor{handler: nil}

	// Should use default handler when handler is nil
	err := processor.Handle(nil, &UnbindRequest{})
	if err != nil {
		t.Errorf("Handle with nil handler returned error: %v", err)
	}
}

func TestDefaultUnbindHandlerNilConnection(t *testing.T) {
	err := defaultUnbindHandler(nil, &UnbindRequest{})
	if err != nil {
		t.Errorf("defaultUnbindHandler with nil connection returned error: %v", err)
	}
}

func TestDefaultUnbindHandlerResetsState(t *testing.T) {
	// Create a mock connection
	mockConn := &unbindMockConn{
		readBuf:  new(bytes.Buffer),
		writeBuf: new(bytes.Buffer),
	}
	conn := NewConnection(mockConn, nil)

	// Set some authentication state
	conn.mu.Lock()
	conn.bindDN = "cn=admin,dc=example,dc=com"
	conn.authenticated = true
	conn.mu.Unlock()

	// Verify state is set
	if conn.BindDN() != "cn=admin,dc=example,dc=com" {
		t.Error("BindDN not set correctly before unbind")
	}
	if !conn.IsAuthenticated() {
		t.Error("Should be authenticated before unbind")
	}

	// Call unbind handler
	err := defaultUnbindHandler(conn, &UnbindRequest{})
	if err != nil {
		t.Errorf("defaultUnbindHandler returned error: %v", err)
	}

	// Verify state is reset
	if conn.BindDN() != "" {
		t.Errorf("BindDN should be empty after unbind, got: %s", conn.BindDN())
	}
	if conn.IsAuthenticated() {
		t.Error("Should not be authenticated after unbind")
	}
}

func TestUnbindClosesConnection(t *testing.T) {
	mockConn := &unbindMockConn{
		readBuf:  new(bytes.Buffer),
		writeBuf: new(bytes.Buffer),
	}
	handler := NewHandler()
	server := &Server{Handler: handler}
	conn := NewConnection(mockConn, server)

	// Create an unbind request message
	msgData := createUnbindMessage(1)
	mockConn.setReadData(msgData)

	// Handle should return after unbind and close the connection
	done := make(chan struct{})
	go func() {
		conn.Handle()
		close(done)
	}()

	select {
	case <-done:
		// Verify connection is closed
		if !conn.isClosed() {
			t.Error("Connection should be closed after unbind")
		}
		if !mockConn.isClosed() {
			t.Error("Underlying connection should be closed after unbind")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not return after unbind")
	}
}

func TestUnbindNoResponse(t *testing.T) {
	mockConn := &unbindMockConn{
		readBuf:  new(bytes.Buffer),
		writeBuf: new(bytes.Buffer),
	}
	handler := NewHandler()
	server := &Server{Handler: handler}
	conn := NewConnection(mockConn, server)

	// Create an unbind request message
	msgData := createUnbindMessage(1)
	mockConn.setReadData(msgData)

	// Handle should return after unbind
	done := make(chan struct{})
	go func() {
		conn.Handle()
		close(done)
	}()

	select {
	case <-done:
		// Verify no response was written (per RFC 4511)
		written := mockConn.getWrittenData()
		if len(written) > 0 {
			t.Errorf("No response should be sent for unbind, but got %d bytes", len(written))
		}
	case <-time.After(time.Second):
		t.Error("Handle did not return after unbind")
	}
}

func TestUnbindAfterBind(t *testing.T) {
	mockConn := &unbindMockConn{
		readBuf:  new(bytes.Buffer),
		writeBuf: new(bytes.Buffer),
	}
	handler := NewHandler()
	server := &Server{Handler: handler}
	conn := NewConnection(mockConn, server)

	// Create a bind request followed by unbind
	bindMsg := createBindMessage(1, 3, "cn=admin,dc=example,dc=com", "")
	unbindMsg := createUnbindMessage(2)
	mockConn.setReadData(append(bindMsg, unbindMsg...))

	done := make(chan struct{})
	go func() {
		conn.Handle()
		close(done)
	}()

	select {
	case <-done:
		// Connection should be closed
		if !conn.isClosed() {
			t.Error("Connection should be closed after unbind")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}

func TestServerContinuesAfterUnbind(t *testing.T) {
	// This test verifies that the server can accept new connections
	// after one connection unbinds

	// First connection
	mockConn1 := &unbindMockConn{
		readBuf:  new(bytes.Buffer),
		writeBuf: new(bytes.Buffer),
	}
	handler := NewHandler()
	server := &Server{Handler: handler}
	conn1 := NewConnection(mockConn1, server)

	// Unbind first connection
	mockConn1.setReadData(createUnbindMessage(1))

	done1 := make(chan struct{})
	go func() {
		conn1.Handle()
		close(done1)
	}()

	select {
	case <-done1:
		// First connection closed
	case <-time.After(time.Second):
		t.Fatal("First connection did not close")
	}

	// Second connection should work fine
	mockConn2 := &unbindMockConn{
		readBuf:  new(bytes.Buffer),
		writeBuf: new(bytes.Buffer),
	}
	conn2 := NewConnection(mockConn2, server)

	// Unbind second connection
	mockConn2.setReadData(createUnbindMessage(1))

	done2 := make(chan struct{})
	go func() {
		conn2.Handle()
		close(done2)
	}()

	select {
	case <-done2:
		// Second connection also closed successfully
		if !conn2.isClosed() {
			t.Error("Second connection should be closed")
		}
	case <-time.After(time.Second):
		t.Error("Second connection did not close")
	}
}

func TestUnbindRequestStruct(t *testing.T) {
	// Verify UnbindRequest struct exists and can be instantiated
	req := &UnbindRequest{}
	if req == nil {
		t.Error("UnbindRequest should not be nil")
	}
}

func TestUnbindConcurrentAccess(t *testing.T) {
	processor := NewUnbindProcessor()

	// Test concurrent access to the processor
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = processor.Handle(nil, &UnbindRequest{})
		}()
	}
	wg.Wait()
}

// Helper types and functions for unbind tests

// unbindMockConn implements net.Conn for testing
type unbindMockConn struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
	closed   bool
	mu       sync.Mutex
}

func (m *unbindMockConn) Read(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, net.ErrClosed
	}
	return m.readBuf.Read(b)
}

func (m *unbindMockConn) Write(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, net.ErrClosed
	}
	return m.writeBuf.Write(b)
}

func (m *unbindMockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *unbindMockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 389}
}

func (m *unbindMockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 54321}
}

func (m *unbindMockConn) SetDeadline(t time.Time) error      { return nil }
func (m *unbindMockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *unbindMockConn) SetWriteDeadline(t time.Time) error { return nil }

func (m *unbindMockConn) setReadData(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readBuf.Reset()
	m.readBuf.Write(data)
}

func (m *unbindMockConn) getWrittenData() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeBuf.Bytes()
}

func (m *unbindMockConn) isClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// createUnbindMessage creates an unbind request message
func createUnbindMessage(messageID int) []byte {
	msgEncoder := ber.NewBEREncoder(64)
	seqPos := msgEncoder.BeginSequence()
	msgEncoder.WriteInteger(int64(messageID))
	// UnbindRequest is APPLICATION 2 with NULL content
	appPos := msgEncoder.WriteApplicationTag(ldap.ApplicationUnbindRequest, false)
	msgEncoder.EndApplicationTag(appPos)
	msgEncoder.EndSequence(seqPos)

	return msgEncoder.Bytes()
}

// createBindMessage creates a bind request message
func createBindMessage(messageID int, version int, dn string, password string) []byte {
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
