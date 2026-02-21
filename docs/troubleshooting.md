# Troubleshooting Guide

This guide helps diagnose and resolve common issues with Oba LDAP server.

## Diagnostic Commands

### Check Server Status

```bash
# Service status
sudo systemctl status oba

# Process status
ps aux | grep oba

# Port listening
netstat -tlnp | grep -E '389|636'
ss -tlnp | grep -E '389|636'
```

### Test LDAP Connectivity

```bash
# Basic connectivity test
ldapsearch -x -H ldap://localhost:389 -b "" -s base "(objectClass=*)"

# Test with authentication
ldapwhoami -x -H ldap://localhost:389 -D "cn=admin,dc=example,dc=com" -W

# Test TLS connection
ldapsearch -x -H ldaps://localhost:636 -b "" -s base "(objectClass=*)"
```

### View Logs

```bash
# Systemd journal
sudo journalctl -u oba -f

# Log file
tail -f /var/log/oba/oba.log

# Parse JSON logs
tail -f /var/log/oba/oba.log | jq '.'
```

## Common Issues

### Server Won't Start

#### Symptom
Server fails to start or exits immediately.

#### Possible Causes and Solutions

1. Port already in use

```bash
# Check what's using the port
sudo lsof -i :389
sudo netstat -tlnp | grep 389

# Solution: Stop the conflicting service or change Oba's port
```

2. Invalid configuration

```bash
# Validate configuration
oba config validate --config /etc/oba/config.yaml

# Check for syntax errors in YAML
```

3. Permission denied

```bash
# Check file permissions
ls -la /etc/oba/config.yaml
ls -la /var/lib/oba
ls -la /etc/oba/certs/

# Fix permissions
sudo chown -R oba:oba /var/lib/oba
sudo chmod 600 /etc/oba/config.yaml
```

4. Missing TLS certificates

```bash
# Verify certificate files exist
ls -la /etc/oba/certs/server.crt
ls -la /etc/oba/certs/server.key

# Verify certificate validity
openssl x509 -in /etc/oba/certs/server.crt -text -noout
```

### Connection Refused

#### Symptom
Clients cannot connect to the LDAP server.

#### Diagnostic Steps

```bash
# Check if server is listening
netstat -tlnp | grep oba

# Check firewall rules
sudo iptables -L -n
sudo ufw status

# Test local connection
ldapsearch -x -H ldap://127.0.0.1:389 -b "" -s base
```

#### Solutions

1. Server not running - Start the service
2. Firewall blocking - Add firewall rules
3. Wrong bind address - Check `server.address` configuration

### Authentication Failures

#### Symptom
Users cannot authenticate (bind) to the server.

#### Diagnostic Steps

```bash
# Check logs for bind failures
grep "bind" /var/log/oba/oba.log | grep -i "fail"

# Test bind with verbose output
ldapwhoami -x -H ldap://localhost:389 -D "cn=admin,dc=example,dc=com" -W -v
```

#### Common Causes

| Cause              | Solution                              |
|--------------------|---------------------------------------|
| Wrong password     | Verify password, reset if necessary   |
| Wrong DN format    | Check DN syntax and case sensitivity  |
| Account locked     | Unlock with `oba user unlock`         |
| Password expired   | Reset password with `oba user passwd` |
| ACL denying access | Review ACL configuration              |

### Account Locked Out

#### Symptom
User receives "account locked" error.

#### Solution

```bash
# Check lockout status in logs
grep "locked" /var/log/oba/oba.log | grep "uid=username"

# Unlock the account
oba user unlock --dn "uid=username,ou=users,dc=example,dc=com"
```

### TLS/SSL Issues

#### Certificate Errors

```bash
# Verify certificate chain
openssl verify -CAfile /etc/oba/certs/ca.crt /etc/oba/certs/server.crt

# Check certificate expiration
openssl x509 -in /etc/oba/certs/server.crt -noout -dates

# Test TLS connection
openssl s_client -connect localhost:636 -showcerts
```

#### Common TLS Errors

| Error                      | Cause                        | Solution                 |
|----------------------------|------------------------------|--------------------------|
| certificate has expired    | Certificate past expiry date | Renew certificate        |
| certificate verify failed  | CA not trusted               | Add CA to trust store    |
| no certificate provided    | Missing certificate config   | Configure tlsCert/tlsKey |
| private key does not match | Key/cert mismatch            | Regenerate key pair      |

### Search Returns No Results

#### Symptom
Search queries return empty results when data exists.

#### Diagnostic Steps

```bash
# Check base DN
ldapsearch -x -H ldap://localhost:389 -b "" -s base namingContexts

# Search with admin credentials
ldapsearch -x -H ldap://localhost:389 \
  -D "cn=admin,dc=example,dc=com" -W \
  -b "dc=example,dc=com" "(objectClass=*)"

# Check ACL permissions
grep "acl" /var/log/oba/oba.log | grep "denied"
```

#### Common Causes

| Cause            | Solution                              |
|------------------|---------------------------------------|
| Wrong base DN    | Verify baseDN in configuration        |
| ACL restrictions | Check ACL rules for search permission |
| Filter syntax    | Verify LDAP filter syntax             |
| Empty directory  | Add entries to the directory          |

### Performance Issues

#### Slow Queries

```bash
# Enable debug logging temporarily
# Set logging.level: "debug" in config

# Check query patterns in logs
grep "search" /var/log/oba/oba.log | jq '.duration_ms'

# Monitor system resources
top -p $(pidof oba)
iostat -x 1
```

#### High Memory Usage

