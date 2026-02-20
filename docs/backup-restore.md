# Backup and Restore Guide

This guide covers backup strategies, procedures, and disaster recovery for Oba LDAP server.

## Backup Overview

Oba supports multiple backup methods:

| Method      | Description                              | Use Case                    |
|-------------|------------------------------------------|-----------------------------|
| Native      | Binary backup of database files          | Fast backup and restore     |
| LDIF        | LDAP Data Interchange Format export      | Portability, migration      |
| Incremental | Changes since last backup                | Frequent backups, less space|

## Backup Commands

### Full Backup (Native Format)

```bash
# Basic full backup
oba backup --output /backup/oba-full.bak

# Compressed backup
oba backup --output /backup/oba-full.bak.gz --compress

# Backup with timestamp
oba backup --output /backup/oba-$(date +%Y%m%d-%H%M%S).bak --compress
```

### Incremental Backup

```bash
# Incremental backup (changes since last full backup)
oba backup --incremental --output /backup/oba-incr.bak
```

### LDIF Export

```bash
# Export to LDIF format
oba backup --format ldif --output /backup/data.ldif

# Compressed LDIF export
oba backup --format ldif --output /backup/data.ldif.gz --compress
```

## Backup Strategies

### Daily Backup Strategy

Recommended for most deployments:

```bash
#!/bin/bash
# daily-backup.sh

BACKUP_DIR="/backup/oba"
DATE=$(date +%Y%m%d)
RETENTION_DAYS=30

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Create compressed backup
oba backup --output "$BACKUP_DIR/oba-$DATE.bak" --compress

# Remove old backups
find "$BACKUP_DIR" -name "oba-*.bak*" -mtime +$RETENTION_DAYS -delete

echo "Backup completed: $BACKUP_DIR/oba-$DATE.bak"
```

### Weekly Full + Daily Incremental Strategy

For larger deployments with frequent changes:

```bash
#!/bin/bash
# backup-strategy.sh

BACKUP_DIR="/backup/oba"
DATE=$(date +%Y%m%d)
DAY_OF_WEEK=$(date +%u)

mkdir -p "$BACKUP_DIR/full" "$BACKUP_DIR/incremental"

if [ "$DAY_OF_WEEK" -eq 7 ]; then
    # Sunday: Full backup
    oba backup --output "$BACKUP_DIR/full/oba-full-$DATE.bak" --compress
    
    # Clean old incremental backups after full backup
    rm -f "$BACKUP_DIR/incremental/"*.bak*
else
    # Weekdays: Incremental backup
    oba backup --incremental --output "$BACKUP_DIR/incremental/oba-incr-$DATE.bak" --compress
fi
```

### Continuous Backup with WAL Archiving

For point-in-time recovery:

```bash
#!/bin/bash
# archive-wal.sh

WAL_DIR="/var/lib/oba/wal"
ARCHIVE_DIR="/backup/oba/wal-archive"
DATE=$(date +%Y%m%d-%H%M%S)

mkdir -p "$ARCHIVE_DIR"

# Copy WAL files to archive
cp "$WAL_DIR"/*.oba "$ARCHIVE_DIR/wal-$DATE/"
```

## Restore Procedures

### Restore from Native Backup

```bash
# Stop the server first
sudo systemctl stop oba

# Restore from backup
oba restore --input /backup/oba-full.bak

# Verify the restore
oba config validate --config /etc/oba/config.yaml

# Start the server
sudo systemctl start oba
```

### Restore with Verification

```bash
# Restore with checksum verification
oba restore --input /backup/oba-full.bak --verify
```

### Restore from LDIF

```bash
# Stop the server
sudo systemctl stop oba

# Clear existing data (if needed)
rm -rf /var/lib/oba/*

# Restore from LDIF
oba restore --format ldif --input /backup/data.ldif

# Start the server
sudo systemctl start oba
```

### Restore Incremental Backups

Restore full backup first, then apply incrementals in order:

```bash
# Stop the server
sudo systemctl stop oba

# Restore full backup
oba restore --input /backup/full/oba-full-20260215.bak

# Apply incremental backups in chronological order
oba restore --input /backup/incremental/oba-incr-20260216.bak
oba restore --input /backup/incremental/oba-incr-20260217.bak
oba restore --input /backup/incremental/oba-incr-20260218.bak

# Start the server
sudo systemctl start oba
```

## Disaster Recovery

### Recovery Checklist

1. Assess the situation and identify the failure
2. Stop the Oba service if still running
3. Identify the most recent valid backup
4. Prepare the recovery environment
5. Restore from backup
6. Verify data integrity
7. Start the service
8. Validate functionality

### Complete Recovery Procedure

