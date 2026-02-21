# Oba

Oba is a lightweight, zero-dependency LDAP server implementation written in pure Go.

## Features

- Pure Go implementation using only the standard library
- Custom embedded database engine (ObaDB) optimized for LDAP workloads
- Full LDAP v3 protocol support
- TLS/LDAPS and StartTLS support
- REST API with JWT authentication
- Access Control Lists (ACL) with hot reload
- Configuration hot reload without restart
- REST API management for ACL and config
- Password policy enforcement
- Encryption at rest (AES-256-GCM)
- Change streams via LDAP Persistent Search
- Write-Ahead Logging (WAL) for crash recovery
- MVCC for concurrent access
- Backup and restore with automatic timestamps

## Quick Start

### Using Docker

```bash
git clone https://github.com/oba-ldap/oba.git
cd oba
docker compose up -d

# Test the connection
ldapsearch -x -H ldap://localhost:1389 -b "dc=example,dc=com" "(objectClass=*)"
```

### Building from Source

```bash
git clone https://github.com/oba-ldap/oba.git
cd oba
make build
./bin/oba serve --config config.yaml
```

## CLI Commands

```
oba serve       Start the LDAP server
oba backup      Create database backup (auto-timestamped)
oba restore     Restore from backup
oba user        User management (add, delete, passwd, list, lock, unlock)
oba config      Configuration management (validate, init, show)
oba reload      Reload configuration without restart (acl)
oba version     Show version information
```

## Configuration

Create a `config.yaml` file (see `config.example.yaml` for all options):

```yaml
server:
  address: ":1389"
  pidFile: "/var/run/oba.pid"

directory:
  baseDN: "dc=example,dc=com"
  rootDN: "cn=admin,dc=example,dc=com"
  rootPassword: "admin"

storage:
  dataDir: "./data"

logging:
  level: "info"
  format: "json"

# External ACL file with hot reload support
aclFile: "/etc/oba/acl.yaml"

# Optional: REST API
rest:
  enabled: true
  address: ":8080"
  jwtSecret: "your-secret-key"
```

See `acl.example.yaml` for ACL configuration format.

## Supported Operations

| Operation | Description               |
|-----------|---------------------------|
| Bind      | Simple authentication     |
| Unbind    | Connection termination    |
| Search    | Entry lookup with filters |
| Add       | Create new entries        |
| Delete    | Remove entries            |
| Modify    | Update entry attributes   |
| ModifyDN  | Rename or move entries    |
| Compare   | Compare attribute values  |
| Abandon   | Cancel pending operations |

## Architecture

```
oba/
├── cmd/oba/           # CLI entry point
├── internal/
│   ├── acl/           # Access control with hot reload
│   ├── backend/       # LDAP backend
│   ├── backup/        # Backup/restore
│   ├── ber/           # ASN.1 BER codec
│   ├── config/        # Configuration parsing
│   ├── crypto/        # Encryption utilities
│   ├── filter/        # Search filter evaluation
│   ├── ldap/          # LDAP protocol messages
│   ├── logging/       # Structured logging
│   ├── password/      # Password policy
│   ├── rest/          # REST API server
│   ├── schema/        # LDAP schema validation
│   ├── server/        # Connection handling
│   └── storage/       # ObaDB storage engine
│       ├── btree/     # B+ tree indexing
│       ├── cache/     # LRU cache
│       ├── engine/    # Storage engine interface
│       ├── index/     # Index management
│       ├── mvcc/      # Multi-version concurrency
│       ├── radix/     # Radix tree for DN hierarchy
│       ├── stream/    # Change streams
│       └── tx/        # Transaction management
├── docs/              # Documentation
└── examples/          # Usage examples
```

## Documentation

- [Getting Started](docs/getting-started.md)
- [Installation Guide](docs/installation.md)
- [Configuration Reference](docs/configuration.md)
- [REST API Reference](docs/REST_API.md)
- [Operations Guide](docs/operations.md)
- [Security Guide](docs/security.md)
- [Backup and Restore](docs/backup-restore.md)
- [Change Streams](docs/change-streams.md)
- [Troubleshooting](docs/troubleshooting.md)

## Development

```bash
make test       # Run tests
make test-race  # Run tests with race detector
make bench      # Run benchmarks
make build      # Build binary
```

Docker build:

```bash
docker build -t oba:latest .
```

## License

MIT License
