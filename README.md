# Oba

Oba is a lightweight, custom embedded database engine (ObaDB) optimized for LDAP workloads. It also provides a zero-dependency LDAP server implementation written in pure Go.

Current Version: 1.1.0 

## Features

- Pure Go implementation using only the standard library
- Custom embedded database engine (ObaDB) optimized for LDAP workloads
- Full LDAP v3 protocol support
- High availability clustering with Raft consensus
- Automatic config, ACL, and log synchronization across cluster nodes
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
- Web admin panel (React + Vite + Tailwind)
- Persistent log storage with archiving, query and export

## Quick Start

### Using Docker

```bash
git clone https://github.com/KilimcininKorOglu/oba.git
cd oba

# Standalone mode
docker compose up -d

# Cluster mode (3-node HA)
docker compose -f docker-compose.cluster.yml up -d

# Test the connection
ldapsearch -x -H ldap://localhost:1389 -b "dc=example,dc=com" "(objectClass=*)"

# Access web admin panel
open http://localhost:3000
```

Default credentials: `cn=admin,dc=example,dc=com` / `admin`

### Building from Source

```bash
git clone https://github.com/KilimcininKorOglu/oba.git
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

## Web Admin Panel

Oba includes a web-based admin panel built with React, Vite, and Tailwind CSS.

### Features

- Real-time dashboard with server statistics (auto-refresh every 5 seconds)
- Storage, security, system, and LDAP operation metrics
- Recent activity feed
- LDAP entry browser and search
- User and group management with lock/unlock support
- ACL rule editor
- Configuration management
- Log viewer with filtering and export
- Password change functionality

### Access

When running with Docker Compose, the web panel is available at `http://localhost:3000`.

### Ports

| Service   | Port |
|-----------|------|
| LDAP      | 1389 |
| LDAPS     | 636  |
| REST API  | 8080 |
| Web Panel | 3000 |

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
│   ├── logging/       # Structured logging with archiving
│   ├── password/      # Password policy
│   ├── raft/          # Raft consensus for HA clustering
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
├── web/               # Web admin panel (React + Vite + Tailwind)
├── docs/              # Documentation
└── examples/          # Usage examples
```

## Documentation

- [Getting Started](docs/getting-started.md)
- [Installation Guide](docs/installation.md)
- [Configuration Reference](docs/configuration.md)
- [Cluster Mode (HA)](docs/cluster.md)
- [REST API Reference](docs/REST_API.md)
- [Operations Guide](docs/operations.md)
- [Security Guide](docs/security.md)
- [Backup and Restore](docs/backup-restore.md)
- [Change Streams](docs/change-streams.md)
- [Troubleshooting](docs/troubleshooting.md)

## Development

```bash
make test        # Run tests
make test-race   # Run tests with race detector
make lint        # Run fmt and vet
make build       # Build binary
make up          # Build and start all services
make down        # Stop all services
make restart     # Restart all services
make logs        # View server logs
```

## License

MIT License
