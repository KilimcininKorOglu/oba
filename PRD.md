# Product Requirements Document: Oba

## 1. Overview

Oba is a lightweight, zero-dependency LDAP (Lightweight Directory Access Protocol) server implementation written in pure Go. The server will handle directory services without relying on any external Go packages, using only the standard library.

## 2. Goals

- Implement a fully functional LDAP server using only Go standard library
- Support core LDAP operations (Bind, Search, Add, Delete, Modify)
- Provide a simple, embeddable solution for Go applications
- Maintain high performance with minimal memory footprint
- Ensure easy deployment as a single binary

## 3. Non-Goals

- Full LDAP v3 specification compliance (initial release)
- LDAP referrals support
- Replication/clustering
- GUI administration interface

## 4. Functional Requirements

### 4.1 Protocol Support

| Feature              | Priority | Description                                    |
|----------------------|----------|------------------------------------------------|
| LDAP v3 Protocol     | High     | ASN.1 BER encoding/decoding                    |
| TCP/TLS Listener     | High     | Accept connections on configurable port        |
| StartTLS             | Medium   | Upgrade plain connection to TLS                |

### 4.2 LDAP Operations

| Operation       | Priority | Description                                         |
|-----------------|----------|-----------------------------------------------------|
| Bind            | High     | Simple authentication (username/password)           |
| Unbind          | High     | Close connection gracefully                         |
| Search          | High     | Query entries with filters and scope                |
| Add             | High     | Create new directory entries                        |
| Delete          | High     | Remove directory entries                            |
| Modify          | High     | Update existing entry attributes                    |
| ModifyDN        | Medium   | Rename or move entries                              |
| Compare         | Low      | Compare attribute values                            |
| Abandon         | Low      | Cancel pending operations                           |

### 4.3 Search Capabilities

- Base, One-Level, and Subtree search scopes
- Filter support: equality, presence, substring, AND, OR, NOT
- Attribute selection
- Size and time limits
- Paging support (Simple Paged Results Control)

### 4.4 Authentication

- Anonymous bind
- Simple bind (DN + password)
- Optional: SASL PLAIN mechanism

## 5. Technical Requirements

### 5.1 Architecture

```
┌─────────────────────────────────────────────────┐
│                   LDAP Server                   │
├─────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────┐  │
│  │   Listener  │  │   Handler   │  │  Store  │  │
│  │  (TCP/TLS)  │──│  (Protocol) │──│ (Data)  │  │
│  └─────────────┘  └─────────────┘  └─────────┘  │
├─────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────┐    │
│  │         ASN.1 BER Codec (Custom)        │    │
│  └─────────────────────────────────────────┘    │
└─────────────────────────────────────────────────┘
```

### 5.2 Core Components

| Component      | Responsibility                                      |
|----------------|-----------------------------------------------------|
| BER Codec      | Encode/decode ASN.1 BER messages                    |
| Message Parser | Parse LDAP protocol messages                        |
| Request Router | Route requests to appropriate handlers              |
| ObaDB          | Custom storage engine (see 5.3)                     |
| Filter Engine  | Evaluate LDAP search filters                        |

### 5.3 Custom Storage Engine (ObaDB)

A purpose-built embedded database engine optimized for LDAP workloads.

#### 5.3.1 Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         ObaDB                               │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────┐    │
│  │                   Query Executor                    │    │
│  └─────────────────────────────────────────────────────┘    │
│  ┌─────────────┐  ┌─────────────┐  ┌───────────────────┐    │
│  │  DN Index   │  │  Attr Index │  │  Filter Optimizer │    │
│  │ (Radix Tree)│  │  (B+ Tree)  │  │                   │    │
│  └─────────────┘  └─────────────┘  └───────────────────┘    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                  Page Manager                       │    │
│  │         (Memory-Mapped File + Buffer Pool)         │    │
│  └─────────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              WAL (Write-Ahead Log)                  │    │
│  └─────────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                  File Storage                       │    │
│  │        (Data File + Index File + WAL File)         │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