```bash
# Check memory usage
ps aux | grep oba

# Review buffer pool configuration
# Reduce bufferPoolSize if necessary
```

#### Solutions

| Issue            | Solution                                  |
|------------------|-------------------------------------------|
| Slow searches    | Add indexes for frequently searched attrs |
| High memory      | Reduce bufferPoolSize                     |
| High disk I/O    | Increase checkpointInterval               |
| Many connections | Increase maxConnections, check for leaks  |

### Data Corruption

#### Symptom
Server reports data corruption or fails to start after crash.

#### Recovery Steps

```bash
# Stop the server
sudo systemctl stop oba

# Check WAL for recovery
ls -la /var/lib/oba/wal/

# Attempt automatic recovery (happens on startup)
sudo systemctl start oba

# If recovery fails, restore from backup
oba restore --input /backup/latest.bak --verify
```

## Error Messages Reference

### LDAP Result Codes

| Code | Name                      | Description                       |
|------|---------------------------|-----------------------------------|
| 0    | SUCCESS                   | Operation completed successfully  |
| 1    | OPERATIONS_ERROR          | Internal server error             |
| 2    | PROTOCOL_ERROR            | Protocol violation                |
| 3    | TIME_LIMIT_EXCEEDED       | Search time limit exceeded        |
| 4    | SIZE_LIMIT_EXCEEDED       | Search size limit exceeded        |
| 7    | AUTH_METHOD_NOT_SUPPORTED | Unsupported authentication method |
| 8    | STRONGER_AUTH_REQUIRED    | TLS required                      |
| 32   | NO_SUCH_OBJECT            | Entry does not exist              |
| 34   | INVALID_DN_SYNTAX         | Malformed DN                      |
| 49   | INVALID_CREDENTIALS       | Wrong password or DN              |
| 50   | INSUFFICIENT_ACCESS       | ACL denied access                 |
| 53   | UNWILLING_TO_PERFORM      | Server refuses operation          |
| 65   | OBJECT_CLASS_VIOLATION    | Schema violation                  |
| 68   | ENTRY_ALREADY_EXISTS      | DN already exists                 |

### Common Error Messages

| Message                   | Meaning                       | Solution                     |
|---------------------------|-------------------------------|------------------------------|
| "connection refused"      | Server not running or blocked | Start server, check firewall |
| "invalid credentials"     | Wrong password or DN          | Verify credentials           |
| "no such object"          | Entry doesn't exist           | Check DN spelling            |
| "insufficient access"     | ACL denied operation          | Review ACL rules             |
| "certificate has expired" | TLS cert expired              | Renew certificate            |
| "account locked"          | Too many failed attempts      | Unlock account               |
| "password expired"        | Password past maxAge          | Reset password               |

## Getting Help

### Collecting Diagnostic Information

When reporting issues, collect:

```bash
# Version information
oba version

# Configuration (sanitized)
oba config show --config /etc/oba/config.yaml | grep -v -i password

# Recent logs
tail -100 /var/log/oba/oba.log

# System information
uname -a
free -m
df -h /var/lib/oba
```

### Debug Mode

Enable debug logging for detailed troubleshooting:

```yaml
logging:
  level: "debug"
```

Remember to disable debug logging in production after troubleshooting.

## Docker Troubleshooting

### Container Won't Start

```bash
# Check container logs
docker compose logs oba

# Check container status
docker compose ps -a

# Inspect container
docker inspect oba-oba-1
```

### Common Docker Issues

| Issue                       | Cause                        | Solution                              |
|-----------------------------|------------------------------|---------------------------------------|
| Container exits immediately | Configuration error          | Check logs with `docker compose logs` |
| Port already in use         | Host port conflict           | Change port mapping in docker-compose |
| Permission denied on volume | Volume ownership mismatch    | Check volume permissions              |
| Cannot connect from host    | Wrong port or network config | Verify port mapping and network mode  |

### Volume Permissions

```bash
# Fix volume permissions
docker compose down
sudo chown -R 1000:1000 ./docker-data
docker compose up -d
```

### Rebuilding the Container

```bash
# Rebuild without cache
docker compose build --no-cache

# Restart with fresh build
docker compose up -d --build
```

## REST API Issues

### Cannot Connect to REST API

```bash
# Check if REST API is enabled in config
grep -A5 "rest:" /var/lib/oba/config.yaml

# Test REST API health endpoint
curl http://localhost:8080/api/v1/health

# Check if port is listening
netstat -tlnp | grep 8080
```

### Authentication Errors

```bash
# Get a new token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/bind \
  -H "Content-Type: application/json" \
  -d '{"dn":"cn=admin,dc=example,dc=com","password":"admin"}' \
  | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

# Test with token
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/stats
```

### CORS Errors

If browser shows CORS errors:

```yaml
# Add your frontend origin to config
rest:
  corsOrigins:
    - "http://localhost:3000"
    - "http://your-domain.com"
```

## Web Admin Panel Issues

### Cannot Access Web UI

```bash
# Check if web container is running
docker compose ps

# Check web container logs
docker compose logs web

# Verify port mapping
curl http://localhost:3000
```

### Dashboard Shows No Data

1. Check if REST API is accessible from web container
2. Verify authentication token is valid
3. Check browser console for errors

```bash
# Test REST API from web container
docker compose exec web wget -qO- http://oba:8080/api/v1/health
```

### Login Fails

1. Verify credentials are correct
2. Check if account is locked
3. Check REST API logs for errors

```bash
# Check for locked accounts
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/entries/uid%3Duser%2Cou%3Dusers%2Cdc%3Dexample%2Cdc%3Dcom/lock-status"
```
