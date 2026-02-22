package raft

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/config"
	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// ClusterBackend wraps a storage engine with Raft consensus for cluster mode.
// Write operations are routed through Raft for replication.
// Read operations can be served locally or from leader based on consistency requirements.
type ClusterBackend struct {
	engine       storage.StorageEngine
	logEngine    storage.StorageEngine
	stateMachine *ObaDBStateMachine
	node         *Node
	config       *config.ClusterConfig
	transport    Transport
	snapStore    *SnapshotStore
	logger       Logger

	// Config and ACL appliers for replication
	configApplier ConfigApplier
	aclApplier    ACLApplier

	// Callbacks
	onLeaderChange func(isLeader bool)

	mu sync.RWMutex
}

// ClusterBackendConfig holds configuration for ClusterBackend.
type ClusterBackendConfig struct {
	Engine         storage.StorageEngine
	ClusterConfig  *config.ClusterConfig
	OnLeaderChange func(isLeader bool)
}

// NewClusterBackend creates a new cluster-aware backend.
func NewClusterBackend(cfg *ClusterBackendConfig) (*ClusterBackend, error) {
	if cfg.Engine == nil {
		return nil, errors.New("storage engine required")
	}
	if cfg.ClusterConfig == nil || !cfg.ClusterConfig.Enabled {
		return nil, errors.New("cluster config required and must be enabled")
	}

	cc := cfg.ClusterConfig

	// Create peer address map
	peerAddrs := make(map[uint64]string)
	for _, p := range cc.Peers {
		if p.ID != cc.NodeID {
			peerAddrs[p.ID] = p.Addr
		}
	}

	// Create transport
	transport := NewTCPTransport(cc.RaftAddr, peerAddrs)

	// Create snapshot store
	snapStore, err := NewSnapshotStore(cc.DataDir)
	if err != nil {
		return nil, err
	}

	// Create state machine
	stateMachine := NewObaDBStateMachine(cfg.Engine)

	// Create Raft node config
	peers := make([]*Peer, len(cc.Peers))
	for i, p := range cc.Peers {
		peers[i] = &Peer{ID: p.ID, Addr: p.Addr}
	}

	electionTimeout := cc.ElectionTimeout
	if electionTimeout == 0 {
		electionTimeout = 150 * time.Millisecond
	}

	heartbeatTimeout := cc.HeartbeatTimeout
	if heartbeatTimeout == 0 {
		heartbeatTimeout = 50 * time.Millisecond
	}

	nodeCfg := &NodeConfig{
		ID:               cc.NodeID,
		Addr:             cc.RaftAddr,
		Peers:            peers,
		ElectionTimeout:  electionTimeout,
		HeartbeatTimeout: heartbeatTimeout,
		DataDir:          cc.DataDir,
	}

	// Create Raft node
	node, err := NewNode(nodeCfg, stateMachine, transport)
	if err != nil {
		return nil, err
	}

	cb := &ClusterBackend{
		engine:         cfg.Engine,
		stateMachine:   stateMachine,
		node:           node,
		config:         cc,
		transport:      transport,
		snapStore:      snapStore,
		logger:         &defaultLogger{},
		onLeaderChange: cfg.OnLeaderChange,
	}

	return cb, nil
}

// SetLogger sets the logger for the cluster backend.
func (cb *ClusterBackend) SetLogger(logger Logger) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.logger = logger
	cb.node.SetLogger(logger)
}

// SetLogEngine sets the log database engine for multi-database replication.
func (cb *ClusterBackend) SetLogEngine(engine storage.StorageEngine) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.logEngine = engine
	cb.stateMachine.SetLogEngine(engine)
}

// SetConfigApplier sets the config applier for config replication.
func (cb *ClusterBackend) SetConfigApplier(applier ConfigApplier) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.configApplier = applier
	cb.stateMachine.SetConfigApplier(applier)
}

// SetACLApplier sets the ACL applier for ACL replication.
func (cb *ClusterBackend) SetACLApplier(applier ACLApplier) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.aclApplier = applier
	cb.stateMachine.SetACLApplier(applier)
}

// Start starts the cluster backend and Raft node.
// On startup, clears the main engine and replays all Raft logs.
func (cb *ClusterBackend) Start() error {
	// Clear main engine before replaying logs
	// This ensures the state machine is rebuilt from the Raft log
	if err := cb.stateMachine.ClearMainEngine(); err != nil {
		return fmt.Errorf("failed to clear main engine: %w", err)
	}
	return cb.node.Start()
}

// Stop stops the cluster backend and Raft node.
func (cb *ClusterBackend) Stop() {
	cb.node.Stop()
	cb.transport.Close()
}

// IsLeader returns true if this node is the cluster leader.
func (cb *ClusterBackend) IsLeader() bool {
	return cb.node.IsLeader()
}

// LeaderID returns the current leader's node ID.
func (cb *ClusterBackend) LeaderID() uint64 {
	return cb.node.LeaderID()
}

