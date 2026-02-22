package raft

import (
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"
)

// Transport defines the interface for Raft RPC communication.
type Transport interface {
	// Send sends an RPC to a peer and waits for response.
	Send(peerID uint64, msgType uint8, data []byte) ([]byte, error)

	// Listen starts listening for incoming RPCs.
	Listen(handler RPCHandler) error

	// Close shuts down the transport.
	Close() error

	// LocalAddr returns the local address.
	LocalAddr() string
}

// RPCHandler handles incoming RPC messages.
// Returns the response data to send back.
type RPCHandler func(msgType uint8, data []byte) []byte

// TCPTransport implements Transport using TCP.
type TCPTransport struct {
	addr     string
	listener net.Listener
	peers    map[uint64]string   // peerID -> address
	conns    map[uint64]net.Conn // peerID -> connection
	handler  RPCHandler
	timeout  time.Duration
	closed   bool
	mu       sync.RWMutex
	wg       sync.WaitGroup
}

// NewTCPTransport creates a new TCP transport.
func NewTCPTransport(addr string, peers map[uint64]string) *TCPTransport {
	return &TCPTransport{
		addr:    addr,
		peers:   peers,
		conns:   make(map[uint64]net.Conn),
		timeout: 5 * time.Second,
	}
}

// SetTimeout sets the connection timeout.
func (t *TCPTransport) SetTimeout(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.timeout = d
}

// LocalAddr returns the local address.
func (t *TCPTransport) LocalAddr() string {
	return t.addr
}

// Send sends an RPC message to a peer and waits for response.
// Message format: [type:1][length:4][data:N]
func (t *TCPTransport) Send(peerID uint64, msgType uint8, data []byte) ([]byte, error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, ErrTransportClosed
	}

	conn, ok := t.conns[peerID]
	if !ok || conn == nil {
		addr, exists := t.peers[peerID]
		if !exists {
			t.mu.Unlock()
			return nil, ErrConnectFailed
		}

		var err error
		conn, err = net.DialTimeout("tcp", addr, t.timeout)
		if err != nil {
			t.mu.Unlock()
			return nil, err
		}
		t.conns[peerID] = conn
	}
	t.mu.Unlock()

	// Set deadline for this operation
	conn.SetDeadline(time.Now().Add(t.timeout))

	// Write message: [type:1][length:4][data:N]
	header := make([]byte, 5)
	header[0] = msgType
	binary.LittleEndian.PutUint32(header[1:5], uint32(len(data)))

	if _, err := conn.Write(header); err != nil {
		t.removeConn(peerID)
		return nil, err
	}
	if _, err := conn.Write(data); err != nil {
		t.removeConn(peerID)
		return nil, err
	}

	// Read response header
	respHeader := make([]byte, 5)
	if _, err := io.ReadFull(conn, respHeader); err != nil {
		t.removeConn(peerID)
		return nil, err
	}

	// Read response data
	respLen := binary.LittleEndian.Uint32(respHeader[1:5])
	respData := make([]byte, respLen)
	if respLen > 0 {
		if _, err := io.ReadFull(conn, respData); err != nil {
			t.removeConn(peerID)
			return nil, err
		}
	}

	return respData, nil
}

// Listen starts accepting connections and handling RPCs.
func (t *TCPTransport) Listen(handler RPCHandler) error {
	var err error
	t.listener, err = net.Listen("tcp", t.addr)
	if err != nil {
		return err
	}

	t.handler = handler

	t.wg.Add(1)
	go t.acceptLoop()

	return nil
}

func (t *TCPTransport) acceptLoop() {
	defer t.wg.Done()

	for {
		conn, err := t.listener.Accept()
		if err != nil {
			t.mu.RLock()
			closed := t.closed
			t.mu.RUnlock()
			if closed {
				return
			}
			continue
		}

		t.wg.Add(1)
		go t.handleConn(conn)
	}
}

