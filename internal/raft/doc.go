// Package raft implements the Raft consensus algorithm for distributed consensus.
//
// Raft is a consensus algorithm designed to be easy to understand. It provides
// the same fault-tolerance and performance as Paxos but with a simpler structure.
//
// # Overview
//
// This package provides a complete Raft implementation with:
//   - Leader election with randomized timeouts
//   - Log replication with consistency guarantees
//   - Membership changes (cluster reconfiguration)
//   - Snapshot and log compaction
//   - TCP-based RPC transport
//
// # Architecture
//
// A Raft cluster consists of multiple nodes, where:
//   - One node is elected as the leader
//   - Other nodes are followers
//   - Leader handles all client requests
//   - Leader replicates log entries to followers
//   - Entries are committed when replicated to majority
//
// # Usage
//
// Create a new Raft node:
//
//	cfg := &raft.NodeConfig{
//	    ID:               1,
//	    Addr:             "localhost:4445",
//	    Peers:            peers,
//	    ElectionTimeout:  150 * time.Millisecond,
//	    HeartbeatTimeout: 50 * time.Millisecond,
//	    DataDir:          "/var/lib/oba/raft",
//	}
//
//	transport := raft.NewTCPTransport(cfg.Addr, peerAddrs)
//	stateMachine := raft.NewObaDBStateMachine(db)
//	node := raft.NewNode(cfg, stateMachine, transport)
//
//	// Start the node
//	go node.Run()
//
//	// Propose a command (only on leader)
//	if node.IsLeader() {
//	    cmd := &raft.Command{Type: raft.CmdPut, DN: dn, Entry: entry}
//	    err := node.Propose(cmd)
//	}
//
// # Consistency Guarantees
//
// Raft provides linearizable consistency:
//   - All committed entries are durable
//   - Committed entries are never lost
//   - All nodes see the same order of committed entries
//
// # Failure Handling
//
// The cluster can tolerate (N-1)/2 failures for N nodes:
//   - 3 nodes: tolerates 1 failure
//   - 5 nodes: tolerates 2 failures
//   - 7 nodes: tolerates 3 failures
//
// # References
//
//   - Raft Paper: https://raft.github.io/raft.pdf
//   - Raft Visualization: https://raft.github.io/
package raft
