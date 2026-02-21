# Operations Guide

This guide covers day-to-day operational procedures for managing an Oba LDAP server.

## Server Management

### Starting the Server

```bash
# Start with default settings
oba serve

# Start with configuration file
oba serve --config /etc/oba/config.yaml

# Start with command-line overrides
oba serve --address :1389 --tls-address :1636 --data-dir /data/oba
```

### Stopping the Server

Oba handles graceful shutdown on SIGTERM and SIGINT signals:

```bash
# If running as systemd service
sudo systemctl stop oba

# If running in foreground
# Press Ctrl+C or send SIGTERM
kill -TERM $(pidof oba)
```

### Checking Server Status

```bash
# Systemd service status
sudo systemctl status oba

# Test LDAP connectivity
ldapsearch -x -H ldap://localhost:389 -b "" -s base "(objectClass=*)" namingContexts

# Check version
oba version
```

## User Management

### Adding Users

```bash
# Add user with password prompt
oba user add --dn "uid=alice,ou=users,dc=example,dc=com" --password

# Using ldapadd
ldapadd -x -H ldap://localhost:389 -D "cn=admin,dc=example,dc=com" -W << EOF
dn: uid=alice,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
objectClass: organizationalPerson
objectClass: person
uid: alice
cn: Alice Smith
sn: Smith
mail: alice@example.com
userPassword: {SSHA}encrypted_password
EOF
```

### Listing Users

```bash
# Using oba CLI
oba user list --base "ou=users,dc=example,dc=com"

# Using ldapsearch
ldapsearch -x -H ldap://localhost:389 \
  -D "cn=admin,dc=example,dc=com" -W \
  -b "ou=users,dc=example,dc=com" \
  "(objectClass=inetOrgPerson)" cn mail uid
```

### Modifying Users

```bash
# Using ldapmodify
ldapmodify -x -H ldap://localhost:389 -D "cn=admin,dc=example,dc=com" -W << EOF
dn: uid=alice,ou=users,dc=example,dc=com
changetype: modify
replace: mail
mail: alice.smith@example.com
EOF
```

### Deleting Users

```bash
# Using oba CLI
oba user delete --dn "uid=alice,ou=users,dc=example,dc=com"

# Using ldapdelete
ldapdelete -x -H ldap://localhost:389 \
  -D "cn=admin,dc=example,dc=com" -W \
  "uid=alice,ou=users,dc=example,dc=com"
```

### Password Management

```bash
# Change user password
oba user passwd --dn "uid=alice,ou=users,dc=example,dc=com"

# Lock user account
oba user lock --dn "uid=alice,ou=users,dc=example,dc=com"

# Unlock user account
oba user unlock --dn "uid=alice,ou=users,dc=example,dc=com"
```

## Advanced LDAP Operations

### ModifyDN (Rename/Move Entry)

Rename an entry or move it to a different location in the directory tree:

```bash
# Rename an entry (change RDN)
ldapmodrdn -x -H ldap://localhost:389 \
  -D "cn=admin,dc=example,dc=com" -W \
  "uid=alice,ou=users,dc=example,dc=com" "uid=alice.smith"

# Move an entry to a different parent
ldapmodrdn -x -H ldap://localhost:389 \
  -D "cn=admin,dc=example,dc=com" -W \
  -s "ou=managers,dc=example,dc=com" \
  "uid=alice,ou=users,dc=example,dc=com" "uid=alice"
```

### Compare Operation

Compare an attribute value without retrieving the entry:

```bash
# Compare returns true (exit code 6) or false (exit code 5)
ldapcompare -x -H ldap://localhost:389 \
  -D "cn=admin,dc=example,dc=com" -W \
  "uid=alice,ou=users,dc=example,dc=com" "mail:alice@example.com"
```

### Extended Operations

#### Password Modify (RFC 3062)

Change a user's password using the Password Modify extended operation:

```bash
# Change own password
ldappasswd -x -H ldap://localhost:389 \
  -D "uid=alice,ou=users,dc=example,dc=com" -W \
  -S "uid=alice,ou=users,dc=example,dc=com"

# Admin changes user password
ldappasswd -x -H ldap://localhost:389 \
  -D "cn=admin,dc=example,dc=com" -W \
  -S "uid=alice,ou=users,dc=example,dc=com"
```

#### Who Am I (RFC 4532)

Check the current authenticated identity:

```bash
ldapwhoami -x -H ldap://localhost:389 \
  -D "uid=alice,ou=users,dc=example,dc=com" -W
# Returns: dn:uid=alice,ou=users,dc=example,dc=com
```

### Paged Search Results

For large result sets, use paged search to retrieve results in chunks:

```bash
# Search with page size of 100
ldapsearch -x -H ldap://localhost:389 \
  -D "cn=admin,dc=example,dc=com" -W \
  -b "ou=users,dc=example,dc=com" \
  -E pr=100/noprompt \
  "(objectClass=person)" cn mail
```

