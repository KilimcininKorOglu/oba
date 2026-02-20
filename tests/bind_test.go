// Package tests provides integration tests for the Oba LDAP server.
package tests

import (
	"net"
	"testing"
	"time"

	"github.com/oba-ldap/oba/internal/ber"
	"github.com/oba-ldap/oba/internal/ldap"
)

// TestIntegrationBind tests bind operations end-to-end.
func TestIntegrationBind(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start test server
	srv, err := NewTestServer(nil)
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer srv.Stop()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	t.Run("anonymous_bind", func(t *testing.T) {
		testAnonymousBind(t, srv)
	})

	t.Run("simple_bind_success", func(t *testing.T) {
		testSimpleBindSuccess(t, srv)
	})

	t.Run("simple_bind_wrong_password", func(t *testing.T) {
		testSimpleBindWrongPassword(t, srv)
	})

	t.Run("simple_bind_invalid_dn", func(t *testing.T) {
		testSimpleBindInvalidDN(t, srv)
	})

	t.Run("bind_version_check", func(t *testing.T) {
		testBindVersionCheck(t, srv)
	})
}

// testAnonymousBind tests anonymous bind operation.
func testAnonymousBind(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Send anonymous bind request
	bindReq := createBindRequest(1, 3, "", "")
	if err := sendMessage(conn, bindReq); err != nil {
		t.Fatalf("failed to send bind request: %v", err)
	}

	// Read response
	resp, err := readMessage(conn)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	// Verify response
	if resp.MessageID != 1 {
		t.Errorf("expected message ID 1, got %d", resp.MessageID)
	}

	resultCode := parseBindResponse(resp)
	if resultCode != ldap.ResultSuccess {
		t.Errorf("expected success, got result code %d", resultCode)
	}
}

// testSimpleBindSuccess tests successful simple bind.
func testSimpleBindSuccess(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	cfg := srv.Config()

	// Send simple bind request with correct credentials
	bindReq := createBindRequest(1, 3, cfg.RootDN, cfg.RootPassword)
	if err := sendMessage(conn, bindReq); err != nil {
		t.Fatalf("failed to send bind request: %v", err)
	}

	// Read response
	resp, err := readMessage(conn)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	resultCode := parseBindResponse(resp)
	if resultCode != ldap.ResultSuccess {
		t.Errorf("expected success, got result code %d", resultCode)
	}
}

// testSimpleBindWrongPassword tests bind with wrong password.
func testSimpleBindWrongPassword(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	cfg := srv.Config()

	// Send simple bind request with wrong password
	bindReq := createBindRequest(1, 3, cfg.RootDN, "wrongpassword")
	if err := sendMessage(conn, bindReq); err != nil {
		t.Fatalf("failed to send bind request: %v", err)
	}

	// Read response
	resp, err := readMessage(conn)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	resultCode := parseBindResponse(resp)
	if resultCode != ldap.ResultInvalidCredentials {
		t.Errorf("expected invalid credentials, got result code %d", resultCode)
	}
}

// testSimpleBindInvalidDN tests bind with non-existent DN.
func testSimpleBindInvalidDN(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Send simple bind request with non-existent DN
	bindReq := createBindRequest(1, 3, "cn=nonexistent,dc=test,dc=com", "password")
	if err := sendMessage(conn, bindReq); err != nil {
		t.Fatalf("failed to send bind request: %v", err)
	}

	// Read response
	resp, err := readMessage(conn)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	resultCode := parseBindResponse(resp)
	if resultCode != ldap.ResultInvalidCredentials {
		t.Errorf("expected invalid credentials, got result code %d", resultCode)
	}
}

// testBindVersionCheck tests that only LDAP v3 is accepted.
func testBindVersionCheck(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Send bind request with LDAP v2
	bindReq := createBindRequest(1, 2, "", "")
	if err := sendMessage(conn, bindReq); err != nil {
		t.Fatalf("failed to send bind request: %v", err)
	}

	// Read response
	resp, err := readMessage(conn)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	resultCode := parseBindResponse(resp)
	if resultCode != ldap.ResultProtocolError {
		t.Errorf("expected protocol error for LDAP v2, got result code %d", resultCode)
	}
}

// createBindRequest creates a bind request message.
func createBindRequest(messageID int, version int, dn, password string) *ldap.LDAPMessage {
	encoder := ber.NewBEREncoder(128)

	// Write version (INTEGER)
	if err := encoder.WriteInteger(int64(version)); err != nil {
		return nil
	}

	// Write name (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(dn)); err != nil {
		return nil
	}

	// Write authentication (simple - context tag [0])
	if err := encoder.WriteTaggedValue(0, false, []byte(password)); err != nil {
		return nil
	}

	return &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationBindRequest,
			Data: encoder.Bytes(),
		},
	}
}

// parseBindResponse parses a bind response and returns the result code.
func parseBindResponse(msg *ldap.LDAPMessage) ldap.ResultCode {
	if msg.Operation == nil {
		return ldap.ResultOperationsError
	}

	if msg.Operation.Tag != ldap.ApplicationBindResponse {
		return ldap.ResultOperationsError
	}

	decoder := ber.NewBERDecoder(msg.Operation.Data)

	// Read result code (ENUMERATED)
	resultCode, err := decoder.ReadEnumerated()
	if err != nil {
		return ldap.ResultOperationsError
	}

	return ldap.ResultCode(resultCode)
}

// sendMessage sends an LDAP message to the connection.
func sendMessage(conn net.Conn, msg *ldap.LDAPMessage) error {
	data, err := msg.Encode()
	if err != nil {
		return err
	}
	_, err = conn.Write(data)
	return err
}

// readMessage reads an LDAP message from the connection.
func readMessage(conn net.Conn) (*ldap.LDAPMessage, error) {
	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Read the tag byte
	tagBuf := make([]byte, 1)
	if _, err := conn.Read(tagBuf); err != nil {
		return nil, err
	}

	// Read the length
	length, lengthBytes, err := readLength(conn)
	if err != nil {
		return nil, err
	}

	// Read the message content
	content := make([]byte, length)
	if _, err := readFull(conn, content); err != nil {
		return nil, err
	}

	// Reconstruct the full message
	fullMessage := make([]byte, 1+len(lengthBytes)+length)
	fullMessage[0] = tagBuf[0]
	copy(fullMessage[1:], lengthBytes)
	copy(fullMessage[1+len(lengthBytes):], content)

	return ldap.ParseLDAPMessage(fullMessage)
}

// readLength reads a BER length from the connection.
func readLength(conn net.Conn) (int, []byte, error) {
	firstByte := make([]byte, 1)
	if _, err := conn.Read(firstByte); err != nil {
		return 0, nil, err
	}

	// Short form
	if firstByte[0]&0x80 == 0 {
		return int(firstByte[0]), firstByte, nil
	}

	// Long form
	numBytes := int(firstByte[0] & 0x7F)
	if numBytes == 0 {
		return 0, nil, ber.ErrInvalidLength
	}

	lengthBytes := make([]byte, numBytes)
	if _, err := readFull(conn, lengthBytes); err != nil {
		return 0, nil, err
	}

	length := 0
	for _, b := range lengthBytes {
		length = (length << 8) | int(b)
	}

	allLengthBytes := make([]byte, 1+numBytes)
	allLengthBytes[0] = firstByte[0]
	copy(allLengthBytes[1:], lengthBytes)

	return length, allLengthBytes, nil
}

// readFull reads exactly len(buf) bytes from the connection.
func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}
