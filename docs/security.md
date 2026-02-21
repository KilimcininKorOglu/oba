# Security Guide

This guide covers security best practices for deploying and operating Oba LDAP server.

## TLS Configuration

### Enabling LDAPS (TLS)

TLS is essential for protecting LDAP traffic. Configure TLS in your configuration file:

```yaml
server:
  address: ":389"      # Plain LDAP (consider disabling in production)
  tlsAddress: ":636"   # LDAPS
  tlsCert: "/etc/oba/certs/server.crt"
  tlsKey: "/etc/oba/certs/server.key"
```

### StartTLS

StartTLS allows upgrading a plain LDAP connection to TLS. This is useful when you want to use a single port for both plain and encrypted connections.

```bash
# Connect using StartTLS
ldapsearch -x -H ldap://localhost:389 -ZZ \
  -D "cn=admin,dc=example,dc=com" -W \
  -b "dc=example,dc=com" "(objectClass=*)"
```

The `-ZZ` flag requires StartTLS to succeed. Use `-Z` to attempt StartTLS but continue if it fails.

**Note:** StartTLS is automatically supported when TLS certificates are configured. No additional configuration is required.

### Generating Self-Signed Certificates

For testing environments:

```bash
# Generate private key
openssl genrsa -out server.key 4096

# Generate certificate signing request
openssl req -new -key server.key -out server.csr \
  -subj "/CN=ldap.example.com/O=Example Inc/C=US"

# Generate self-signed certificate
openssl x509 -req -days 365 -in server.csr -signkey server.key -out server.crt

# Set permissions
chmod 600 server.key
chmod 644 server.crt
```

### Using Let's Encrypt Certificates

For production environments:

```bash
# Install certbot
sudo apt install certbot

# Obtain certificate
sudo certbot certonly --standalone -d ldap.example.com

# Configure Oba to use Let's Encrypt certificates
# server:
#   tlsCert: "/etc/letsencrypt/live/ldap.example.com/fullchain.pem"
#   tlsKey: "/etc/letsencrypt/live/ldap.example.com/privkey.pem"
```

### TLS Version Requirements

Oba enforces TLS 1.2 as the minimum version by default. The supported cipher suites are:

| Cipher Suite                                | TLS Version |
|---------------------------------------------|-------------|
| TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384       | TLS 1.2     |
| TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384         | TLS 1.2     |
| TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256       | TLS 1.2     |
| TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256         | TLS 1.2     |
| TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256 | TLS 1.2     |
| TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256   | TLS 1.2     |

TLS 1.3 cipher suites are automatically managed by Go.

## Password Security

### Password Policy Configuration

Enable and configure password policy:

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

## Encryption at Rest

Oba supports encryption at rest using AES-256-GCM to protect stored data.

### Configuration

```yaml
security:
  encryption:
    enabled: true
    keyFile: "/etc/oba/encryption.key"
```

### Key Generation

Generate a 256-bit encryption key:

```bash
# Generate key file (hex format)
openssl rand -hex 32 > /etc/oba/encryption.key

# Set secure permissions
chmod 600 /etc/oba/encryption.key
chown oba:oba /etc/oba/encryption.key
```

### Key Format

The key file must contain exactly:
- 32 bytes of raw binary data, OR
- 64 hexadecimal characters

### Key Rotation

To rotate the encryption key:

1. Stop the server
2. Export data to LDIF (unencrypted)
3. Replace the key file
4. Re-import data
5. Start the server

```bash
# Export current data
oba backup --format ldif --output /backup/data.ldif

# Replace key
openssl rand -hex 32 > /etc/oba/encryption.key

# Clear and re-import
rm -rf /var/lib/oba/*
oba restore --format ldif --input /backup/data.ldif
```

### Security Considerations

- Store the key file on a separate, encrypted filesystem
- Never commit the key file to version control
- Back up the key file securely (without it, data is unrecoverable)
- Use environment variables for key path in containerized deployments

