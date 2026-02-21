# Configuration Reference

This document provides a complete reference for all Oba configuration options.

## Configuration File Format

Oba uses YAML configuration files. Generate a default configuration with:

```bash
oba config init > config.yaml
```

## Environment Variable Overrides

Configuration values can be overridden using environment variables following the pattern:

```
OBA_<SECTION>_<KEY>
```

Examples:
- `OBA_SERVER_ADDRESS=:1389`
- `OBA_DIRECTORY_ROOT_PASSWORD=secret`
- `OBA_LOGGING_LEVEL=debug`

## Server Configuration

| Parameter             | Type     | Default | Description                        |
|-----------------------|----------|---------|------------------------------------|
| server.address        | string   | ":389"  | LDAP listen address                |
| server.tlsAddress     | string   | ":636"  | LDAPS listen address               |
| server.tlsCert        | string   | ""      | Path to TLS certificate file       |
| server.tlsKey         | string   | ""      | Path to TLS private key file       |
| server.maxConnections | int      | 10000   | Maximum concurrent connections     |
| server.readTimeout    | duration | 30s     | Read timeout per operation         |
| server.writeTimeout   | duration | 30s     | Write timeout per operation        |
| server.pidFile        | string   | ""      | PID file path (for reload command) |

Example:

```yaml
server:
  address: ":389"
  tlsAddress: ":636"
  tlsCert: "/etc/oba/certs/server.crt"
  tlsKey: "/etc/oba/certs/server.key"
  maxConnections: 10000
  readTimeout: 30s
  writeTimeout: 30s
  pidFile: "/var/run/oba.pid"
```

## Directory Configuration

| Parameter              | Type   | Default | Description             |
|------------------------|--------|---------|-------------------------|
| directory.baseDN       | string | ""      | Base distinguished name |
| directory.rootDN       | string | ""      | Administrator DN        |
| directory.rootPassword | string | ""      | Administrator password  |

Example:

```yaml
directory:
  baseDN: "dc=example,dc=com"
  rootDN: "cn=admin,dc=example,dc=com"
  rootPassword: "${OBA_DIRECTORY_ROOT_PASSWORD}"
```

## Storage Configuration

| Parameter                  | Type     | Default        | Description                         |
|----------------------------|----------|----------------|-------------------------------------|
| storage.dataDir            | string   | "/var/lib/oba" | Data directory path                 |
| storage.walDir             | string   | ""             | WAL directory (defaults to dataDir) |
| storage.pageSize           | int      | 4096           | Page size in bytes                  |
| storage.bufferPoolSize     | string   | "256MB"        | Buffer pool size                    |
| storage.checkpointInterval | duration | 5m             | Checkpoint interval                 |
| storage.cacheSize          | int      | 10000          | Entry cache size (LRU)              |

Both absolute and relative paths are supported for `dataDir` and `walDir`. Relative paths are resolved from the current working directory.

Example:

```yaml
storage:
  # Absolute path
  dataDir: "/var/lib/oba"
  
  # Or relative path (resolved from current directory)
  # dataDir: "./data"
  
  walDir: "/var/lib/oba/wal"
  pageSize: 4096
  bufferPoolSize: "256MB"
  checkpointInterval: 5m
  cacheSize: 10000
```

### Storage File Layout

Oba creates the following files in the data directory:

| File      | Description                            |
|-----------|----------------------------------------|
| data.oba  | Main data file with entries            |
| index.oba | B+ tree indexes for attribute searches |
| wal.oba   | Write-ahead log for crash recovery     |

### Index Configuration

Oba automatically creates indexes for commonly searched attributes. The following indexes are created by default:

| Attribute   | Index Type | Description                |
|-------------|------------|----------------------------|
| objectClass | Equality   | Fast objectClass filtering |
| uid         | Equality   | User identifier lookups    |
| cn          | Equality   | Common name searches       |
| mail        | Equality   | Email address lookups      |
| member      | Equality   | Group membership queries   |

Custom indexes can be created programmatically using the storage engine API.

## Logging Configuration

| Parameter      | Type   | Default  | Description                          |
|----------------|--------|----------|--------------------------------------|
| logging.level  | string | "info"   | Log level: debug, info, warn, error  |
| logging.format | string | "json"   | Log format: text, json               |
| logging.output | string | "stdout" | Output: stdout, stderr, or file path |

Example:

```yaml
logging:
  level: "info"
  format: "json"
  output: "/var/log/oba/oba.log"
```

### Log Levels

| Level | Description                          |
|-------|--------------------------------------|
| debug | Detailed debugging information       |
| info  | General operational information      |
| warn  | Warning conditions                   |
| error | Error conditions requiring attention |

## Security Configuration

### Password Policy

| Parameter                                | Type     | Default | Description                         |
|------------------------------------------|----------|---------|-------------------------------------|
| security.passwordPolicy.enabled          | bool     | false   | Enable password policy enforcement  |
| security.passwordPolicy.minLength        | int      | 8       | Minimum password length             |
| security.passwordPolicy.requireUppercase | bool     | true    | Require uppercase letter            |
| security.passwordPolicy.requireLowercase | bool     | true    | Require lowercase letter            |
| security.passwordPolicy.requireDigit     | bool     | true    | Require numeric digit               |
| security.passwordPolicy.requireSpecial   | bool     | false   | Require special character           |
| security.passwordPolicy.maxAge           | duration | 0       | Password expiration (0 = never)     |
| security.passwordPolicy.historyCount     | int      | 0       | Number of old passwords to remember |

Example:

