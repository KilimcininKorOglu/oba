package raft

import (
	"bytes"
	"encoding/binary"
	"io"
	"sync"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// ObaDBStateMachine wraps ObaDB storage engine as a Raft state machine.
type ObaDBStateMachine struct {
	engine storage.StorageEngine
	mu     sync.Mutex
}

// NewObaDBStateMachine creates a new state machine wrapping ObaDB.
func NewObaDBStateMachine(engine storage.StorageEngine) *ObaDBStateMachine {
	return &ObaDBStateMachine{engine: engine}
}

// Apply applies a command to ObaDB.
func (sm *ObaDBStateMachine) Apply(cmd *Command) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	tx, err := sm.engine.Begin()
	if err != nil {
		return err
	}

	var applyErr error
	switch cmd.Type {
	case CmdPut:
		entry, err := deserializeEntry(cmd.EntryData)
		if err != nil {
			sm.engine.Rollback(tx)
			return err
		}
		applyErr = sm.engine.Put(tx, entry)

	case CmdDelete:
		applyErr = sm.engine.Delete(tx, cmd.DN)

	case CmdModifyDN:
		// ModifyDN = Delete old + Put new
		if err := sm.engine.Delete(tx, cmd.OldDN); err != nil {
			sm.engine.Rollback(tx)
			return err
		}
		entry, err := deserializeEntry(cmd.EntryData)
		if err != nil {
			sm.engine.Rollback(tx)
			return err
		}
		applyErr = sm.engine.Put(tx, entry)
	}

	if applyErr != nil {
		sm.engine.Rollback(tx)
		return applyErr
	}

	return sm.engine.Commit(tx)
}

// Snapshot creates a snapshot of the current state.
// This serializes all entries in the database.
func (sm *ObaDBStateMachine) Snapshot() ([]byte, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	tx, err := sm.engine.Begin()
	if err != nil {
		return nil, err
	}
	defer sm.engine.Rollback(tx)

	var buf bytes.Buffer

	// Iterate all entries from root
	iter := sm.engine.SearchByDN(tx, "", storage.ScopeSubtree)
	defer iter.Close()

	var entries []*storage.Entry
	for iter.Next() {
		entries = append(entries, iter.Entry().Clone())
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}

	// Write entry count
	binary.Write(&buf, binary.LittleEndian, uint32(len(entries)))

	// Write each entry
	for _, entry := range entries {
		data := serializeEntry(entry)
		binary.Write(&buf, binary.LittleEndian, uint32(len(data)))
		buf.Write(data)
	}

	return buf.Bytes(), nil
}

// Restore restores state from a snapshot.
func (sm *ObaDBStateMachine) Restore(data []byte) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(data) == 0 {
		return nil
	}

	reader := bytes.NewReader(data)

	// Read entry count
	var count uint32
	if err := binary.Read(reader, binary.LittleEndian, &count); err != nil {
		return err
	}

	// Read and apply each entry
	for i := uint32(0); i < count; i++ {
		var entryLen uint32
		if err := binary.Read(reader, binary.LittleEndian, &entryLen); err != nil {
			return err
		}

		entryData := make([]byte, entryLen)
		if _, err := io.ReadFull(reader, entryData); err != nil {
			return err
		}

		entry, err := deserializeEntry(entryData)
		if err != nil {
			return err
		}

		tx, err := sm.engine.Begin()
		if err != nil {
			return err
		}

		if err := sm.engine.Put(tx, entry); err != nil {
			sm.engine.Rollback(tx)
			return err
		}

		if err := sm.engine.Commit(tx); err != nil {
			return err
		}
	}

	return nil
}

// serializeEntry encodes an entry to bytes.
func serializeEntry(entry *storage.Entry) []byte {
	var buf bytes.Buffer

	// DN
	dnBytes := []byte(entry.DN)
	binary.Write(&buf, binary.LittleEndian, uint16(len(dnBytes)))
	buf.Write(dnBytes)

	// Attribute count
	binary.Write(&buf, binary.LittleEndian, uint16(len(entry.Attributes)))

	// Attributes
	for name, values := range entry.Attributes {
		// Attribute name
		nameBytes := []byte(name)
		binary.Write(&buf, binary.LittleEndian, uint16(len(nameBytes)))
		buf.Write(nameBytes)

		// Value count
		binary.Write(&buf, binary.LittleEndian, uint16(len(values)))

		// Values
		for _, value := range values {
			binary.Write(&buf, binary.LittleEndian, uint32(len(value)))
			buf.Write(value)
		}
	}

	return buf.Bytes()
}

// deserializeEntry decodes an entry from bytes.
func deserializeEntry(data []byte) (*storage.Entry, error) {
	if len(data) == 0 {
		return nil, ErrLogCorrupted
	}

	reader := bytes.NewReader(data)

	// DN
	var dnLen uint16
	if err := binary.Read(reader, binary.LittleEndian, &dnLen); err != nil {
		return nil, ErrLogCorrupted
	}
	dnBytes := make([]byte, dnLen)
	if _, err := io.ReadFull(reader, dnBytes); err != nil {
		return nil, ErrLogCorrupted
	}

	entry := storage.NewEntry(string(dnBytes))

	// Attribute count
	var attrCount uint16
	if err := binary.Read(reader, binary.LittleEndian, &attrCount); err != nil {
		return nil, ErrLogCorrupted
	}

	// Attributes
	for i := uint16(0); i < attrCount; i++ {
		// Attribute name
		var nameLen uint16
		if err := binary.Read(reader, binary.LittleEndian, &nameLen); err != nil {
			return nil, ErrLogCorrupted
		}
		nameBytes := make([]byte, nameLen)
		if _, err := io.ReadFull(reader, nameBytes); err != nil {
			return nil, ErrLogCorrupted
		}
		name := string(nameBytes)

		// Value count
		var valueCount uint16
		if err := binary.Read(reader, binary.LittleEndian, &valueCount); err != nil {
			return nil, ErrLogCorrupted
		}

		// Values
		values := make([][]byte, valueCount)
		for j := uint16(0); j < valueCount; j++ {
			var valueLen uint32
			if err := binary.Read(reader, binary.LittleEndian, &valueLen); err != nil {
				return nil, ErrLogCorrupted
			}
			value := make([]byte, valueLen)
			if _, err := io.ReadFull(reader, value); err != nil {
				return nil, ErrLogCorrupted
			}
			values[j] = value
		}

		entry.Attributes[name] = values
	}

	return entry, nil
}

// CreateCommand creates a Raft command for a Put operation.
func CreatePutCommand(entry *storage.Entry) *Command {
	return &Command{
		Type:      CmdPut,
		DN:        entry.DN,
		EntryDN:   entry.DN,
		EntryData: serializeEntry(entry),
	}
}

// CreateDeleteCommand creates a Raft command for a Delete operation.
func CreateDeleteCommand(dn string) *Command {
	return &Command{
		Type: CmdDelete,
		DN:   dn,
	}
}

// CreateModifyDNCommand creates a Raft command for a ModifyDN operation.
func CreateModifyDNCommand(oldDN string, newEntry *storage.Entry) *Command {
	return &Command{
		Type:      CmdModifyDN,
		DN:        newEntry.DN,
		OldDN:     oldDN,
		EntryDN:   newEntry.DN,
		EntryData: serializeEntry(newEntry),
	}
}