```bash
#!/bin/bash
# disaster-recovery.sh

BACKUP_FILE="$1"
DATA_DIR="/var/lib/oba"
CONFIG_FILE="/etc/oba/config.yaml"

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup-file>"
    exit 1
fi

echo "Starting disaster recovery..."

# Step 1: Stop the service
echo "Stopping Oba service..."
sudo systemctl stop oba

# Step 2: Backup current state (if any)
if [ -d "$DATA_DIR" ] && [ "$(ls -A $DATA_DIR)" ]; then
    echo "Backing up current state..."
    sudo mv "$DATA_DIR" "${DATA_DIR}.failed.$(date +%Y%m%d-%H%M%S)"
fi

# Step 3: Create fresh data directory
echo "Creating data directory..."
sudo mkdir -p "$DATA_DIR"
sudo chown oba:oba "$DATA_DIR"

# Step 4: Restore from backup
echo "Restoring from backup: $BACKUP_FILE"
oba restore --input "$BACKUP_FILE" --verify

if [ $? -ne 0 ]; then
    echo "ERROR: Restore failed!"
    exit 1
fi

# Step 5: Validate configuration
echo "Validating configuration..."
oba config validate --config "$CONFIG_FILE"

# Step 6: Start the service
echo "Starting Oba service..."
sudo systemctl start oba

# Step 7: Verify functionality
sleep 5
echo "Verifying LDAP connectivity..."
if ldapsearch -x -H ldap://localhost:389 -b "" -s base > /dev/null 2>&1; then
    echo "Recovery completed successfully!"
else
    echo "WARNING: Service started but LDAP check failed"
    exit 1
fi
```

### Point-in-Time Recovery

Point-in-time recovery allows restoring the database to a specific moment using WAL archives.

**Note:** This feature requires WAL archiving to be configured and running continuously.

```bash
# Restore to a specific point in time
# 1. Restore the most recent full backup before the target time
oba restore --input /backup/full/oba-full-20260215.bak

# 2. Apply WAL records up to the target time
oba restore --wal-dir /backup/wal-archive --target-time "2026-02-18T10:30:00Z"
```

**Limitations:**
- WAL replay is only available for changes after the last full backup
- Target time must be after the backup timestamp
- WAL files must be continuous (no gaps)

## Backup Verification

### Verify Backup Integrity

```bash
# Verify backup file integrity
oba restore --input /backup/oba-full.bak --verify --dry-run
```

### Test Restore Procedure

Periodically test your backup by restoring to a test environment:

```bash
#!/bin/bash
# test-restore.sh

TEST_DIR="/tmp/oba-restore-test"
BACKUP_FILE="$1"

# Create test environment
mkdir -p "$TEST_DIR"

# Restore to test directory
OBA_STORAGE_DATA_DIR="$TEST_DIR" oba restore --input "$BACKUP_FILE" --verify

# Verify data
if [ $? -eq 0 ]; then
    echo "Backup verification: PASSED"
    rm -rf "$TEST_DIR"
else
    echo "Backup verification: FAILED"
    exit 1
fi
```

## Backup Storage Best Practices

### Local Storage

- Store backups on a separate disk or partition
- Use RAID for redundancy
- Monitor disk space

### Remote Storage

```bash
# Copy backup to remote server
scp /backup/oba-full.bak backup-server:/backup/oba/

# Sync to S3-compatible storage
aws s3 cp /backup/oba-full.bak s3://my-bucket/oba-backups/

# Sync to remote directory
rsync -avz /backup/oba/ backup-server:/backup/oba/
```

### Backup Retention Policy

| Backup Type  | Retention Period | Storage Location |
|--------------|------------------|------------------|
| Daily        | 7 days           | Local            |
| Weekly       | 4 weeks          | Local + Remote   |
| Monthly      | 12 months        | Remote           |
| Yearly       | 7 years          | Archive          |

## Monitoring Backups

### Backup Status Check

```bash
#!/bin/bash
# check-backup-status.sh

BACKUP_DIR="/backup/oba"
MAX_AGE_HOURS=25

# Find most recent backup
LATEST=$(find "$BACKUP_DIR" -name "oba-*.bak*" -type f -printf '%T@ %p\n' | sort -n | tail -1 | cut -d' ' -f2)

if [ -z "$LATEST" ]; then
    echo "ERROR: No backup found!"
    exit 1
fi

# Check age
AGE_SECONDS=$(( $(date +%s) - $(stat -c %Y "$LATEST") ))
AGE_HOURS=$(( AGE_SECONDS / 3600 ))

if [ $AGE_HOURS -gt $MAX_AGE_HOURS ]; then
    echo "WARNING: Latest backup is $AGE_HOURS hours old"
    exit 1
fi

echo "OK: Latest backup is $AGE_HOURS hours old ($LATEST)"
```

### Alerting on Backup Failures

Integrate with your monitoring system:

```bash
# Add to cron with error handling
0 2 * * * /usr/local/bin/daily-backup.sh || /usr/local/bin/send-alert.sh "Oba backup failed"
```