#### 5.3.2 Data Structures

| Structure        | Purpose                                | Implementation          |
|------------------|----------------------------------------|-------------------------|
| Radix Tree       | DN hierarchy traversal                 | Compressed path nodes   |
| B+ Tree          | Attribute value indexing               | 4KB page-aligned nodes  |
| Slot Page        | Entry storage                          | Slotted page format     |
| Free List        | Space reclamation                      | Linked page list        |

#### 5.3.3 File Layout

```
data.oba (Main Data File)
┌────────────────────────────────────────┐
│ Header (4KB)                           │
│ - Magic number, version                │
│ - Page size, total pages               │
│ - Root page pointers                   │
├────────────────────────────────────────┤
│ Page 1: DN Radix Tree Root             │
├────────────────────────────────────────┤
│ Page 2-N: Data Pages (Slotted)         │
│ - Entry DN                             │
│ - Attributes (key-value pairs)         │
│ - Overflow pointer (if needed)         │
├────────────────────────────────────────┤
│ Page N+1: Free Page List               │
└────────────────────────────────────────┘

index.oba (Index File)
┌────────────────────────────────────────┐
│ Header (4KB)                           │
├────────────────────────────────────────┤
│ B+ Tree for each indexed attribute     │
│ - objectClass index                    │
│ - uid index                            │
│ - cn index                             │
│ - Custom attribute indexes             │
└────────────────────────────────────────┘

wal.oba (Write-Ahead Log)
┌────────────────────────────────────────┐
│ LSN | TxID | Operation | Data | CRC    │
└────────────────────────────────────────┘
```

#### 5.3.4 Core Features

| Feature              | Description                                              |
|----------------------|----------------------------------------------------------|
| MVCC                 | Multi-version concurrency control for read isolation     |
| Copy-on-Write        | Safe updates without in-place modification               |
| Crash Recovery       | WAL-based recovery with checkpointing                    |
| Memory-Mapped I/O    | Zero-copy reads via mmap                                 |
| Buffer Pool          | LRU cache for hot pages                                  |
| Compression          | Optional LZ4-style compression (custom implementation)   |
| Checksums            | CRC32 per page for data integrity                        |

#### 5.3.5 DN Radix Tree

Optimized for LDAP's hierarchical namespace:

```
Root
 └─ dc=example
     └─ dc=com
         ├─ ou=users
         │   ├─ uid=alice  → Page 42, Slot 3
         │   └─ uid=bob    → Page 42, Slot 7
         └─ ou=groups
             └─ cn=admins  → Page 58, Slot 1
```

- O(k) lookup where k = DN component count
- Efficient subtree iteration for scope=subtree searches
- Parent pointer for scope=onelevel searches

#### 5.3.6 Attribute Indexing

B+ Tree indexes for fast attribute-based searches:

```go
type Index struct {
    Attribute string      // e.g., "uid", "mail"
    Type      IndexType   // Equality, Presence, Substring
    Root      PageID      // B+ tree root page
}

type IndexType int
const (
    IndexEquality  IndexType = iota // (uid=alice)
    IndexPresence                   // (mail=*)
    IndexSubstring                  // (cn=*admin*)
)
```

#### 5.3.7 Transaction Support

```go
type Transaction struct {
    ID        uint64
    StartLSN  uint64
    State     TxState
    WriteSet  []PageID
    ReadSet   []PageID
}

// ACID guarantees
// - Atomicity: WAL-based rollback
// - Consistency: Schema validation
// - Isolation: MVCC snapshots
// - Durability: fsync on commit
```

#### 5.3.8 Storage Engine API

