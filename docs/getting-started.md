# Getting Started with Oba

Oba is a lightweight, zero-dependency LDAP server implementation written in pure Go. This guide will help you get up and running quickly.

## Prerequisites

- Go 1.22 or later
- A Unix-like operating system (Linux, macOS) or Windows

## Quick Start

### 1. Build from Source

```bash
git clone https://github.com/oba-ldap/oba.git
cd oba
go build -o oba ./cmd/oba
```

### 2. Generate Default Configuration

```bash
./oba config init > config.yaml
```

### 3. Edit Configuration

Open `config.yaml` and set your directory configuration:

```yaml
directory:
  baseDN: "dc=example,dc=com"
  rootDN: "cn=admin,dc=example,dc=com"
  rootPassword: "your-secure-password"
```

### 4. Start the Server

```bash
./oba serve --config config.yaml
```

The server will start listening on:
- Port 389 for LDAP connections
- Port 636 for LDAPS connections (if TLS is configured)

### 5. Test the Connection

Using standard LDAP tools:

```bash
ldapsearch -x -H ldap://localhost:389 -b "dc=example,dc=com" "(objectClass=*)"
```

## Basic Operations

### Adding a User

```bash
./oba user add --dn "uid=alice,ou=users,dc=example,dc=com" --password
```

### Listing Users

```bash
./oba user list --base "ou=users,dc=example,dc=com"
```

### Changing a Password

```bash
./oba user passwd --dn "uid=alice,ou=users,dc=example,dc=com"
```

## Directory Structure

After installation, Oba uses the following directory structure:

| Path              | Description                    |
|-------------------|--------------------------------|
| /var/lib/oba      | Default data directory         |
| /var/lib/oba/wal  | Write-ahead log files          |
| /etc/oba          | Configuration files (optional) |
| /var/log/oba      | Log files (if file logging)    |

## Next Steps

- [Installation Guide](installation.md) - Detailed installation instructions
- [Configuration Reference](configuration.md) - Complete configuration options
- [Security Guide](security.md) - Security best practices
- [Operations Guide](operations.md) - Day-to-day operations
