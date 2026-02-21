package raft

import (
	"bytes"
	"encoding/binary"
	"io"
)

// RPC message types.
const (
	RPCRequestVote uint8 = iota
	RPCRequestVoteReply
	RPCAppendEntries
	RPCAppendEntriesReply
	RPCInstallSnapshot
	RPCInstallSnapshotReply
)

// RequestVoteArgs is sent by candidates to gather votes.
type RequestVoteArgs struct {
	Term         uint64 // Candidate's term
	CandidateID  uint64 // Candidate requesting vote
	LastLogIndex uint64 // Index of candidate's last log entry
	LastLogTerm  uint64 // Term of candidate's last log entry
}

// Serialize encodes RequestVoteArgs to bytes.
func (r *RequestVoteArgs) Serialize() []byte {
	buf := make([]byte, 32)
	binary.LittleEndian.PutUint64(buf[0:8], r.Term)
	binary.LittleEndian.PutUint64(buf[8:16], r.CandidateID)
	binary.LittleEndian.PutUint64(buf[16:24], r.LastLogIndex)
	binary.LittleEndian.PutUint64(buf[24:32], r.LastLogTerm)
	return buf
}

// DeserializeRequestVoteArgs decodes RequestVoteArgs from bytes.
func DeserializeRequestVoteArgs(data []byte) (*RequestVoteArgs, error) {
	if len(data) < 32 {
		return nil, ErrLogCorrupted
	}
	return &RequestVoteArgs{
		Term:         binary.LittleEndian.Uint64(data[0:8]),
		CandidateID:  binary.LittleEndian.Uint64(data[8:16]),
		LastLogIndex: binary.LittleEndian.Uint64(data[16:24]),
		LastLogTerm:  binary.LittleEndian.Uint64(data[24:32]),
	}, nil
}

// RequestVoteReply is the response to RequestVote.
type RequestVoteReply struct {
	Term        uint64 // Current term, for candidate to update itself
	VoteGranted bool   // True if candidate received vote
}

// Serialize encodes RequestVoteReply to bytes.
func (r *RequestVoteReply) Serialize() []byte {
	buf := make([]byte, 9)
	binary.LittleEndian.PutUint64(buf[0:8], r.Term)
	if r.VoteGranted {
		buf[8] = 1
	}
	return buf
}

// DeserializeRequestVoteReply decodes RequestVoteReply from bytes.
func DeserializeRequestVoteReply(data []byte) (*RequestVoteReply, error) {
	if len(data) < 9 {
		return nil, ErrLogCorrupted
	}
	return &RequestVoteReply{
		Term:        binary.LittleEndian.Uint64(data[0:8]),
		VoteGranted: data[8] == 1,
	}, nil
}

// AppendEntriesArgs is sent by leader to replicate log entries.
type AppendEntriesArgs struct {
	Term         uint64      // Leader's term
	LeaderID     uint64      // So follower can redirect clients
	PrevLogIndex uint64      // Index of log entry immediately preceding new ones
	PrevLogTerm  uint64      // Term of prevLogIndex entry
	Entries      []*LogEntry // Log entries to store (empty for heartbeat)
	LeaderCommit uint64      // Leader's commitIndex
}

// Serialize encodes AppendEntriesArgs to bytes.
func (a *AppendEntriesArgs) Serialize() []byte {
	var buf bytes.Buffer

	// Fixed fields
	header := make([]byte, 48)
	binary.LittleEndian.PutUint64(header[0:8], a.Term)
	binary.LittleEndian.PutUint64(header[8:16], a.LeaderID)
	binary.LittleEndian.PutUint64(header[16:24], a.PrevLogIndex)
	binary.LittleEndian.PutUint64(header[24:32], a.PrevLogTerm)
	binary.LittleEndian.PutUint64(header[32:40], uint64(len(a.Entries)))
	binary.LittleEndian.PutUint64(header[40:48], a.LeaderCommit)
	buf.Write(header)

	// Entries
	for _, entry := range a.Entries {
		entryData := entry.Serialize()
		binary.Write(&buf, binary.LittleEndian, uint32(len(entryData)))
		buf.Write(entryData)
	}

	return buf.Bytes()
}