```go
type StorageEngine interface {
    // Transaction management
    Begin() (*Transaction, error)
    Commit(tx *Transaction) error
    Rollback(tx *Transaction) error

    // Entry operations
    Get(tx *Transaction, dn string) (*Entry, error)
    Put(tx *Transaction, entry *Entry) error
    Delete(tx *Transaction, dn string) error

    // Search operations
    SearchByDN(tx *Transaction, baseDN string, scope Scope) Iterator
    SearchByFilter(tx *Transaction, baseDN string, filter Filter) Iterator

    // Index management
    CreateIndex(attribute string, indexType IndexType) error
    DropIndex(attribute string) error

    // Maintenance
    Checkpoint() error
    Compact() error
    Stats() *EngineStats
}
```

### 5.4 Configuration

```go
type Config struct {
    Address     string        // Listen address (default: ":389")
    TLSAddress  string        // TLS listen address (default: ":636")
    TLSCert     string        // Path to TLS certificate
    TLSKey      string        // Path to TLS private key
    BaseDN      string        // Base distinguished name
    RootDN      string        // Admin DN
    RootPW      string        // Admin password
    MaxConns    int           // Maximum concurrent connections
    ReadTimeout time.Duration // Read timeout per operation
}
```

## 6. Non-Functional Requirements

### 6.1 Performance

| Metric                  | Target           |
|-------------------------|------------------|
| Concurrent connections  | 10,000+          |
| Search operations/sec   | 50,000+ (simple) |
| Memory usage (idle)     | < 10 MB          |
| Startup time            | < 100 ms         |

### 6.2 ObaDB Performance

| Metric                  | Target           |
|-------------------------|------------------|
| Point lookup (by DN)    | < 10 us          |
| Range scan              | 100,000+ rows/s  |
| Write throughput        | 10,000+ ops/s    |
| WAL fsync latency       | < 1 ms           |
| Recovery time           | < 1s per 100MB   |
| Max database size       | 1 TB+            |

### 6.3 Security

- TLS 1.2+ support
- Password hashing (SHA256, PBKDF2 via stdlib crypto packages)
- Access control lists (ACL) for entries
- Rate limiting for bind attempts

### 6.4 Reliability

- Graceful shutdown
- Connection draining
- Panic recovery per connection
- Structured logging

## 7. API Design

### 7.1 Server Interface

```go
type Server interface {
    Start() error
    Stop(ctx context.Context) error
    AddEntry(dn string, attrs map[string][]string) error
    DeleteEntry(dn string) error
}
```

### 7.2 Backend Interface

Backend interface wraps StorageEngine and provides LDAP-specific operations:

```go
type Backend interface {
    Bind(dn, password string) error
    Search(baseDN string, scope int, filter Filter) ([]*Entry, error)
    Add(entry *Entry) error
    Delete(dn string) error
    Modify(dn string, changes []Modification) error
}
```

Note: Backend uses StorageEngine (5.3.8) internally for data persistence while adding authentication and LDAP protocol logic.

## 8. Additional Specifications

### 8.1 Schema Validation

```go
type Schema struct {
    ObjectClasses  map[string]*ObjectClass
    AttributeTypes map[string]*AttributeType
}

type ObjectClass struct {
    Name       string
    Superior   string
    Kind       ObjectClassKind // Abstract, Structural, Auxiliary
    Must       []string        // Required attributes
    May        []string        // Optional attributes
}

type AttributeType struct {
    Name         string
    Syntax       string   // e.g., "1.3.6.1.4.1.1466.115.121.1.15" (DirectoryString)
    SingleValue  bool
    NoUserMod    bool     // Read-only (e.g., createTimestamp)
    Usage        AttrUsage
}
```

- Validate entries against schema on Add/Modify
- Support standard LDAP syntaxes (DirectoryString, DN, Integer, Boolean, etc.)
- Load schema from LDIF or embedded defaults

### 8.2 Access Control Lists (ACL)

```go
type ACL struct {
    Target     string      // DN pattern or "*"
    Scope      ACLScope    // Base, One, Subtree
    Subject    string      // Who: DN, "anonymous", "authenticated", "*"
    Rights     []ACLRight  // Read, Write, Add, Delete, Search, Compare
    Attributes []string    // Specific attrs or "*"
}

type ACLRight int
const (
    ACLRead ACLRight = 1 << iota
    ACLWrite
    ACLAdd
    ACLDelete
    ACLSearch
    ACLCompare
)
```

