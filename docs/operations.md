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

Most configuration changes require a server restart:

```bash
# Validate new configuration first
oba config validate --config /etc/oba/config.yaml

# Restart the server
sudo systemctl restart oba
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
# Daily backup at 2 AM
0 2 * * * /usr/local/bin/oba backup --output /backup/oba-$(date +\%Y\%m\%d).bak --compress

# Weekly full backup on Sunday
0 3 * * 0 /usr/local/bin/oba backup --output /backup/oba-full-$(date +\%Y\%m\%d).bak --compress

# Daily incremental backup
0 2 * * 1-6 /usr/local/bin/oba backup --incremental --output /backup/oba-incr-$(date +\%Y\%m\%d).bak

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
