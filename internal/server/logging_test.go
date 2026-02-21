// Package server provides the LDAP server implementation.
package server

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/ldap"
	"github.com/KilimcininKorOglu/oba/internal/logging"
)

// testLogger is a logger that captures output for testing.
type testLogger struct {
	buf       *bytes.Buffer
	requestID string
	fields    map[string]interface{}
}

func newTestLogger() *testLogger {
	return &testLogger{
		buf:    new(bytes.Buffer),
		fields: make(map[string]interface{}),
	}
}

func (l *testLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.log("debug", msg, keysAndValues...)
}

func (l *testLogger) Info(msg string, keysAndValues ...interface{}) {
	l.log("info", msg, keysAndValues...)
}

func (l *testLogger) Warn(msg string, keysAndValues ...interface{}) {
	l.log("warn", msg, keysAndValues...)
}

func (l *testLogger) Error(msg string, keysAndValues ...interface{}) {
	l.log("error", msg, keysAndValues...)
}

func (l *testLogger) WithRequestID(requestID string) logging.Logger {
	newLogger := &testLogger{
		buf:       l.buf,
		requestID: requestID,
		fields:    make(map[string]interface{}),
	}
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

func (l *testLogger) WithFields(keysAndValues ...interface{}) logging.Logger {
	newLogger := &testLogger{
		buf:       l.buf,
		requestID: l.requestID,
		fields:    make(map[string]interface{}),
	}
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for i := 0; i < len(keysAndValues)-1; i += 2 {
		if key, ok := keysAndValues[i].(string); ok {
			newLogger.fields[key] = keysAndValues[i+1]
		}
	}
	return newLogger
}

func (l *testLogger) log(level, msg string, keysAndValues ...interface{}) {
	entry := map[string]interface{}{
		"level": level,
		"msg":   msg,
	}
	if l.requestID != "" {
		entry["request_id"] = l.requestID
	}
	for k, v := range l.fields {
		entry[k] = v
	}
	for i := 0; i < len(keysAndValues)-1; i += 2 {
		if key, ok := keysAndValues[i].(string); ok {
			entry[key] = keysAndValues[i+1]
		}
	}
	data, _ := json.Marshal(entry)
	l.buf.Write(data)
	l.buf.WriteByte('\n')
}

func (l *testLogger) SetLevel(_ logging.Level)           {}
func (l *testLogger) SetFormat(_ logging.Format)         {}
func (l *testLogger) SetOutput(_ io.Writer)              {}
func (l *testLogger) GetLevel() logging.Level            { return logging.LevelInfo }
func (l *testLogger) GetFormat() logging.Format          { return logging.FormatJSON }
func (l *testLogger) SetStore(_ *logging.LogStore)       {}
func (l *testLogger) GetStore() *logging.LogStore        { return nil }
func (l *testLogger) CloseStore() error                  { return nil }
func (l *testLogger) WithSource(_ string) logging.Logger { return l }
func (l *testLogger) WithUser(_ string) logging.Logger   { return l }

func (l *testLogger) getOutput() string {
	return l.buf.String()
}

func (l *testLogger) reset() {
	l.buf.Reset()
}

func TestConnectionLoggingOnEstablish(t *testing.T) {
	mockConn := newMockConn()
	logger := newTestLogger()
	server := &Server{
		Handler: NewHandler(),
		Logger:  logger,
	}
	conn := NewConnection(mockConn, server)

	// Create an unbind request to close the connection
	unbindMsg := createUnbindRequestMessage(1)
	mockConn.setReadData(unbindMsg)

	done := make(chan struct{})
	go func() {
		conn.Handle()
		close(done)
	}()

	select {
	case <-done:
		output := logger.getOutput()
		if !strings.Contains(output, "connection established") {
			t.Error("Expected 'connection established' log message")
		}
		if !strings.Contains(output, "connection closed") {
			t.Error("Expected 'connection closed' log message")
		}
		if !strings.Contains(output, "192.168.1.100") {
			t.Error("Expected client address in log")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}

func TestConnectionLoggingBindSuccess(t *testing.T) {
	mockConn := newMockConn()
	logger := newTestLogger()
	handler := NewHandler()
	handler.SetBindHandler(func(conn *Connection, req *ldap.BindRequest) *OperationResult {
		return &OperationResult{ResultCode: ldap.ResultSuccess}
	})
	server := &Server{
		Handler: handler,
		Logger:  logger,
	}
	conn := NewConnection(mockConn, server)

	// Create a bind request followed by unbind
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
		output := logger.getOutput()
		if !strings.Contains(output, "bind request") {
			t.Error("Expected 'bind request' debug log message")
		}
		if !strings.Contains(output, "bind successful") {
			t.Error("Expected 'bind successful' log message")
		}
		if !strings.Contains(output, "cn=admin,dc=example,dc=com") {
			t.Error("Expected DN in log")
		}
		if !strings.Contains(output, "duration_ms") {
			t.Error("Expected duration_ms in log")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}

func TestConnectionLoggingBindFailure(t *testing.T) {
	mockConn := newMockConn()
	logger := newTestLogger()
	handler := NewHandler()
	handler.SetBindHandler(func(conn *Connection, req *ldap.BindRequest) *OperationResult {
		return &OperationResult{
			ResultCode:        ldap.ResultInvalidCredentials,
			DiagnosticMessage: "invalid password",
		}
	})
	server := &Server{
		Handler: handler,
		Logger:  logger,
	}
	conn := NewConnection(mockConn, server)

	// Create a bind request followed by unbind
	bindMsg := createBindRequestMessage(1, 3, "cn=admin,dc=example,dc=com", "wrong")
	unbindMsg := createUnbindRequestMessage(2)
	mockConn.setReadData(append(bindMsg, unbindMsg...))

	done := make(chan struct{})
	go func() {
		conn.Handle()
		close(done)
	}()

	select {
	case <-done:
		output := logger.getOutput()
		if !strings.Contains(output, "bind failed") {
			t.Error("Expected 'bind failed' log message")
		}
		if !strings.Contains(output, "invalidCredentials") {
			t.Error("Expected result code in log")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}

func TestConnectionLoggingSearch(t *testing.T) {
	mockConn := newMockConn()
	logger := newTestLogger()
	handler := NewHandler()
	handler.SetSearchHandler(func(conn *Connection, req *ldap.SearchRequest) *SearchResult {
		return &SearchResult{
			OperationResult: OperationResult{ResultCode: ldap.ResultSuccess},
			Entries: []*SearchEntry{
				{DN: "cn=test,dc=example,dc=com"},
			},
		}
	})
	server := &Server{
		Handler: handler,
		Logger:  logger,
	}
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
		output := logger.getOutput()
		if !strings.Contains(output, "search request") {
			t.Error("Expected 'search request' debug log message")
		}
		if !strings.Contains(output, "search completed") {
			t.Error("Expected 'search completed' log message")
		}
		if !strings.Contains(output, "dc=example,dc=com") {
			t.Error("Expected base DN in log")
		}
		if !strings.Contains(output, `"results":1`) {
			t.Error("Expected results count in log")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}

func TestConnectionLoggingAdd(t *testing.T) {
	mockConn := newMockConn()
	logger := newTestLogger()
	handler := NewHandler()
	handler.SetAddHandler(func(conn *Connection, req *ldap.AddRequest) *OperationResult {
		return &OperationResult{ResultCode: ldap.ResultSuccess}
	})
	server := &Server{
		Handler: handler,
		Logger:  logger,
	}
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
		output := logger.getOutput()
		if !strings.Contains(output, "add request") {
			t.Error("Expected 'add request' debug log message")
		}
		if !strings.Contains(output, "add successful") {
			t.Error("Expected 'add successful' log message")
		}
		if !strings.Contains(output, "cn=test,dc=example,dc=com") {
			t.Error("Expected entry DN in log")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}

func TestConnectionLoggingDelete(t *testing.T) {
	mockConn := newMockConn()
	logger := newTestLogger()
	handler := NewHandler()
	handler.SetDeleteHandler(func(conn *Connection, req *ldap.DeleteRequest) *OperationResult {
		return &OperationResult{ResultCode: ldap.ResultSuccess}
	})
	server := &Server{
		Handler: handler,
		Logger:  logger,
	}
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
		output := logger.getOutput()
		if !strings.Contains(output, "delete request") {
			t.Error("Expected 'delete request' debug log message")
		}
		if !strings.Contains(output, "delete successful") {
			t.Error("Expected 'delete successful' log message")
		}
		if !strings.Contains(output, "cn=test,dc=example,dc=com") {
			t.Error("Expected DN in log")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}

func TestConnectionLoggingModify(t *testing.T) {
	mockConn := newMockConn()
	logger := newTestLogger()
	handler := NewHandler()
	handler.SetModifyHandler(func(conn *Connection, req *ldap.ModifyRequest) *OperationResult {
		return &OperationResult{ResultCode: ldap.ResultSuccess}
	})
	server := &Server{
		Handler: handler,
		Logger:  logger,
	}
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
		output := logger.getOutput()
		if !strings.Contains(output, "modify request") {
			t.Error("Expected 'modify request' debug log message")
		}
		if !strings.Contains(output, "modify successful") {
			t.Error("Expected 'modify successful' log message")
		}
		if !strings.Contains(output, "cn=test,dc=example,dc=com") {
			t.Error("Expected object DN in log")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}

func TestConnectionRequestID(t *testing.T) {
	mockConn := newMockConn()
	logger := newTestLogger()
	server := &Server{
		Handler: NewHandler(),
		Logger:  logger,
	}
	conn := NewConnection(mockConn, server)

	// Check that request ID is generated
	if conn.RequestID() == "" {
		t.Error("Expected non-empty request ID")
	}

	// Check that request ID is in logs
	unbindMsg := createUnbindRequestMessage(1)
	mockConn.setReadData(unbindMsg)

	done := make(chan struct{})
	go func() {
		conn.Handle()
		close(done)
	}()

	select {
	case <-done:
		output := logger.getOutput()
		if !strings.Contains(output, "request_id") {
			t.Error("Expected request_id in log output")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}

func TestConnectionLogger(t *testing.T) {
	mockConn := newMockConn()
	logger := newTestLogger()
	server := &Server{
		Handler: NewHandler(),
		Logger:  logger,
	}
	conn := NewConnection(mockConn, server)

	// Check that Logger() returns a non-nil logger
	if conn.Logger() == nil {
		t.Error("Expected non-nil logger")
	}
}

func TestConnectionSetTLS(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	if conn.IsTLS() {
		t.Error("Expected IsTLS to be false initially")
	}

	conn.SetTLS(true)

	if !conn.IsTLS() {
		t.Error("Expected IsTLS to be true after SetTLS(true)")
	}
}

func TestConnectionSetLogger(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	newLogger := newTestLogger()
	conn.SetLogger(newLogger)

	// The logger should be set
	if conn.Logger() == nil {
		t.Error("Expected non-nil logger after SetLogger")
	}
}

func TestConnectionLoggingWithNilServer(t *testing.T) {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	// Should not panic with nil server
	if conn.Logger() == nil {
		t.Error("Expected non-nil logger even with nil server")
	}

	// Request ID should still be generated
	if conn.RequestID() == "" {
		t.Error("Expected non-empty request ID even with nil server")
	}
}

func TestConnectionLoggingProtocolError(t *testing.T) {
	mockConn := newMockConn()
	logger := newTestLogger()
	server := &Server{
		Handler: NewHandler(),
		Logger:  logger,
	}
	conn := NewConnection(mockConn, server)

	// Write invalid data
	invalidData := []byte{0x30, 0x05, 0x02, 0x01, 0x01, 0xFF, 0xFF}
	mockConn.setReadData(invalidData)

	done := make(chan struct{})
	go func() {
		conn.Handle()
		close(done)
	}()

	select {
	case <-done:
		output := logger.getOutput()
		// Should log connection established and closed
		if !strings.Contains(output, "connection established") {
			t.Error("Expected 'connection established' log message")
		}
		if !strings.Contains(output, "connection closed") {
			t.Error("Expected 'connection closed' log message")
		}
	case <-time.After(time.Second):
		t.Error("Handle did not complete")
	}
}
