package raft

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// ConfigApplier is the interface for applying config changes from Raft.
type ConfigApplier interface {
	ApplyFromRaft(section string, data map[string]string) error
	GetConfigSnapshot() ([]byte, error)
	RestoreConfigSnapshot(data []byte) error
}

// ACLApplier is the interface for applying ACL changes from Raft.
type ACLApplier interface {
	ApplyFullConfigFromRaft(rules []ACLRuleData, defaultPolicy string) error
	AddRuleFromRaft(rule *ACLRuleData, index int) error
	UpdateRuleFromRaft(rule *ACLRuleData, index int) error
	DeleteRuleFromRaft(index int) error
	SetDefaultPolicyFromRaft(policy string) error
	GetACLSnapshot() ([]byte, error)
	RestoreACLSnapshot(data []byte) error
}

// ObaDBStateMachine wraps ObaDB storage engine as a Raft state machine.
// Supports multiple databases (main LDAP and log database).
type ObaDBStateMachine struct {
	mainEngine    storage.StorageEngine
	logEngine     storage.StorageEngine
	configApplier ConfigApplier
	aclApplier    ACLApplier
	mu            sync.Mutex
}

// NewObaDBStateMachine creates a new state machine wrapping ObaDB.
func NewObaDBStateMachine(engine storage.StorageEngine) *ObaDBStateMachine {
	return &ObaDBStateMachine{mainEngine: engine}
}

// ClearMainEngine clears all entries from the main engine.
// This is called on startup before replaying the Raft log.
func (sm *ObaDBStateMachine) ClearMainEngine() error {
	// Skip clearing - let log replay handle everything
	// ClearIndexes was causing "index already exists" error on restart
	return nil
}

// SetLogEngine sets the log database engine for multi-database support.
func (sm *ObaDBStateMachine) SetLogEngine(engine storage.StorageEngine) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.logEngine = engine
}

// SetConfigApplier sets the config applier for config replication.
func (sm *ObaDBStateMachine) SetConfigApplier(applier ConfigApplier) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.configApplier = applier
}

// SetACLApplier sets the ACL applier for ACL replication.
func (sm *ObaDBStateMachine) SetACLApplier(applier ACLApplier) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.aclApplier = applier
}

// getEngine returns the appropriate engine for the given database ID.
func (sm *ObaDBStateMachine) getEngine(dbID uint8) storage.StorageEngine {
	switch dbID {
	case DBLog:
		if sm.logEngine != nil {
			return sm.logEngine
		}
		// Don't fallback to main engine for log entries
		return nil
	default:
		return sm.mainEngine
	}
}

// Apply applies a command to ObaDB.
func (sm *ObaDBStateMachine) Apply(cmd *Command) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	switch cmd.Type {
	case CmdPut, CmdDelete, CmdModifyDN:
		return sm.applyLDAPCommand(cmd)
	case CmdConfigUpdate:
		return sm.applyConfigCommand(cmd)
	case CmdACLFullUpdate, CmdACLAddRule, CmdACLUpdateRule, CmdACLDeleteRule, CmdACLSetDefault:
		return sm.applyACLCommand(cmd)
	default:
		return fmt.Errorf("unknown command type: %d", cmd.Type)
	}
}

// applyLDAPCommand applies LDAP operations (Put, Delete, ModifyDN).
func (sm *ObaDBStateMachine) applyLDAPCommand(cmd *Command) error {
	engine := sm.getEngine(cmd.DatabaseID)
	if engine == nil {
		// Skip if engine not available (e.g., log engine not set yet)
		return nil
	}
	tx, err := engine.Begin()
	if err != nil {
		return err
	}

	var applyErr error
	switch cmd.Type {
	case CmdPut:
		entry, err := deserializeEntry(cmd.EntryData)
		if err != nil {
			engine.Rollback(tx)
			return err
		}
		applyErr = engine.Put(tx, entry)

	case CmdDelete:
		applyErr = engine.Delete(tx, cmd.DN)
		// Ignore "entry not found" error during log replay
		// Entry may have been deleted by ClearMainEngine
		if applyErr != nil && applyErr.Error() == "entry not found" {
			applyErr = nil
		}

	case CmdModifyDN:
		// ModifyDN = Delete old + Put new
		// Normalize oldDN to match storage format (lowercase)
		oldDN := strings.ToLower(strings.TrimSpace(cmd.OldDN))
		engine.Delete(tx, oldDN) // Ignore error - old entry may not exist
		entry, err := deserializeEntry(cmd.EntryData)
		if err != nil {
			engine.Rollback(tx)
			return err
		}
		applyErr = engine.Put(tx, entry)
	}

	if applyErr != nil {
		engine.Rollback(tx)
		return applyErr
	}

	return engine.Commit(tx)
}

