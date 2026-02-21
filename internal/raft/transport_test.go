package raft

import (
	"net"
	"sync"
	"testing"
	"time"
)

func TestTCPTransportSendReceive(t *testing.T) {
	// Create two transports
	peers1 := map[uint64]string{2: "127.0.0.1:14446"}
	peers2 := map[uint64]string{1: "127.0.0.1:14445"}

	transport1 := NewTCPTransport("127.0.0.1:14445", peers1)
	transport2 := NewTCPTransport("127.0.0.1:14446", peers2)

	defer transport1.Close()
	defer transport2.Close()

	// Start listening on transport2
	received := make(chan []byte, 1)
	err := transport2.Listen(func(msgType uint8, data []byte) []byte {
		received <- data
		return []byte("response")
	})
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	// Give listener time to start
	time.Sleep(50 * time.Millisecond)

	// Send from transport1 to transport2
	resp, err := transport1.Send(2, RPCRequestVote, []byte("hello"))
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Check response
	if string(resp) != "response" {
		t.Errorf("Response mismatch: got %s", string(resp))
	}

	// Check received data
	select {
	case data := <-received:
		if string(data) != "hello" {
			t.Errorf("Received data mismatch: got %s", string(data))
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for received data")
	}
}

func TestTCPTransportConnectionReuse(t *testing.T) {
	peers1 := map[uint64]string{2: "127.0.0.1:14448"}
	peers2 := map[uint64]string{1: "127.0.0.1:14447"}

	transport1 := NewTCPTransport("127.0.0.1:14447", peers1)
	transport2 := NewTCPTransport("127.0.0.1:14448", peers2)

	defer transport1.Close()
	defer transport2.Close()

	callCount := 0
	var mu sync.Mutex

	err := transport2.Listen(func(msgType uint8, data []byte) []byte {
		mu.Lock()
		callCount++
		mu.Unlock()
		return []byte("ok")
	})
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Send multiple messages
	for i := 0; i < 5; i++ {
		_, err := transport1.Send(2, RPCAppendEntries, []byte("msg"))
		if err != nil {
			t.Fatalf("Send %d failed: %v", i, err)
		}
	}

	mu.Lock()
	if callCount != 5 {
		t.Errorf("Expected 5 calls, got %d", callCount)
	}
	mu.Unlock()
}

func TestTCPTransportClose(t *testing.T) {
	transport := NewTCPTransport("127.0.0.1:14449", nil)

	err := transport.Listen(func(msgType uint8, data []byte) []byte {
		return nil
	})
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	// Close should not error
	if err := transport.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Double close should be safe
	if err := transport.Close(); err != nil {
		t.Errorf("Double close failed: %v", err)
	}

	// Send after close should fail
	_, err = transport.Send(1, RPCRequestVote, []byte("test"))
	if err != ErrTransportClosed {
		t.Errorf("Expected ErrTransportClosed, got %v", err)
	}
}

func TestTCPTransportConnectFailed(t *testing.T) {
	// No peer at this address
	peers := map[uint64]string{2: "127.0.0.1:19999"}
	transport := NewTCPTransport("127.0.0.1:14450", peers)
	transport.SetTimeout(100 * time.Millisecond)
	defer transport.Close()

	_, err := transport.Send(2, RPCRequestVote, []byte("test"))
	if err == nil {
		t.Error("Expected connection error")
	}
}

func TestTCPTransportUnknownPeer(t *testing.T) {
	transport := NewTCPTransport("127.0.0.1:14451", nil)
	defer transport.Close()

	_, err := transport.Send(999, RPCRequestVote, []byte("test"))
	if err != ErrConnectFailed {
		t.Errorf("Expected ErrConnectFailed, got %v", err)
	}
}

func TestTCPTransportAddRemovePeer(t *testing.T) {
	transport := NewTCPTransport("127.0.0.1:14452", nil)
	defer transport.Close()

	// Initially no peers
	_, err := transport.Send(2, RPCRequestVote, []byte("test"))
	if err != ErrConnectFailed {
		t.Errorf("Expected ErrConnectFailed for unknown peer")
	}

	// Add peer
	transport.AddPeer(2, "127.0.0.1:14453")

	// Remove peer
	transport.RemovePeer(2)

	// Should fail again
	_, err = transport.Send(2, RPCRequestVote, []byte("test"))
	if err != ErrConnectFailed {
		t.Errorf("Expected ErrConnectFailed after remove")
	}
}

func TestInMemoryTransport(t *testing.T) {
	network := NewInMemoryNetwork()

	transport1 := network.NewTransport(1, "node1:4445")
	transport2 := network.NewTransport(2, "node2:4445")

	received := make(chan []byte, 1)
	transport2.Listen(func(msgType uint8, data []byte) []byte {
		received <- data
		return []byte("pong")
	})

	resp, err := transport1.Send(2, RPCRequestVote, []byte("ping"))
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if string(resp) != "pong" {
		t.Errorf("Response mismatch: got %s", string(resp))
	}

	select {
	case data := <-received:
		if string(data) != "ping" {
			t.Errorf("Received data mismatch")
		}
	default:
		t.Error("No data received")
	}
}

func TestInMemoryTransportClose(t *testing.T) {
	network := NewInMemoryNetwork()

	transport1 := network.NewTransport(1, "node1:4445")
	transport2 := network.NewTransport(2, "node2:4445")

	transport2.Listen(func(msgType uint8, data []byte) []byte {
		return []byte("ok")
	})

	// Close transport2
	transport2.Close()

	// Send should fail
	_, err := transport1.Send(2, RPCRequestVote, []byte("test"))
	if err != ErrConnectFailed {
		t.Errorf("Expected ErrConnectFailed, got %v", err)
	}
}

func TestInMemoryTransportUnknownPeer(t *testing.T) {
	network := NewInMemoryNetwork()
	transport1 := network.NewTransport(1, "node1:4445")

	_, err := transport1.Send(999, RPCRequestVote, []byte("test"))
	if err != ErrConnectFailed {
		t.Errorf("Expected ErrConnectFailed, got %v", err)
	}
}

func TestTCPTransportLocalAddr(t *testing.T) {
	transport := NewTCPTransport("127.0.0.1:14454", nil)
	if transport.LocalAddr() != "127.0.0.1:14454" {
		t.Errorf("LocalAddr mismatch")
	}
}

func TestInMemoryTransportLocalAddr(t *testing.T) {
	network := NewInMemoryNetwork()
	transport := network.NewTransport(1, "node1:4445")
	if transport.LocalAddr() != "node1:4445" {
		t.Errorf("LocalAddr mismatch")
	}
}

func getFreePort() int {
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