- Evaluate ACLs in order (first match wins)
- Default deny policy
- Support attribute-level permissions

### 8.3 Error Handling

| Error Type          | Strategy                                    |
|---------------------|---------------------------------------------|
| Protocol errors     | Return LDAP result code, log warning        |
| Storage errors      | Return LDAP_OPERATIONS_ERROR, log error     |
| Authentication fail | Return LDAP_INVALID_CREDENTIALS, rate limit |
| Schema violation    | Return LDAP_OBJECT_CLASS_VIOLATION          |
| ACL denied          | Return LDAP_INSUFFICIENT_ACCESS             |
| Internal panic      | Recover, close connection, log stack trace  |

Standard LDAP result codes (RFC 4511 Section 4.1.9) will be used.

### 8.4 Backup and Restore

```go
type BackupOptions struct {
    Path       string    // Output directory
    Compress   bool      // Compress backup files
    Incremental bool     // Only changes since last backup
}

type RestoreOptions struct {
    Path       string    // Backup directory
    Verify     bool      // Verify checksums before restore
}
```

- Online backup (consistent snapshot via MVCC)
- LDIF export/import for interoperability
- Point-in-time recovery using WAL

### 8.5 Logging

```go
type LogConfig struct {
    Level   LogLevel  // Debug, Info, Warn, Error
    Format  LogFormat // Text, JSON
    Output  string    // stdout, stderr, or file path
}
```

Log format (JSON):
```json
{
    "ts": "2026-02-18T10:30:00Z",
    "level": "info",
    "msg": "bind successful",
    "dn": "uid=alice,ou=users,dc=example,dc=com",
    "client": "192.168.1.100:54321",
    "duration_ms": 2
}
```

- Structured logging with consistent fields
- Request ID for tracing
- Configurable verbosity per component

### 8.6 Development Environment

Docker-based development environment for consistent builds and testing.

```dockerfile
# Dockerfile.dev
FROM golang:1.22-alpine

WORKDIR /app
RUN apk add --no-cache git make

COPY go.mod ./
RUN go mod download

COPY . .
CMD ["go", "run", "./cmd/oba"]
```

```yaml
# docker-compose.yml
services:
  oba:
    build:
      context: .
      dockerfile: Dockerfile.dev
    ports:
      - "389:389"
      - "636:636"
    volumes:
      - .:/app
      - oba-data:/var/lib/oba
    environment:
      - OBA_LOG_LEVEL=debug

  test-client:
    image: osixia/openldap:latest
    entrypoint: ["ldapsearch", "-x", "-H", "ldap://oba:389"]
    depends_on:
      - oba

volumes:
  oba-data:
```

Features:
- Hot reload with volume mounts
- Isolated test environment
- Pre-configured LDAP test client
- Persistent data volume for ObaDB

### 8.7 CLI Tool

Command-line interface for server management and administration.

```
oba <command> [options]

Commands:
  serve       Start the LDAP server
  backup      Create database backup
  restore     Restore from backup
  user        User management
  config      Configuration management
  version     Show version information
```

#### 8.7.1 Server Commands

```bash
# Start server with default config
oba serve

# Start with custom config file
oba serve --config /etc/oba/config.yaml

# Start with specific options
oba serve --address :389 --tls-address :636 --data-dir /var/lib/oba
```

#### 8.7.2 Backup Commands

```bash
# Full backup
oba backup --output /backup/oba-20260218.bak

# Incremental backup
oba backup --incremental --output /backup/oba-incr.bak

# Compressed backup
oba backup --compress --output /backup/oba.bak.gz

# Export to LDIF
oba backup --format ldif --output /backup/data.ldif
```

#### 8.7.3 Restore Commands

```bash
# Restore from backup
oba restore --input /backup/oba-20260218.bak

# Restore with verification
oba restore --verify --input /backup/oba.bak

# Import from LDIF
oba restore --format ldif --input /backup/data.ldif
```