// applyConfigCommand applies config update commands.
func (sm *ObaDBStateMachine) applyConfigCommand(cmd *Command) error {
	if sm.configApplier == nil {
		return nil // Config applier not set, skip
	}

	configCmd, err := DeserializeConfigCommand(cmd.ConfigData)
	if err != nil {
		return fmt.Errorf("failed to deserialize config command: %w", err)
	}

	return sm.configApplier.ApplyFromRaft(configCmd.Section, configCmd.Data)
}

// applyACLCommand applies ACL update commands.
func (sm *ObaDBStateMachine) applyACLCommand(cmd *Command) error {
	if sm.aclApplier == nil {
		return nil // ACL applier not set, skip
	}

	aclCmd, err := DeserializeACLCommand(cmd.ACLData)
	if err != nil {
		return fmt.Errorf("failed to deserialize ACL command: %w", err)
	}

	switch cmd.Type {
	case CmdACLFullUpdate:
		return sm.aclApplier.ApplyFullConfigFromRaft(aclCmd.Rules, aclCmd.DefaultPolicy)
	case CmdACLAddRule:
		return sm.aclApplier.AddRuleFromRaft(aclCmd.Rule, aclCmd.RuleIndex)
	case CmdACLUpdateRule:
		return sm.aclApplier.UpdateRuleFromRaft(aclCmd.Rule, aclCmd.RuleIndex)
	case CmdACLDeleteRule:
		return sm.aclApplier.DeleteRuleFromRaft(aclCmd.RuleIndex)
	case CmdACLSetDefault:
		return sm.aclApplier.SetDefaultPolicyFromRaft(aclCmd.DefaultPolicy)
	}

	return nil
}

// Snapshot creates a snapshot of the current state.
// This serializes all entries from both databases, plus config and ACL.
func (sm *ObaDBStateMachine) Snapshot() ([]byte, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.mainEngine == nil {
		return nil, fmt.Errorf("main engine is nil")
	}

	var buf bytes.Buffer

	// Snapshot version (for backward compatibility)
	binary.Write(&buf, binary.LittleEndian, uint8(2)) // Version 2 includes config/ACL

	// Snapshot main database
	mainData, err := sm.snapshotEngine(sm.mainEngine)
	if err != nil {
		return nil, err
	}
	binary.Write(&buf, binary.LittleEndian, uint32(len(mainData)))
	buf.Write(mainData)

	// Snapshot log database if available
	if sm.logEngine != nil {
		logData, err := sm.snapshotEngine(sm.logEngine)
		if err != nil {
			return nil, err
		}
		binary.Write(&buf, binary.LittleEndian, uint32(len(logData)))
		buf.Write(logData)
	} else {
		binary.Write(&buf, binary.LittleEndian, uint32(0))
	}

	// Snapshot config if available
	if sm.configApplier != nil {
		configData, err := sm.configApplier.GetConfigSnapshot()
		if err != nil {
			return nil, err
		}
		binary.Write(&buf, binary.LittleEndian, uint32(len(configData)))
		buf.Write(configData)
	} else {
		binary.Write(&buf, binary.LittleEndian, uint32(0))
	}

	// Snapshot ACL if available
	if sm.aclApplier != nil {
		aclData, err := sm.aclApplier.GetACLSnapshot()
		if err != nil {
			return nil, err
		}
		binary.Write(&buf, binary.LittleEndian, uint32(len(aclData)))
		buf.Write(aclData)
	} else {
		binary.Write(&buf, binary.LittleEndian, uint32(0))
	}

	return buf.Bytes(), nil
}