### Password Policy Parameters

| Parameter        | Recommended Value | Description                          |
|------------------|-------------------|--------------------------------------|
| minLength        | 12+               | Minimum password length              |
| requireUppercase | true              | At least one uppercase letter        |
| requireLowercase | true              | At least one lowercase letter        |
| requireDigit     | true              | At least one number                  |
| requireSpecial   | true              | At least one special character       |
| maxAge           | 90d               | Password expiration period           |
| historyCount     | 5                 | Prevent reuse of recent passwords    |

### Password Storage

Oba stores passwords using secure hashing algorithms:

| Algorithm | Format                  | Description                              |
|-----------|-------------------------|------------------------------------------|
| SSHA      | `{SSHA}base64...`       | Salted SHA-1 (legacy compatibility)      |
| SHA256    | `{SHA256}base64...`     | SHA-256 hash                             |
| SHA512    | `{SHA512}base64...`     | SHA-512 hash                             |
| PBKDF2    | `{PBKDF2}iterations$...`| PBKDF2-SHA256 with configurable iterations|

**Security features:**
- All hashes are salted to prevent rainbow table attacks
- PBKDF2 uses 100,000 iterations by default
- Passwords are never stored in plain text
- Comparison is done in constant time to prevent timing attacks

**Recommended:** Use PBKDF2 for new deployments. SHA256/SHA512 are supported for compatibility.

## Rate Limiting and Account Lockout

### Configuration

```yaml
security:
  rateLimit:
    enabled: true
    maxAttempts: 5
    lockoutDuration: 15m
```

### Behavior

- After `maxAttempts` failed authentication attempts, the account is locked
- Lockout duration is configurable via `lockoutDuration`
- Administrators can manually unlock accounts using `oba user unlock`

### Unlocking Accounts

```bash
# Unlock a locked account
oba user unlock --dn "uid=alice,ou=users,dc=example,dc=com"
```

## Access Control Lists (ACL)

### ACL Configuration

```yaml
acl:
  defaultPolicy: "deny"
  rules:
    # Admin has full access
    - target: "*"
      subject: "cn=admin,dc=example,dc=com"
      rights: ["read", "write", "add", "delete", "search", "compare"]
    
    # Authenticated users can read user entries
    - target: "ou=users,dc=example,dc=com"
      subject: "authenticated"
      rights: ["read", "search"]
      attributes: ["cn", "mail", "uid", "telephoneNumber"]
    
    # Users can modify their own entry
    - target: "ou=users,dc=example,dc=com"
      subject: "self"
      rights: ["read", "write"]
      attributes: ["userPassword", "mail", "telephoneNumber"]
    
    # Anonymous can search for names only
    - target: "ou=users,dc=example,dc=com"
      subject: "anonymous"
      rights: ["search"]
      attributes: ["cn"]
```

### ACL Rights

| Right   | Description                              |
|---------|------------------------------------------|
| read    | Read entry attributes                    |
| write   | Modify entry attributes                  |
| add     | Create new entries                       |
| delete  | Remove entries                           |
| search  | Search for entries                       |
| compare | Compare attribute values                 |

### ACL Subjects

| Subject       | Description                              |
|---------------|------------------------------------------|
| anonymous     | Unauthenticated connections              |
| authenticated | Any authenticated user                   |
| self          | The entry being accessed matches bind DN |
| DN            | Specific user DN                         |
| *             | Everyone (anonymous and authenticated)   |

## Network Security

### Firewall Configuration

Restrict access to LDAP ports:

```bash
# Allow LDAP from internal network only
sudo ufw allow from 10.0.0.0/8 to any port 389
sudo ufw allow from 10.0.0.0/8 to any port 636

# Or using iptables
iptables -A INPUT -p tcp --dport 389 -s 10.0.0.0/8 -j ACCEPT
iptables -A INPUT -p tcp --dport 636 -s 10.0.0.0/8 -j ACCEPT
iptables -A INPUT -p tcp --dport 389 -j DROP
iptables -A INPUT -p tcp --dport 636 -j DROP
```