## Configuration Management

### Viewing Configuration

```bash
# Show effective configuration
oba config show --config /etc/oba/config.yaml

# Show in JSON format
oba config show --config /etc/oba/config.yaml --format json
```

### Validating Configuration

```bash
oba config validate --config /etc/oba/config.yaml
```

### Generating Default Configuration

```bash
oba config init > /etc/oba/config.yaml
```

### Applying Configuration Changes

Many configuration settings support hot reload without server restart.

#### Hot-Reloadable Settings

| Setting                     | Hot Reload | Method                  |
|-----------------------------|------------|-------------------------|
| `logging.level`             | Yes        | File watcher / REST API |
| `logging.format`            | Yes        | File watcher / REST API |
| `server.maxConnections`     | Yes        | File watcher / REST API |
| `server.readTimeout`        | Yes        | File watcher / REST API |
| `server.writeTimeout`       | Yes        | File watcher / REST API |
| `server.tlsCert/tlsKey`     | Yes        | File watcher / REST API |
| `security.rateLimit.*`      | Yes        | File watcher / REST API |
| `security.passwordPolicy.*` | Yes        | File watcher / REST API |
| `rest.rateLimit`            | Yes        | File watcher / REST API |
| `rest.tokenTTL`             | Yes        | File watcher / REST API |
| `rest.corsOrigins`          | Yes        | File watcher / REST API |
| `aclFile` (external)        | Yes        | File watcher / REST API |
| `server.address`            | No         | Requires restart        |
| `directory.*`               | No         | Requires restart        |
| `storage.*`                 | No         | Requires restart        |

#### Automatic Hot Reload (File Watcher)

When config file is specified, changes are detected automatically:

```bash
# Edit config file - changes applied within ~300ms
vim /etc/oba/config.yaml

# Check logs for reload confirmation
tail -f /var/log/oba/oba.log | grep "config"
# {"level":"info","msg":"config file changed, applying hot-reloadable settings"}
# {"level":"info","msg":"log level changed","old":"info","new":"debug"}
# {"level":"info","msg":"config reload completed"}
```

#### Hot Reload via REST API

Update configuration through REST API (requires admin authentication):

```bash
# Get JWT token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/bind \
  -H "Content-Type: application/json" \
  -d '{"dn":"cn=admin,dc=example,dc=com","password":"admin"}' \
  | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

# View current config
curl http://localhost:8080/api/v1/config \
  -H "Authorization: Bearer $TOKEN"

# Update log level
curl -X PATCH http://localhost:8080/api/v1/config/logging \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"level": "debug"}'

# Update server settings
curl -X PATCH http://localhost:8080/api/v1/config/server \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"maxConnections": 5000, "readTimeout": "60s"}'

# Update rate limit
curl -X PATCH http://localhost:8080/api/v1/config/security.ratelimit \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true, "maxAttempts": 3}'

# Save changes to file
curl -X POST http://localhost:8080/api/v1/config/save \
  -H "Authorization: Bearer $TOKEN"
```

#### Settings Requiring Restart

For non-hot-reloadable settings:

```bash
# Validate new configuration first
oba config validate --config /etc/oba/config.yaml

# Restart the server
sudo systemctl restart oba
```

### Reloading ACL Without Restart

If using an external ACL file (`aclFile` config option), ACL rules can be reloaded without restarting the server:

```bash
# Automatic reload: Edit the ACL file
# Changes are detected automatically within ~300ms

# Manual reload via CLI
oba reload acl

# Manual reload via signal
kill -SIGHUP $(cat /var/run/oba.pid)

# Reload via REST API
curl -X POST http://localhost:8080/api/v1/acl/reload \
  -H "Authorization: Bearer $TOKEN"
```

### ACL Management via REST API

Manage ACL rules dynamically through REST API:

```bash
# View current ACL
curl http://localhost:8080/api/v1/acl \
  -H "Authorization: Bearer $TOKEN"

# Add a new rule
curl -X POST http://localhost:8080/api/v1/acl/rules \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "rule": {
      "target": "ou=groups,dc=example,dc=com",
      "subject": "authenticated",
      "rights": ["read", "search"]
    }
  }'

# Update a rule
curl -X PUT http://localhost:8080/api/v1/acl/rules/1 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "target": "ou=users,dc=example,dc=com",
    "subject": "authenticated",
    "rights": ["read", "write", "search"]
  }'

# Delete a rule
curl -X DELETE http://localhost:8080/api/v1/acl/rules/2 \
  -H "Authorization: Bearer $TOKEN"

# Save ACL to file
curl -X POST http://localhost:8080/api/v1/acl/save \
  -H "Authorization: Bearer $TOKEN"
```