// snapshotEngine creates a snapshot of a single engine.
func (sm *ObaDBStateMachine) snapshotEngine(engine storage.StorageEngine) ([]byte, error) {
	tx, err := engine.Begin()
	if err != nil {
		return nil, err
	}
	defer engine.Rollback(tx)

	var buf bytes.Buffer

	// Iterate all entries from root
	iter := engine.SearchByDN(tx, "", storage.ScopeSubtree)
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

	// Check snapshot version
	var version uint8
	if err := binary.Read(reader, binary.LittleEndian, &version); err != nil {
		// Old format without version, reset reader and use version 1
		reader = bytes.NewReader(data)
		version = 1
	}

	// Restore main database
	var mainLen uint32
	if err := binary.Read(reader, binary.LittleEndian, &mainLen); err != nil {
		return err
	}
	if mainLen > 0 {
		mainData := make([]byte, mainLen)
		if _, err := io.ReadFull(reader, mainData); err != nil {
			return err
		}
		if err := sm.restoreEngine(sm.mainEngine, mainData); err != nil {
			return err
		}
	}

	// Restore log database if available
	var logLen uint32
	if err := binary.Read(reader, binary.LittleEndian, &logLen); err != nil {
		// Old snapshot format without log database
		return nil
	}
	if logLen > 0 && sm.logEngine != nil {
		logData := make([]byte, logLen)
		if _, err := io.ReadFull(reader, logData); err != nil {
			return err
		}
		if err := sm.restoreEngine(sm.logEngine, logData); err != nil {
			return err
		}
	}

	// Version 2+ includes config and ACL
	if version >= 2 {
		// Restore config if available
		var configLen uint32
		if err := binary.Read(reader, binary.LittleEndian, &configLen); err != nil {
			return nil // No config data, ok
		}
		if configLen > 0 && sm.configApplier != nil {
			configData := make([]byte, configLen)
			if _, err := io.ReadFull(reader, configData); err != nil {
				return err
			}
			if err := sm.configApplier.RestoreConfigSnapshot(configData); err != nil {
				return err
			}
		}

		// Restore ACL if available
		var aclLen uint32
		if err := binary.Read(reader, binary.LittleEndian, &aclLen); err != nil {
			return nil // No ACL data, ok
		}
		if aclLen > 0 && sm.aclApplier != nil {
			aclData := make([]byte, aclLen)
			if _, err := io.ReadFull(reader, aclData); err != nil {
				return err
			}
			if err := sm.aclApplier.RestoreACLSnapshot(aclData); err != nil {
				return err
			}
		}
	}

	return nil
}

// restoreEngine restores a single engine from snapshot data.
func (sm *ObaDBStateMachine) restoreEngine(engine storage.StorageEngine, data []byte) error {
	reader := bytes.NewReader(data)

	// Read entry count
	var count uint32
	if err := binary.Read(reader, binary.LittleEndian, &count); err != nil {
		return err
	}

	// First, delete all existing entries
	tx, err := engine.Begin()
	if err != nil {
		return err
	}

	// Get all existing DNs
	iter := engine.SearchByDN(tx, "", storage.ScopeSubtree)
	var existingDNs []string
	for iter.Next() {
		if entry := iter.Entry(); entry != nil {
			existingDNs = append(existingDNs, entry.DN)
		}
	}
	iter.Close()
	engine.Rollback(tx)

	// Delete existing entries (in reverse order to handle children first)
	for i := len(existingDNs) - 1; i >= 0; i-- {
		tx, err := engine.Begin()
		if err != nil {
			continue
		}
		engine.Delete(tx, existingDNs[i])
		engine.Commit(tx)
	}

	// Read and apply each entry from snapshot
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

		tx, err := engine.Begin()
		if err != nil {
			return err
		}

		if err := engine.Put(tx, entry); err != nil {
			engine.Rollback(tx)
			return err
		}

		if err := engine.Commit(tx); err != nil {
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
	return CreatePutCommandForDB(entry, DBMain)
}

// CreatePutCommandForDB creates a Raft command for a Put operation on a specific database.
func CreatePutCommandForDB(entry *storage.Entry, dbID uint8) *Command {
	return &Command{
		Type:       CmdPut,
		DatabaseID: dbID,
		DN:         entry.DN,
		EntryDN:    entry.DN,
		EntryData:  serializeEntry(entry),
	}
}

// CreateDeleteCommand creates a Raft command for a Delete operation.
func CreateDeleteCommand(dn string) *Command {
	return CreateDeleteCommandForDB(dn, DBMain)
}

// CreateDeleteCommandForDB creates a Raft command for a Delete operation on a specific database.
func CreateDeleteCommandForDB(dn string, dbID uint8) *Command {
	return &Command{
		Type:       CmdDelete,
		DatabaseID: dbID,
		DN:         dn,
	}
}

// CreateModifyDNCommand creates a Raft command for a ModifyDN operation.
func CreateModifyDNCommand(oldDN string, newEntry *storage.Entry) *Command {
	return CreateModifyDNCommandForDB(oldDN, newEntry, DBMain)
}

// CreateModifyDNCommandForDB creates a Raft command for a ModifyDN operation on a specific database.
func CreateModifyDNCommandForDB(oldDN string, newEntry *storage.Entry, dbID uint8) *Command {
	return &Command{
		Type:       CmdModifyDN,
		DatabaseID: dbID,
		DN:         newEntry.DN,
		OldDN:      oldDN,
		EntryDN:    newEntry.DN,
		EntryData:  serializeEntry(newEntry),
	}
}
