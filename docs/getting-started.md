# Getting Started with Oba

Oba is a lightweight, zero-dependency LDAP server implementation written in pure Go. This guide will help you get up and running quickly.

## Prerequisites

- Go 1.22 or later (for building from source)
- A Unix-like operating system (Linux, macOS) or Windows
- Docker (optional, for containerized deployment)

## Quick Start

### Option 1: Using Docker (Recommended)

The fastest way to get started:

```bash
# Clone the repository
git clone https://github.com/oba-ldap/oba.git
cd oba

# Start with Docker Compose
docker compose up -d

# Test the connection
ldapsearch -x -H ldap://localhost:1389 -b "dc=example,dc=com" "(objectClass=*)"
```

### Option 2: Build from Source

```bash
# Clone the repository
git clone https://github.com/oba-ldap/oba.git
cd oba

# Build using Makefile
make build

# Or build directly with Go
go build -o bin/oba ./cmd/oba
```

### 2. Create Configuration File

```bash
# Copy the example configuration
cp config.example.yaml config.yaml

# Edit the configuration
# Set your directory configuration (baseDN, rootDN, rootPassword)
# IMPORTANT: dataDir must be an absolute path
```

Example `config.yaml`:

```yaml
server:
  address: ":1389"

directory:
  baseDN: "dc=example,dc=com"
  rootDN: "cn=admin,dc=example,dc=com"
  rootPassword: "your-secure-password"

storage:
  dataDir: "/var/lib/oba"  # Must be absolute path
  pageSize: 4096

logging:
  level: "info"
  format: "json"
  output: "stdout"
```

### 3. Start the Server

```bash
./bin/oba serve --config config.yaml
```

**Note:** Port 389 requires root privileges. Use a higher port like 1389 for non-root users, or run with sudo.

### 4. Test the Connection

```bash
# Anonymous search
ldapsearch -x -H ldap://localhost:1389 -b "dc=example,dc=com" "(objectClass=*)"

# Authenticated search
ldapsearch -x -H ldap://localhost:1389 \
  -D "cn=admin,dc=example,dc=com" -w your-secure-password \
  -b "dc=example,dc=com" "(objectClass=*)"
```

## Adding Your First Entries

Create the base structure:

```bash
ldapadd -x -H ldap://localhost:1389 -D "cn=admin,dc=example,dc=com" -w your-secure-password << 'EOF'
dn: dc=example,dc=com
objectClass: top
objectClass: domain
dc: example

dn: ou=users,dc=example,dc=com
objectClass: top
objectClass: organizationalUnit
ou: users

dn: ou=groups,dc=example,dc=com
objectClass: top
objectClass: organizationalUnit
ou: groups

dn: uid=alice,ou=users,dc=example,dc=com
objectClass: top
objectClass: person
objectClass: inetOrgPerson
uid: alice
cn: Alice Smith
sn: Smith
mail: alice@example.com
userPassword: secret123
EOF
```

## Makefile Commands

| Command           | Description                    |
|-------------------|--------------------------------|
| `make build`      | Build the binary to bin/       |
| `make test`       | Run all tests                  |
| `make test-race`  | Run tests with race detector   |
| `make test-cover` | Run tests with coverage        |
| `make bench`      | Run benchmarks                 |
| `make clean`      | Remove build artifacts         |
| `make run`        | Build and run the server       |

## Directory Structure

After installation, Oba uses the following directory structure:

| Path            | Description                  |
|-----------------|------------------------------|
| /var/lib/oba    | Default data directory       |
| /var/lib/oba/wal| Write-ahead log files        |
| /etc/oba        | Configuration files (optional)|
| /var/log/oba    | Log files (if file logging)  |

## Next Steps

- [Installation Guide](installation.md) - Detailed installation instructions
- [Configuration Reference](configuration.md) - Complete configuration options
- [Security Guide](security.md) - Security best practices
- [Operations Guide](operations.md) - Day-to-day operations