// LeaderAddr returns the current leader's address.
func (cb *ClusterBackend) LeaderAddr() string {
	leaderID := cb.node.LeaderID()
	if leaderID == 0 {
		return ""
	}

	for _, p := range cb.config.Peers {
		if p.ID == leaderID {
			return p.Addr
		}
	}
	return ""
}

// NodeID returns this node's ID.
func (cb *ClusterBackend) NodeID() uint64 {
	return cb.node.ID()
}

// Term returns the current Raft term.
func (cb *ClusterBackend) Term() uint64 {
	return cb.node.Term()
}

// State returns the current node state as string.
func (cb *ClusterBackend) State() string {
	return StateString(cb.node.State())
}

// CommitIndex returns the current commit index.
func (cb *ClusterBackend) CommitIndex() uint64 {
	return cb.node.CommitIndex()
}

// Put stores an entry through Raft consensus.
// Only the leader can accept writes.
func (cb *ClusterBackend) Put(entry *storage.Entry) error {
	if !cb.IsLeader() {
		return ErrNotLeader
	}

	// Check uid uniqueness before proposing to Raft
	if err := cb.checkUIDUnique(entry); err != nil {
		return err
	}

	cmd := CreatePutCommand(entry)
	return cb.node.Propose(cmd)
}

// Delete removes an entry through Raft consensus.
// Only the leader can accept writes.
func (cb *ClusterBackend) Delete(dn string) error {
	if !cb.IsLeader() {
		return ErrNotLeader
	}

	cmd := CreateDeleteCommand(dn)
	return cb.node.Propose(cmd)
}

// ModifyDN renames an entry through Raft consensus.
// Only the leader can accept writes.
func (cb *ClusterBackend) ModifyDN(oldDN string, newEntry *storage.Entry) error {
	if !cb.IsLeader() {
		return ErrNotLeader
	}

	cmd := CreateModifyDNCommand(oldDN, newEntry)
	return cb.node.Propose(cmd)
}

// PutLog stores a log entry through Raft consensus.
// Only the leader can accept writes.
func (cb *ClusterBackend) PutLog(entry *storage.Entry) error {
	if !cb.IsLeader() {
		return ErrNotLeader
	}

	cmd := CreatePutCommandForDB(entry, DBLog)
	return cb.node.Propose(cmd)
}

// DeleteLog removes a log entry through Raft consensus.
// Only the leader can accept writes.
func (cb *ClusterBackend) DeleteLog(dn string) error {
	if !cb.IsLeader() {
		return ErrNotLeader
	}

	cmd := CreateDeleteCommandForDB(dn, DBLog)
	return cb.node.Propose(cmd)
}

// Get retrieves an entry from local storage.
// Reads are served locally for performance.
func (cb *ClusterBackend) Get(dn string) (*storage.Entry, error) {
	tx, err := cb.engine.Begin()
	if err != nil {
		return nil, err
	}
	defer cb.engine.Rollback(tx)

	return cb.engine.Get(tx, dn)
}

// Search searches entries from local storage.
// Reads are served locally for performance.
func (cb *ClusterBackend) Search(baseDN string, scope storage.Scope) storage.Iterator {
	tx, err := cb.engine.Begin()
	if err != nil {
		return &emptyIterator{err: err}
	}
	// Note: Transaction will be held until iterator is closed
	return cb.engine.SearchByDN(tx, baseDN, scope)
}

// ClusterStatus returns the current cluster status.
type ClusterStatus struct {
	NodeID      uint64       `json:"nodeId"`
	State       string       `json:"state"`
	Term        uint64       `json:"term"`
	LeaderID    uint64       `json:"leaderId"`
	LeaderAddr  string       `json:"leaderAddr"`
	CommitIndex uint64       `json:"commitIndex"`
	LastApplied uint64       `json:"lastApplied"`
	Peers       []PeerStatus `json:"peers"`
}

// PeerStatus represents a peer's status.
type PeerStatus struct {
	ID   uint64 `json:"id"`
	Addr string `json:"addr"`
}

// Status returns the current cluster status.
func (cb *ClusterBackend) Status() *ClusterStatus {
	status := &ClusterStatus{
		NodeID:      cb.node.ID(),
		State:       StateString(cb.node.State()),
		Term:        cb.node.Term(),
		LeaderID:    cb.node.LeaderID(),
		LeaderAddr:  cb.LeaderAddr(),
		CommitIndex: cb.node.CommitIndex(),
		LastApplied: cb.node.LastApplied(),
		Peers:       make([]PeerStatus, 0, len(cb.config.Peers)),
	}

	for _, p := range cb.config.Peers {
		status.Peers = append(status.Peers, PeerStatus{
			ID:   p.ID,
			Addr: p.Addr,
		})
	}

	return status
}

// emptyIterator is returned when an error occurs before iteration.
type emptyIterator struct {
	err error
}

