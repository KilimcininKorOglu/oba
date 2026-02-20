# Oba

Oba is a lightweight, zero-dependency LDAP server implementation written in pure Go.

## Features

- Pure Go implementation using only the standard library
- Custom embedded database engine (ObaDB) optimized for LDAP workloads
- Full LDAP v3 protocol support
- TLS/LDAPS and StartTLS support
- Access Control Lists (ACL)
- Password policy enforcement
- Write-Ahead Logging (WAL) for crash recovery
- MVCC for concurrent access
- Index persistence cache for fast startup
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
oba version     Show version information
```

## Configuration

Create a `config.yaml` file (see `config.example.yaml` for all options):

```yaml
server:
  address: ":1389"

directory:
  baseDN: "dc=example,dc=com"
  rootDN: "cn=admin,dc=example,dc=com"
  rootPassword: "admin"

storage:
  dataDir: "./data"
  cacheSize: 10000

logging:
  level: "info"
  format: "json"
```

## Supported Operations

| Operation | Description                |
|-----------|----------------------------|
| Bind      | Simple authentication      |
| Unbind    | Connection termination     |
| Search    | Entry lookup with filters  |
| Add       | Create new entries         |
| Delete    | Remove entries             |
| Modify    | Update entry attributes    |
| ModifyDN  | Rename or move entries     |
| Compare   | Compare attribute values   |
| Abandon   | Cancel pending operations  |

## Architecture

```
oba/
├── cmd/oba/           # CLI entry point
├── internal/
│   ├── ber/           # ASN.1 BER codec
│   ├── ldap/          # LDAP protocol messages
│   ├── server/        # Connection handling
│   ├── storage/       # ObaDB storage engine
│   │   ├── btree/     # B+ tree indexing
│   │   ├── radix/     # Radix tree for DN hierarchy
│   │   ├── mvcc/      # Multi-version concurrency
│   │   ├── cache/     # Index persistence cache
│   │   └── engine/    # Storage engine interface
│   ├── backend/       # LDAP backend
│   ├── filter/        # Search filter evaluation
│   ├── schema/        # LDAP schema validation
│   ├── acl/           # Access control
│   ├── password/      # Password policy
│   └── backup/        # Backup/restore
├── docs/              # Documentation
└── examples/          # Usage examples
```

## Documentation

- [Getting Started](docs/getting-started.md)
- [Installation Guide](docs/installation.md)
- [Configuration Reference](docs/configuration.md)
- [Operations Guide](docs/operations.md)
- [Security Guide](docs/security.md)
- [Backup and Restore](docs/backup-restore.md)
- [Troubleshooting](docs/troubleshooting.md)

## Development

```bash
make test       # Run tests
make test-race  # Run tests with race detector
make bench      # Run benchmarks
make build      # Build binary
make docker     # Build Docker image
```

## License

MIT License
