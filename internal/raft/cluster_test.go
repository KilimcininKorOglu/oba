package raft

import (
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/config"
	"github.com/KilimcininKorOglu/oba/internal/storage"
)

func TestClusterBackendCreation(t *testing.T) {
	engine := NewMockStorageEngine()

	cfg := &ClusterBackendConfig{
		Engine: engine,
		ClusterConfig: &config.ClusterConfig{
			Enabled:          true,
			NodeID:           1,
			RaftAddr:         "127.0.0.1:14500",
			ElectionTimeout:  50 * time.Millisecond,
			HeartbeatTimeout: 20 * time.Millisecond,
			DataDir:          t.TempDir(),
			Peers: []config.PeerConfig{
				{ID: 1, Addr: "127.0.0.1:14500"},
			},
		},
	}

	cb, err := NewClusterBackend(cfg)
	if err != nil {
		t.Fatalf("NewClusterBackend failed: %v", err)
	}
	defer cb.Stop()

	if cb.NodeID() != 1 {
		t.Errorf("NodeID should be 1")
	}
}

func TestClusterBackendSingleNode(t *testing.T) {
	engine := NewMockStorageEngine()

	cfg := &ClusterBackendConfig{
		Engine: engine,
		ClusterConfig: &config.ClusterConfig{
			Enabled:          true,
			NodeID:           1,
			RaftAddr:         "127.0.0.1:14501",
			ElectionTimeout:  50 * time.Millisecond,
			HeartbeatTimeout: 20 * time.Millisecond,
			DataDir:          t.TempDir(),
			Peers: []config.PeerConfig{
				{ID: 1, Addr: "127.0.0.1:14501"},
			},
		},
	}

	cb, err := NewClusterBackend(cfg)
	if err != nil {
		t.Fatalf("NewClusterBackend failed: %v", err)
	}
	defer cb.Stop()

	// Start the cluster
	if err := cb.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for leader election
	time.Sleep(200 * time.Millisecond)

	// Single node should become leader
	if !cb.IsLeader() {
		t.Error("Single node should become leader")
	}

	// Test Put
	entry := &storage.Entry{
		DN:         "cn=test,dc=example,dc=com",
		Attributes: make(map[string][][]byte),
	}
	entry.Attributes["cn"] = [][]byte{[]byte("test")}

	if err := cb.Put(entry); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Wait for apply (Raft needs time to replicate and apply)
	time.Sleep(300 * time.Millisecond)

	// Verify entry exists
	if !engine.HasEntry("cn=test,dc=example,dc=com") {
		t.Error("Entry should exist after Put")
	}
}

func TestClusterBackendNotLeader(t *testing.T) {
	engine := NewMockStorageEngine()

	cfg := &ClusterBackendConfig{
		Engine: engine,
		ClusterConfig: &config.ClusterConfig{
			Enabled:          true,
			NodeID:           1,
			RaftAddr:         "127.0.0.1:14502",
			ElectionTimeout:  1 * time.Hour, // Never timeout
			HeartbeatTimeout: 50 * time.Millisecond,
			DataDir:          t.TempDir(),
			Peers: []config.PeerConfig{
				{ID: 1, Addr: "127.0.0.1:14502"},
				{ID: 2, Addr: "127.0.0.1:14503"},
				{ID: 3, Addr: "127.0.0.1:14504"},
			},
		},
	}

	cb, err := NewClusterBackend(cfg)
	if err != nil {
		t.Fatalf("NewClusterBackend failed: %v", err)
	}
	defer cb.Stop()

	cb.Start()

	// Node is follower, Put should fail
	entry := &storage.Entry{DN: "cn=test"}
	err = cb.Put(entry)
	if err != ErrNotLeader {
		t.Errorf("Expected ErrNotLeader, got %v", err)
	}

	// Delete should also fail
	err = cb.Delete("cn=test")
	if err != ErrNotLeader {
		t.Errorf("Expected ErrNotLeader for Delete, got %v", err)
	}
}