### Binding to Specific Interfaces

Bind to internal interfaces only:

```yaml
server:
  address: "10.0.0.1:389"
  tlsAddress: "10.0.0.1:636"
```

### Using a Reverse Proxy

For additional security, place Oba behind a reverse proxy:

```nginx
# nginx configuration
stream {
    upstream ldap_backend {
        server 127.0.0.1:389;
    }
    
    server {
        listen 389;
        proxy_pass ldap_backend;
        proxy_timeout 30s;
    }
}
```

## Secure Deployment Checklist

### Pre-Deployment

- [ ] Generate or obtain TLS certificates
- [ ] Configure strong password policy
- [ ] Set up ACL rules following least privilege principle
- [ ] Configure rate limiting
- [ ] Set secure file permissions

### File Permissions

```bash
# Configuration file
chmod 600 /etc/oba/config.yaml
chown oba:oba /etc/oba/config.yaml

# TLS certificates
chmod 600 /etc/oba/certs/server.key
chmod 644 /etc/oba/certs/server.crt
chown oba:oba /etc/oba/certs/*

# Data directory
chmod 700 /var/lib/oba
chown oba:oba /var/lib/oba
```

### Post-Deployment

- [ ] Verify TLS is working correctly
- [ ] Test ACL rules
- [ ] Verify password policy enforcement
- [ ] Set up monitoring and alerting
- [ ] Configure log rotation
- [ ] Document emergency procedures

## Security Monitoring

### Audit Logging

Enable detailed logging for security events:

```yaml
logging:
  level: "info"
  format: "json"
  output: "/var/log/oba/oba.log"
```

### Log Events to Monitor

| Event Type              | Log Level | Action                          |
|-------------------------|-----------|----------------------------------|
| Failed authentication   | warn      | Monitor for brute force attacks  |
| Account lockout         | warn      | Investigate potential attacks    |
| ACL denial              | info      | Review access patterns           |
| Configuration change    | info      | Audit trail                      |
| TLS handshake failure   | warn      | Check client compatibility       |

### Log Analysis

```bash
# Find failed authentication attempts
grep "bind failed" /var/log/oba/oba.log | jq '.'

# Count failed attempts by IP
grep "bind failed" /var/log/oba/oba.log | jq -r '.client' | sort | uniq -c | sort -rn

# Find account lockouts
grep "account locked" /var/log/oba/oba.log
```

## Incident Response

### Suspected Breach

1. Isolate the server from the network
2. Preserve logs and evidence
3. Analyze access patterns
4. Reset compromised credentials
5. Review and update ACL rules
6. Restore from known-good backup if necessary

### Account Compromise

```bash
# Lock the compromised account immediately
oba user lock --dn "uid=compromised,ou=users,dc=example,dc=com"

# Review recent activity in logs
grep "uid=compromised" /var/log/oba/oba.log

# Reset password after investigation
oba user passwd --dn "uid=compromised,ou=users,dc=example,dc=com"

# Unlock when ready
oba user unlock --dn "uid=compromised,ou=users,dc=example,dc=com"
```

## Compliance Considerations

### Data Protection

- Encrypt data at rest (use encrypted filesystem)
- Encrypt data in transit (TLS)
- Implement access controls (ACL)
- Maintain audit logs

### Password Requirements

Common compliance frameworks require:

| Framework | Min Length | Complexity | Expiration |
|-----------|------------|------------|------------|
| PCI DSS   | 7          | Yes        | 90 days    |
| HIPAA     | 8          | Yes        | Varies     |
| SOC 2     | 8          | Yes        | 90 days    |
| NIST      | 8          | No         | No         |

Configure password policy according to your compliance requirements.