#### 8.7.4 User Management

```bash
# Add user
oba user add --dn "uid=alice,ou=users,dc=example,dc=com" --password

# Delete user
oba user delete --dn "uid=alice,ou=users,dc=example,dc=com"

# Change password
oba user passwd --dn "uid=alice,ou=users,dc=example,dc=com"

# List users
oba user list --base "ou=users,dc=example,dc=com"

# Lock/unlock user
oba user lock --dn "uid=alice,ou=users,dc=example,dc=com"
oba user unlock --dn "uid=alice,ou=users,dc=example,dc=com"
```

#### 8.7.5 Config Commands

```bash
# Validate config file
oba config validate --config /etc/oba/config.yaml

# Generate default config
oba config init > /etc/oba/config.yaml

# Show effective config
oba config show
```

### 8.8 Configuration File

YAML-based configuration with environment variable override support.

```yaml
# /etc/oba/config.yaml

server:
  address: ":389"
  tlsAddress: ":636"
  tlsCert: "/etc/oba/certs/server.crt"
  tlsKey: "/etc/oba/certs/server.key"
  maxConnections: 10000
  readTimeout: 30s
  writeTimeout: 30s

directory:
  baseDN: "dc=example,dc=com"
  rootDN: "cn=admin,dc=example,dc=com"
  rootPassword: "${OBA_ROOT_PASSWORD}"  # Environment variable

storage:
  dataDir: "/var/lib/oba"
  walDir: "/var/lib/oba/wal"
  pageSize: 4096
  bufferPoolSize: "256MB"
  checkpointInterval: 5m

logging:
  level: "info"           # debug, info, warn, error
  format: "json"          # text, json
  output: "/var/log/oba/oba.log"

security:
  passwordPolicy:
    enabled: true
    minLength: 8
    requireUppercase: true
    requireLowercase: true
    requireDigit: true
    requireSpecial: false
    maxAge: 90d
    historyCount: 5

  rateLimit:
    enabled: true
    maxAttempts: 5
    lockoutDuration: 15m

acl:
  defaultPolicy: "deny"
  rules:
    - target: "*"
      subject: "cn=admin,dc=example,dc=com"
      rights: ["read", "write", "add", "delete"]
    - target: "ou=users,dc=example,dc=com"
      subject: "authenticated"
      rights: ["read", "search"]
      attributes: ["cn", "mail", "uid"]
```

Environment variable override pattern: `OBA_<SECTION>_<KEY>` (e.g., `OBA_SERVER_ADDRESS=:1389`)

### 8.9 Password Policy

```go
type PasswordPolicy struct {
    Enabled          bool
    MinLength        int
    MaxLength        int
    RequireUppercase bool
    RequireLowercase bool
    RequireDigit     bool
    RequireSpecial   bool
    MaxAge           time.Duration  // 0 = never expires
    MinAge           time.Duration  // Minimum time before change allowed
    HistoryCount     int            // Number of old passwords to remember
    MaxFailures      int            // Lock after N failed attempts
    LockoutDuration  time.Duration
    AllowUserChange  bool           // Users can change own password
}
```

Password policy attributes (per-user override):

| Attribute                  | Description                        |
|----------------------------|------------------------------------|
| pwdPolicySubentry          | DN of applicable policy            |
| pwdChangedTime             | Last password change timestamp     |
| pwdAccountLockedTime       | Account lock timestamp             |
| pwdFailureTime             | Failed attempt timestamps          |
| pwdHistory                 | Previous password hashes           |
| pwdGraceUseTime            | Grace login timestamps             |
| pwdReset                   | Password was reset by admin        |
| pwdMustChange              | User must change password on login |

### 8.10 Operational Attributes

Automatically managed attributes (read-only for users):