func TestClusterBackendStatus(t *testing.T) {
	engine := NewMockStorageEngine()

	cfg := &ClusterBackendConfig{
		Engine: engine,
		ClusterConfig: &config.ClusterConfig{
			Enabled:          true,
			NodeID:           1,
			RaftAddr:         "127.0.0.1:14505",
			ElectionTimeout:  50 * time.Millisecond,
			HeartbeatTimeout: 20 * time.Millisecond,
			DataDir:          t.TempDir(),
			Peers: []config.PeerConfig{
				{ID: 1, Addr: "127.0.0.1:14505"},
				{ID: 2, Addr: "127.0.0.1:14506"},
			},
		},
	}

	cb, err := NewClusterBackend(cfg)
	if err != nil {
		t.Fatalf("NewClusterBackend failed: %v", err)
	}
	defer cb.Stop()

	status := cb.Status()

	if status.NodeID != 1 {
		t.Errorf("NodeID should be 1")
	}
	if len(status.Peers) != 2 {
		t.Errorf("Should have 2 peers")
	}
}

func TestClusterBackendGet(t *testing.T) {
	engine := NewMockStorageEngine()

	// Pre-populate engine
	entry := &storage.Entry{
		DN:         "cn=existing,dc=example,dc=com",
		Attributes: make(map[string][][]byte),
	}
	entry.Attributes["cn"] = [][]byte{[]byte("existing")}
	engine.entries[entry.DN] = entry

	cfg := &ClusterBackendConfig{
		Engine: engine,
		ClusterConfig: &config.ClusterConfig{
			Enabled:          true,
			NodeID:           1,
			RaftAddr:         "127.0.0.1:14507",
			ElectionTimeout:  50 * time.Millisecond,
			HeartbeatTimeout: 20 * time.Millisecond,
			DataDir:          t.TempDir(),
			Peers: []config.PeerConfig{
				{ID: 1, Addr: "127.0.0.1:14507"},
			},
		},
	}

	cb, err := NewClusterBackend(cfg)
	if err != nil {
		t.Fatalf("NewClusterBackend failed: %v", err)
	}
	defer cb.Stop()

	// Get should work without being leader (local read)
	retrieved, err := cb.Get("cn=existing,dc=example,dc=com")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved == nil {
		t.Error("Entry should be found")
	}
	if retrieved.DN != entry.DN {
		t.Errorf("DN mismatch")
	}
}

func TestClusterBackendDelete(t *testing.T) {
	engine := NewMockStorageEngine()

	// Pre-populate engine
	entry := &storage.Entry{
		DN:         "cn=todelete,dc=example,dc=com",
		Attributes: make(map[string][][]byte),
	}
	engine.entries[entry.DN] = entry

	cfg := &ClusterBackendConfig{
		Engine: engine,
		ClusterConfig: &config.ClusterConfig{
			Enabled:          true,
			NodeID:           1,
			RaftAddr:         "127.0.0.1:14508",
			ElectionTimeout:  50 * time.Millisecond,
			HeartbeatTimeout: 20 * time.Millisecond,
			DataDir:          t.TempDir(),
			Peers: []config.PeerConfig{
				{ID: 1, Addr: "127.0.0.1:14508"},
			},
		},
	}

	cb, err := NewClusterBackend(cfg)
	if err != nil {
		t.Fatalf("NewClusterBackend failed: %v", err)
	}
	defer cb.Stop()

	cb.Start()
	time.Sleep(200 * time.Millisecond) // Wait for leader

	if !cb.IsLeader() {
		t.Skip("Not leader, skipping delete test")
	}

	// Delete
	if err := cb.Delete("cn=todelete,dc=example,dc=com"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Wait for apply (Raft needs time to replicate and apply)
	time.Sleep(300 * time.Millisecond)

	// Verify deleted
	if engine.HasEntry("cn=todelete,dc=example,dc=com") {
		t.Error("Entry should be deleted")
	}
}

func TestClusterBackendInvalidConfig(t *testing.T) {
	// Nil engine
	_, err := NewClusterBackend(&ClusterBackendConfig{
		Engine: nil,
		ClusterConfig: &config.ClusterConfig{
			Enabled: true,
		},
	})
	if err == nil {
		t.Error("Should fail with nil engine")
	}

	// Disabled cluster
	_, err = NewClusterBackend(&ClusterBackendConfig{
		Engine: NewMockStorageEngine(),
		ClusterConfig: &config.ClusterConfig{
			Enabled: false,
		},
	})
	if err == nil {
		t.Error("Should fail with disabled cluster")
	}

	// Nil cluster config
	_, err = NewClusterBackend(&ClusterBackendConfig{
		Engine:        NewMockStorageEngine(),
		ClusterConfig: nil,
	})
	if err == nil {
		t.Error("Should fail with nil cluster config")
	}
}