Check reload status in logs:

```bash
grep "ACL" /var/log/oba/oba.log | tail -5
# {"level":"info","msg":"ACL file changed, reloading",...}
# {"level":"info","msg":"ACL reloaded successfully","rules":5,...}
```

## Monitoring

### Log Analysis

```bash
# View recent logs (systemd)
sudo journalctl -u oba -f

# View log file
tail -f /var/log/oba/oba.log

# Parse JSON logs with jq
tail -f /var/log/oba/oba.log | jq '.'

# Filter by log level
tail -f /var/log/oba/oba.log | jq 'select(.level == "error")'
```

### Connection Monitoring

```bash
# Count active connections
netstat -an | grep :389 | grep ESTABLISHED | wc -l

# Monitor connection rate
watch -n 1 'netstat -an | grep :389 | grep ESTABLISHED | wc -l'
```

### Resource Monitoring

```bash
# Memory usage
ps aux | grep oba

# Detailed process info
top -p $(pidof oba)

# File descriptors
ls -la /proc/$(pidof oba)/fd | wc -l
```

## Maintenance Tasks

### Database Maintenance

The storage engine performs automatic maintenance, but you can trigger manual operations:

```bash
# Checkpoint (flush WAL to data files)
# This happens automatically based on checkpointInterval

# Compact database (reclaim space)
# Performed during backup operations
```

### Log Rotation

Configure logrotate for Oba logs. Create `/etc/logrotate.d/oba`:

```
/var/log/oba/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    create 0640 oba oba
    postrotate
        systemctl reload oba > /dev/null 2>&1 || true
    endscript
}
```

### Disk Space Management

Monitor disk usage for data and log directories:

```bash
# Check data directory size
du -sh /var/lib/oba

# Check log directory size
du -sh /var/log/oba

# Check WAL size
du -sh /var/lib/oba/wal
```

## Health Checks

### Basic Health Check

```bash
#!/bin/bash
# health-check.sh

LDAP_HOST="localhost"
LDAP_PORT="389"

if ldapsearch -x -H "ldap://${LDAP_HOST}:${LDAP_PORT}" -b "" -s base "(objectClass=*)" > /dev/null 2>&1; then
    echo "OK"
    exit 0
else
    echo "FAIL"
    exit 1
fi
```

### Comprehensive Health Check

```bash
#!/bin/bash
# comprehensive-health-check.sh

LDAP_HOST="localhost"
LDAP_PORT="389"
ADMIN_DN="cn=admin,dc=example,dc=com"
BASE_DN="dc=example,dc=com"

# Check LDAP connectivity
echo -n "LDAP connectivity: "
if ldapsearch -x -H "ldap://${LDAP_HOST}:${LDAP_PORT}" -b "" -s base > /dev/null 2>&1; then
    echo "OK"
else
    echo "FAIL"
    exit 1
fi

# Check process is running
echo -n "Process running: "
if pidof oba > /dev/null; then
    echo "OK"
else
    echo "FAIL"
    exit 1
fi

# Check disk space
echo -n "Disk space: "
USAGE=$(df /var/lib/oba | tail -1 | awk '{print $5}' | tr -d '%')
if [ "$USAGE" -lt 90 ]; then
    echo "OK (${USAGE}% used)"
else
    echo "WARNING (${USAGE}% used)"
fi

echo "All checks passed"
exit 0
```

## Scheduled Tasks

### Cron Jobs

Example crontab entries:

```cron
# Daily backup at 2 AM (timestamp added automatically)
0 2 * * * /usr/local/bin/oba backup --data-dir /var/lib/oba --output /backup/oba.bak --compress

# Weekly full backup on Sunday
0 3 * * 0 /usr/local/bin/oba backup --data-dir /var/lib/oba --output /backup/oba-full.bak --compress

# Daily incremental backup
0 2 * * 1-6 /usr/local/bin/oba backup --data-dir /var/lib/oba --incremental --output /backup/oba-incr.bak

# Health check every 5 minutes
*/5 * * * * /usr/local/bin/health-check.sh || /usr/local/bin/alert.sh
```

## Scaling Considerations

### Connection Limits

Adjust `maxConnections` based on expected load:

```yaml
server:
  maxConnections: 10000
```

Ensure system limits support the configured value:

```bash
# Check current limits
ulimit -n

# Set in systemd service
# LimitNOFILE=65536
```

### Buffer Pool Sizing

Adjust `bufferPoolSize` based on available memory:

```yaml
storage:
  bufferPoolSize: "1GB"  # For servers with 4GB+ RAM
```

### Checkpoint Interval

Balance between recovery time and write performance:

```yaml
storage:
  checkpointInterval: 5m  # Default, good for most workloads
```

- Shorter intervals: Faster recovery, more I/O overhead
- Longer intervals: Less I/O overhead, longer recovery time
