# Security Guide

This guide covers security best practices for deploying and operating Oba LDAP server.

## TLS Configuration

### Enabling TLS

TLS is essential for protecting LDAP traffic. Configure TLS in your configuration file:

```yaml
server:
  tlsAddress: ":636"
  tlsCert: "/etc/oba/certs/server.crt"
  tlsKey: "/etc/oba/certs/server.key"
```

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

| Cipher Suite                                  | TLS Version |
|-----------------------------------------------|-------------|
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

Oba stores passwords using secure hashing:

- SHA256 for basic hashing
- PBKDF2 for enhanced security
- Salted hashes to prevent rainbow table attacks

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