// DeserializeAppendEntriesArgs decodes AppendEntriesArgs from bytes.
func DeserializeAppendEntriesArgs(data []byte) (*AppendEntriesArgs, error) {
	if len(data) < 48 {
		return nil, ErrLogCorrupted
	}

	args := &AppendEntriesArgs{
		Term:         binary.LittleEndian.Uint64(data[0:8]),
		LeaderID:     binary.LittleEndian.Uint64(data[8:16]),
		PrevLogIndex: binary.LittleEndian.Uint64(data[16:24]),
		PrevLogTerm:  binary.LittleEndian.Uint64(data[24:32]),
		LeaderCommit: binary.LittleEndian.Uint64(data[40:48]),
	}

	numEntries := binary.LittleEndian.Uint64(data[32:40])
	args.Entries = make([]*LogEntry, 0, numEntries)

	reader := bytes.NewReader(data[48:])
	for i := uint64(0); i < numEntries; i++ {
		var entryLen uint32
		if err := binary.Read(reader, binary.LittleEndian, &entryLen); err != nil {
			return nil, ErrLogCorrupted
		}

		entryData := make([]byte, entryLen)
		if _, err := io.ReadFull(reader, entryData); err != nil {
			return nil, ErrLogCorrupted
		}

		entry, err := DeserializeLogEntry(entryData)
		if err != nil {
			return nil, err
		}
		args.Entries = append(args.Entries, entry)
	}

	return args, nil
}

// AppendEntriesReply is the response to AppendEntries.
type AppendEntriesReply struct {
	Term          uint64 // Current term, for leader to update itself
	Success       bool   // True if follower contained entry matching prevLogIndex/prevLogTerm
	ConflictTerm  uint64 // Term of conflicting entry (optimization)
	ConflictIndex uint64 // First index of conflicting term (optimization)
}

// Serialize encodes AppendEntriesReply to bytes.
func (r *AppendEntriesReply) Serialize() []byte {
	buf := make([]byte, 25)
	binary.LittleEndian.PutUint64(buf[0:8], r.Term)
	if r.Success {
		buf[8] = 1
	}
	binary.LittleEndian.PutUint64(buf[9:17], r.ConflictTerm)
	binary.LittleEndian.PutUint64(buf[17:25], r.ConflictIndex)
	return buf
}

// DeserializeAppendEntriesReply decodes AppendEntriesReply from bytes.
func DeserializeAppendEntriesReply(data []byte) (*AppendEntriesReply, error) {
	if len(data) < 25 {
		return nil, ErrLogCorrupted
	}
	return &AppendEntriesReply{
		Term:          binary.LittleEndian.Uint64(data[0:8]),
		Success:       data[8] == 1,
		ConflictTerm:  binary.LittleEndian.Uint64(data[9:17]),
		ConflictIndex: binary.LittleEndian.Uint64(data[17:25]),
	}, nil
}

// InstallSnapshotArgs is sent by leader to send snapshot to lagging follower.
type InstallSnapshotArgs struct {
	Term              uint64 // Leader's term
	LeaderID          uint64 // So follower can redirect clients
	LastIncludedIndex uint64 // Snapshot replaces all entries up through this index
	LastIncludedTerm  uint64 // Term of lastIncludedIndex
	Data              []byte // Raw snapshot data
}

// Serialize encodes InstallSnapshotArgs to bytes.
func (s *InstallSnapshotArgs) Serialize() []byte {
	size := 32 + 4 + len(s.Data)
	buf := make([]byte, size)

	binary.LittleEndian.PutUint64(buf[0:8], s.Term)
	binary.LittleEndian.PutUint64(buf[8:16], s.LeaderID)
	binary.LittleEndian.PutUint64(buf[16:24], s.LastIncludedIndex)
	binary.LittleEndian.PutUint64(buf[24:32], s.LastIncludedTerm)
	binary.LittleEndian.PutUint32(buf[32:36], uint32(len(s.Data)))
	copy(buf[36:], s.Data)

	return buf
}

// DeserializeInstallSnapshotArgs decodes InstallSnapshotArgs from bytes.
func DeserializeInstallSnapshotArgs(data []byte) (*InstallSnapshotArgs, error) {
	if len(data) < 36 {
		return nil, ErrLogCorrupted
	}

	dataLen := binary.LittleEndian.Uint32(data[32:36])
	if len(data) < 36+int(dataLen) {
		return nil, ErrLogCorrupted
	}

	return &InstallSnapshotArgs{
		Term:              binary.LittleEndian.Uint64(data[0:8]),
		LeaderID:          binary.LittleEndian.Uint64(data[8:16]),
		LastIncludedIndex: binary.LittleEndian.Uint64(data[16:24]),
		LastIncludedTerm:  binary.LittleEndian.Uint64(data[24:32]),
		Data:              data[36 : 36+dataLen],
	}, nil
}

// InstallSnapshotReply is the response to InstallSnapshot.
type InstallSnapshotReply struct {
	Term uint64 // Current term, for leader to update itself
}

// Serialize encodes InstallSnapshotReply to bytes.
func (r *InstallSnapshotReply) Serialize() []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf[0:8], r.Term)
	return buf
}

// DeserializeInstallSnapshotReply decodes InstallSnapshotReply from bytes.
func DeserializeInstallSnapshotReply(data []byte) (*InstallSnapshotReply, error) {
	if len(data) < 8 {
		return nil, ErrLogCorrupted
	}
	return &InstallSnapshotReply{
		Term: binary.LittleEndian.Uint64(data[0:8]),
	}, nil
}