func (i *emptyIterator) Next() bool            { return false }
func (i *emptyIterator) Entry() *storage.Entry { return nil }
func (i *emptyIterator) Error() error          { return i.err }
func (i *emptyIterator) Close()                {}

// ProposeConfigChange proposes a config change through Raft consensus.
// Only the leader can accept config changes.
func (cb *ClusterBackend) ProposeConfigChange(section string, data map[string]string, version uint64) error {
	if !cb.IsLeader() {
		return ErrNotLeader
	}

	cmd, err := CreateConfigUpdateCommand(section, data, version)
	if err != nil {
		return err
	}

	return cb.node.Propose(cmd)
}

// ProposeACLFullUpdate proposes a full ACL update through Raft consensus.
// Only the leader can accept ACL changes.
func (cb *ClusterBackend) ProposeACLFullUpdate(rules []ACLRuleData, defaultPolicy string, version uint64) error {
	if !cb.IsLeader() {
		return ErrNotLeader
	}

	cmd, err := CreateACLFullUpdateCommand(rules, defaultPolicy, version)
	if err != nil {
		return err
	}

	return cb.node.Propose(cmd)
}

// ProposeACLAddRule proposes adding an ACL rule through Raft consensus.
// Only the leader can accept ACL changes.
func (cb *ClusterBackend) ProposeACLAddRule(rule *ACLRuleData, index int, version uint64) error {
	if !cb.IsLeader() {
		return ErrNotLeader
	}

	cmd, err := CreateACLAddRuleCommand(rule, index, version)
	if err != nil {
		return err
	}

	return cb.node.Propose(cmd)
}

// ProposeACLUpdateRule proposes updating an ACL rule through Raft consensus.
// Only the leader can accept ACL changes.
func (cb *ClusterBackend) ProposeACLUpdateRule(rule *ACLRuleData, index int, version uint64) error {
	if !cb.IsLeader() {
		return ErrNotLeader
	}

	cmd, err := CreateACLUpdateRuleCommand(rule, index, version)
	if err != nil {
		return err
	}

	return cb.node.Propose(cmd)
}

// ProposeACLDeleteRule proposes deleting an ACL rule through Raft consensus.
// Only the leader can accept ACL changes.
func (cb *ClusterBackend) ProposeACLDeleteRule(index int, version uint64) error {
	if !cb.IsLeader() {
		return ErrNotLeader
	}

	cmd, err := CreateACLDeleteRuleCommand(index, version)
	if err != nil {
		return err
	}

	return cb.node.Propose(cmd)
}

// ProposeACLSetDefault proposes setting the default ACL policy through Raft consensus.
// Only the leader can accept ACL changes.
func (cb *ClusterBackend) ProposeACLSetDefault(defaultPolicy string, version uint64) error {
	if !cb.IsLeader() {
		return ErrNotLeader
	}

	cmd, err := CreateACLSetDefaultCommand(defaultPolicy, version)
	if err != nil {
		return err
	}

	return cb.node.Propose(cmd)
}

// checkUIDUnique checks if the uid attribute value is unique across all entries.
// This is called before proposing a Put command to Raft.
func (cb *ClusterBackend) checkUIDUnique(entry *storage.Entry) error {
	if entry == nil {
		return nil
	}

	// Find uid attribute (case-insensitive)
	var uidValue string
	for name, values := range entry.Attributes {
		if strings.ToLower(name) == "uid" {
			if len(values) > 0 && len(values[0]) > 0 {
				uidValue = string(values[0])
				break
			}
		}
	}

	if uidValue == "" {
		return nil // No uid attribute, nothing to check
	}

	cb.logger.Info("checkUIDUnique: checking", "uid", uidValue, "dn", entry.DN)

	// Search for existing entries with the same uid using the storage engine
	tx, err := cb.engine.Begin()
	if err != nil {
		cb.logger.Error("checkUIDUnique: failed to begin tx", "error", err)
		return nil // Skip check on error
	}
	defer cb.engine.Rollback(tx)

	// Scan full subtree to reliably include all entries.
	iter := cb.engine.SearchByDN(tx, "", storage.ScopeSubtree)
	defer iter.Close()

	count := 0
	for iter.Next() {
		existing := iter.Entry()
		if existing == nil {
			continue
		}
		for name, values := range existing.Attributes {
			if strings.ToLower(name) != "uid" {
				continue
			}
			for _, v := range values {
				if string(v) == uidValue {
					count++
					cb.logger.Info("checkUIDUnique: found entry", "existingDN", existing.DN, "entryDN", entry.DN)
					if existing.DN != entry.DN {
						cb.logger.Info("checkUIDUnique: duplicate found", "existingDN", existing.DN)
						return ErrUIDNotUnique
					}
				}
			}
		}
	}
	cb.logger.Info("checkUIDUnique: scan complete", "count", count)

	return nil
}

// ErrUIDNotUnique is returned when trying to add an entry with a duplicate uid.
var ErrUIDNotUnique = errors.New("uid attribute value is not unique")