func (t *TCPTransport) handleConn(conn net.Conn) {
	defer t.wg.Done()
	defer conn.Close()

	for {
		t.mu.RLock()
		closed := t.closed
		t.mu.RUnlock()
		if closed {
			return
		}

		// Set read deadline
		conn.SetReadDeadline(time.Now().Add(t.timeout * 2))

		// Read message header
		header := make([]byte, 5)
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}

		msgType := header[0]
		dataLen := binary.LittleEndian.Uint32(header[1:5])

		// Sanity check: prevent allocation of unreasonably large buffers
		if dataLen > 64*1024*1024 { // 64MB max
			return
		}

		// Read message data
		data := make([]byte, dataLen)
		if dataLen > 0 {
			if _, err := io.ReadFull(conn, data); err != nil {
				return
			}
		}

		// Handle the message
		var resp []byte
		if t.handler != nil {
			resp = t.handler(msgType, data)
		}

		// Write response
		respHeader := make([]byte, 5)
		respHeader[0] = msgType
		binary.LittleEndian.PutUint32(respHeader[1:5], uint32(len(resp)))

		conn.SetWriteDeadline(time.Now().Add(t.timeout))
		if _, err := conn.Write(respHeader); err != nil {
			return
		}
		if len(resp) > 0 {
			if _, err := conn.Write(resp); err != nil {
				return
			}
		}
	}
}

func (t *TCPTransport) removeConn(peerID uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if conn, ok := t.conns[peerID]; ok {
		conn.Close()
		delete(t.conns, peerID)
	}
}

// Close shuts down the transport.
func (t *TCPTransport) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	t.mu.Unlock()

	// Close listener
	if t.listener != nil {
		t.listener.Close()
	}

	// Close all peer connections
	t.mu.Lock()
	for _, conn := range t.conns {
		conn.Close()
	}
	t.conns = make(map[uint64]net.Conn)
	t.mu.Unlock()

	// Wait for goroutines
	t.wg.Wait()

	return nil
}

// AddPeer adds a new peer to the transport.
func (t *TCPTransport) AddPeer(peerID uint64, addr string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.peers == nil {
		t.peers = make(map[uint64]string)
	}
	t.peers[peerID] = addr
}

// RemovePeer removes a peer from the transport.
func (t *TCPTransport) RemovePeer(peerID uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.peers, peerID)
	if conn, ok := t.conns[peerID]; ok {
		conn.Close()
		delete(t.conns, peerID)
	}
}

// InMemoryTransport implements Transport for testing.
type InMemoryTransport struct {
	addr    string
	network *InMemoryNetwork
	handler RPCHandler
	closed  bool
	mu      sync.RWMutex
}

// InMemoryNetwork simulates a network for testing.
type InMemoryNetwork struct {
	transports map[uint64]*InMemoryTransport
	mu         sync.RWMutex
}

// NewInMemoryNetwork creates a new in-memory network.
func NewInMemoryNetwork() *InMemoryNetwork {
	return &InMemoryNetwork{
		transports: make(map[uint64]*InMemoryTransport),
	}
}

// NewTransport creates a new in-memory transport for a node.
func (n *InMemoryNetwork) NewTransport(nodeID uint64, addr string) *InMemoryTransport {
	t := &InMemoryTransport{
		addr:    addr,
		network: n,
	}

	n.mu.Lock()
	n.transports[nodeID] = t
	n.mu.Unlock()

	return t
}

// Send sends an RPC to a peer.
func (t *InMemoryTransport) Send(peerID uint64, msgType uint8, data []byte) ([]byte, error) {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return nil, ErrTransportClosed
	}
	t.mu.RUnlock()

	t.network.mu.RLock()
	peer, ok := t.network.transports[peerID]
	t.network.mu.RUnlock()

	if !ok {
		return nil, ErrConnectFailed
	}

	peer.mu.RLock()
	handler := peer.handler
	closed := peer.closed
	peer.mu.RUnlock()

	if closed || handler == nil {
		return nil, ErrConnectFailed
	}

	return handler(msgType, data), nil
}

// Listen starts listening for RPCs.
func (t *InMemoryTransport) Listen(handler RPCHandler) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handler = handler
	return nil
}

// Close shuts down the transport.
func (t *InMemoryTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	t.handler = nil
	return nil
}

// LocalAddr returns the local address.
func (t *InMemoryTransport) LocalAddr() string {
	return t.addr
}