```yaml
security:
  passwordPolicy:
    enabled: true
    minLength: 12
    requireUppercase: true
    requireLowercase: true
    requireDigit: true
    requireSpecial: true
    maxAge: 90d
    historyCount: 5
```

### Rate Limiting

| Parameter                          | Type     | Default | Description                        |
|------------------------------------|----------|---------|------------------------------------|
| security.rateLimit.enabled         | bool     | false   | Enable rate limiting               |
| security.rateLimit.maxAttempts     | int      | 5       | Max failed attempts before lockout |
| security.rateLimit.lockoutDuration | duration | 15m     | Account lockout duration           |

Example:

```yaml
security:
  rateLimit:
    enabled: true
    maxAttempts: 5
    lockoutDuration: 15m
```

### Encryption at Rest

| Parameter                   | Type   | Default | Description                       |
|-----------------------------|--------|---------|-----------------------------------|
| security.encryption.enabled | bool   | false   | Enable encryption for stored data |
| security.encryption.keyFile | string | ""      | Path to encryption key file       |

Oba uses AES-256-GCM for encryption at rest. The key file must contain exactly 32 bytes (raw binary) or 64 hexadecimal characters.

Example:

```yaml
security:
  encryption:
    enabled: true
    keyFile: "/etc/oba/encryption.key"
```

Generate an encryption key:

```bash
openssl rand -hex 32 > /etc/oba/encryption.key
chmod 600 /etc/oba/encryption.key
```

## ACL Configuration

Oba supports two ACL configuration methods:

### Option 1: External ACL File (Recommended)

Use an external YAML file for ACL rules. This enables hot reload without server restart.

| Parameter | Type   | Default | Description               |
|-----------|--------|---------|---------------------------|
| aclFile   | string | ""      | Path to external ACL file |

```yaml
aclFile: "/etc/oba/acl.yaml"
```

The external ACL file format:

```yaml
version: 1
defaultPolicy: "deny"
rules:
  - target: "*"
    subject: "cn=admin,dc=example,dc=com"
    rights: ["read", "write", "add", "delete", "search", "compare"]
  - target: "ou=users,dc=example,dc=com"
    subject: "authenticated"
    rights: ["read", "search"]
```

Hot reload ACL without restart:

```bash
# Automatic: Changes detected within ~300ms
# Manual: Send SIGHUP signal
kill -SIGHUP $(cat /var/run/oba.pid)
# Or use CLI
oba reload acl
```

### Option 2: Inline ACL (No Hot Reload)

| Parameter         | Type   | Default | Description                 |
|-------------------|--------|---------|-----------------------------|
| acl.defaultPolicy | string | "deny"  | Default policy: allow, deny |
| acl.rules         | array  | []      | List of ACL rules           |

### ACL Rule Structure

| Field      | Type     | Description                                           |
|------------|----------|-------------------------------------------------------|
| target     | string   | DN pattern or "*" for all entries                     |
| subject    | string   | Who: DN, "anonymous", "authenticated"                 |
| rights     | []string | Operations: read, write, add, delete, search, compare |
| attributes | []string | Specific attributes or "*" for all                    |

Example:

```yaml
acl:
  defaultPolicy: "deny"
  rules:
    - target: "*"
      subject: "cn=admin,dc=example,dc=com"
      rights: ["read", "write", "add", "delete", "search", "compare"]
    - target: "ou=users,dc=example,dc=com"
      subject: "authenticated"
      rights: ["read", "search"]
      attributes: ["cn", "mail", "uid"]
    - target: "*"
      subject: "anonymous"
      rights: ["search"]
      attributes: ["cn"]
```

## Complete Configuration Example

```yaml
# Oba LDAP Server Configuration

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
  rootPassword: "${OBA_DIRECTORY_ROOT_PASSWORD}"

storage:
  dataDir: "/var/lib/oba"
  walDir: "/var/lib/oba/wal"
  pageSize: 4096
  bufferPoolSize: "256MB"
  checkpointInterval: 5m

logging:
  level: "info"
  format: "json"
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
      rights: ["read", "write", "add", "delete", "search", "compare"]
    - target: "ou=users,dc=example,dc=com"
      subject: "authenticated"
      rights: ["read", "search"]
```

## Duration Format

Duration values support the following units:

| Unit | Description |
|------|-------------|
| s    | Seconds     |
| m    | Minutes     |
| h    | Hours       |
| d    | Days        |

Examples: `30s`, `5m`, `1h`, `90d`

## REST API Configuration

| Parameter        | Type     | Default | Description                  |
|------------------|----------|---------|------------------------------|
| rest.enabled     | bool     | false   | Enable REST API              |
| rest.address     | string   | ":8080" | HTTP listen address          |
| rest.tlsAddress  | string   | ""      | HTTPS listen address         |
| rest.jwtSecret   | string   | ""      | JWT secret for token signing |
| rest.tokenTTL    | duration | 24h     | JWT token validity period    |
| rest.rateLimit   | int      | 100     | Requests per second per IP   |
| rest.corsOrigins | []string | ["*"]   | Allowed CORS origins         |

Example:

```yaml
rest:
  enabled: true
  address: ":8080"
  tlsAddress: ":8443"
  jwtSecret: "your-secret-key-at-least-32-characters"
  tokenTTL: 24h
  rateLimit: 100
  corsOrigins:
    - "https://app.example.com"
```

Generate a JWT secret:

```bash
openssl rand -hex 32
```

See [REST API Documentation](REST_API.md) for endpoint details.

## Validating Configuration

Validate your configuration file before starting the server:

```bash
oba config validate --config /etc/oba/config.yaml
```

Show the effective configuration (with environment overrides applied):

```bash
oba config show --config /etc/oba/config.yaml
```