| Attribute          | Description                              | Example                          |
|--------------------|------------------------------------------|----------------------------------|
| createTimestamp    | Entry creation time                      | 20260218103000Z                  |
| modifyTimestamp    | Last modification time                   | 20260218153045Z                  |
| creatorsName       | DN of entry creator                      | cn=admin,dc=example,dc=com       |
| modifiersName      | DN of last modifier                      | uid=alice,ou=users,dc=example... |
| entryDN            | DN of the entry itself                   | uid=bob,ou=users,dc=example...   |
| entryUUID          | Unique identifier (RFC 4530)             | 550e8400-e29b-41d4-a716-...      |
| subschemaSubentry  | DN of applicable schema                  | cn=schema                        |
| hasSubordinates    | Whether entry has children               | TRUE / FALSE                     |
| numSubordinates    | Count of immediate children              | 5                                |

```go
type OperationalAttrs struct {
    CreateTimestamp   time.Time
    ModifyTimestamp   time.Time
    CreatorsName      string
    ModifiersName     string
    EntryUUID         string
    HasSubordinates   bool
    NumSubordinates   int
}
```

### 8.11 Extended Operations

Support for common LDAP extended operations (RFC 4511 Section 4.12):

| OID                           | Name              | Description                    |
|-------------------------------|-------------------|--------------------------------|
| 1.3.6.1.4.1.4203.1.11.1       | Password Modify   | Change user password (RFC 3062)|
| 1.3.6.1.4.1.4203.1.11.3       | Who Am I          | Return current bind DN         |
| 1.3.6.1.4.1.1466.20037        | StartTLS          | Upgrade to TLS connection      |

#### Password Modify Extended Operation

```go
type PasswordModifyRequest struct {
    UserIdentity []byte  // Optional: DN of user (default: bound user)
    OldPassword  []byte  // Optional: current password
    NewPassword  []byte  // Optional: new password (server generates if empty)
}

type PasswordModifyResponse struct {
    GenPassword []byte  // Server-generated password (if requested)
}
```

- Allows users to change own password
- Admin can reset any user's password
- Validates against password policy
- Updates pwdChangedTime and pwdHistory

#### Who Am I Extended Operation

```go
type WhoAmIResponse struct {
    AuthzID string  // "dn:uid=alice,ou=users,dc=example,dc=com" or "u:alice"
}
```

- Returns empty string for anonymous bind
- Useful for connection pooling and debugging

## 9. Milestones

| Phase    | Deliverable                              | Duration |
|----------|------------------------------------------|----------|
| Phase 1  | BER codec + basic message parsing        | 2 weeks  |
| Phase 2  | ObaDB: Page manager + WAL                | 2 weeks  |
| Phase 3  | ObaDB: Radix tree + B+ tree indexes      | 3 weeks  |
| Phase 4  | ObaDB: MVCC + transactions               | 2 weeks  |
| Phase 5  | Bind, Unbind, Search operations          | 2 weeks  |
| Phase 6  | Add, Delete, Modify operations           | 1 week   |
| Phase 7  | TLS support + StartTLS                   | 1 week   |
| Phase 8  | Filter engine + advanced search          | 2 weeks  |
| Phase 9  | Schema validation + ACL                  | 2 weeks  |
| Phase 10 | Password policy + operational attrs      | 1 week   |
| Phase 11 | Extended operations                      | 1 week   |
| Phase 12 | CLI tool + config file                   | 2 weeks  |
| Phase 13 | Backup/restore + logging                 | 1 week   |
| Phase 14 | Testing, benchmarks, documentation       | 2 weeks  |

## 10. Success Criteria

- Pass LDAP compliance tests (ldapsearch, ldapadd, ldapmodify)
- Compatible with common LDAP clients (Apache Directory Studio, ldap-utils)
- Zero external dependencies (go.mod contains only standard library)
- Benchmark results meet performance targets
- Unit test coverage > 80%

## 11. References

- RFC 4511: LDAP Protocol
- RFC 4512: LDAP Directory Information Models
- RFC 4513: LDAP Authentication Methods
- ITU-T X.690: ASN.1 BER Encoding Rules
