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

## Configuration

Create a `config.yaml` file:

```yaml
server:
  address: ":1389"

directory:
  baseDN: "dc=example,dc=com"
  rootDN: "cn=admin,dc=example,dc=com"
  rootPassword: "admin"

storage:
  dataDir: "/var/lib/oba"
  cacheSize: 10000

logging:
  level: "info"
  format: "json"
```

## Supported Operations

| Operation  | Description                    |
|------------|--------------------------------|
| Bind       | Simple authentication          |
| Unbind     | Connection termination         |
| Search     | Entry lookup with filters      |
| Add        | Create new entries             |
| Delete     | Remove entries                 |
| Modify     | Update entry attributes        |
| ModifyDN   | Rename or move entries         |
| Compare    | Compare attribute values       |
| Abandon    | Cancel pending operations      |

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
# Run tests
make test

# Run tests with race detector
make test-race

# Run benchmarks
make bench

# Build
make build
```

## License

MIT License
