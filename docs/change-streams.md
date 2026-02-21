# Change Streams and Persistent Search

This document describes the real-time change notification features of Oba LDAP Server.

## Overview

Change Streams allow you to monitor entry changes (add, update, delete) in the LDAP directory in real-time. This feature can be used in two ways:

1. **LDAP Persistent Search** - For standard LDAP clients (RFC draft-ietf-ldapext-psearch)
2. **Go Internal API** - For applications using Oba as a library

## LDAP Persistent Search

### What is it?

Persistent Search is a control added to the LDAP protocol. It works like a normal search request, but:
- Results continue to flow as long as the connection remains open
- You receive notifications when new entries are added, existing entries are updated, or deleted
- Each change is sent as a SearchResultEntry

### Control Details

| Property    | Value                               |
|-------------|-------------------------------------|
| OID         | 2.16.840.1.113730.3.4.3             |
| Criticality | true or false                       |
| Value       | changeTypes, changesOnly, returnECs |

### Parameters

#### changeTypes (Bitmask)

Specifies which types of changes you want to monitor:

| Value | Meaning                |
|-------|------------------------|
| 1     | Add (new entry)        |
| 2     | Delete (deleted entry) |
| 4     | Modify (updated entry) |
| 8     | ModDN (DN change)      |
| 15    | All (1+2+4+8)          |

#### changesOnly

| Value | Meaning                                               |
|-------|-------------------------------------------------------|
| true  | Send only changes (don't send existing entries)       |
| false | First send existing entries, then monitor for changes |

#### returnECs (Entry Change Notification)

| Value | Meaning                                  |
|-------|------------------------------------------|
| true  | Send change information with each result |
| false | Send only the entry                      |

### Usage Examples

#### With ldapsearch

```bash
# Monitor all changes (changesOnly=false)
# ps=<changeTypes>/<changesOnly>/<returnECs>
ldapsearch -H ldap://localhost:1389 \
  -x -D "cn=admin,dc=example,dc=com" -w admin \
  -b "dc=example,dc=com" \
  -E 'ps=15/0/1' \
  "(objectClass=*)"

# Monitor only new changes (changesOnly=true)
ldapsearch -H ldap://localhost:1389 \
  -x -D "cn=admin,dc=example,dc=com" -w admin \
  -b "ou=users,dc=example,dc=com" \
  -E 'ps=15/1/1' \
  "(objectClass=*)"

# Monitor only adds and deletes (changeTypes=3 = add+delete)
ldapsearch -H ldap://localhost:1389 \
  -x -D "cn=admin,dc=example,dc=com" -w admin \
  -b "dc=example,dc=com" \
  -E 'ps=3/1/1' \
  "(objectClass=*)"

# Monitor only modify operations (changeTypes=4)
ldapsearch -H ldap://localhost:1389 \
  -x -D "cn=admin,dc=example,dc=com" -w admin \
  -b "dc=example,dc=com" \
  -E 'ps=4/1/1' \
  "(objectClass=*)"
```

**Parameter Explanation:** `ps=<changeTypes>/<changesOnly>/<returnECs>`
- changeTypes: 1=add, 2=delete, 4=modify, 8=modDN, 15=all
- changesOnly: 0=false (also send existing entries), 1=true
- returnECs: 0=false, 1=true (add Entry Change Notification control)

#### With Python ldap3

```python
from ldap3 import Server, Connection, SUBTREE
from ldap3.extend.standard.persistentSearch import PersistentSearch

server = Server('ldap://localhost:389')
conn = Connection(server, 'cn=admin,dc=example,dc=com', 'secret', auto_bind=True)

# Start persistent search
ps = PersistentSearch(
    conn,
    search_base='dc=example,dc=com',
    search_filter='(objectClass=*)',
    search_scope=SUBTREE,
    changes_only=True,
    notifications=True,
    streaming=True
)

# Listen for changes
for response in ps.listen():
    if response['type'] == 'searchResEntry':
        print(f"Entry: {response['dn']}")
        print(f"Change: {response.get('controls', {})}")
```

#### With Java JNDI

```java
import javax.naming.directory.*;
import javax.naming.ldap.*;

// Create control
byte[] controlValue = createPersistentSearchControlValue(15, true, true);
Control psControl = new BasicControl(
    "2.16.840.1.113730.3.4.3",  // OID
    true,                        // critical
    controlValue
);

// Perform search
LdapContext ctx = new InitialLdapContext(env, null);
ctx.setRequestControls(new Control[]{psControl});

NamingEnumeration<SearchResult> results = ctx.search(
    "dc=example,dc=com",
    "(objectClass=*)",
    searchControls
);

// Process results (blocking)
while (results.hasMore()) {
    SearchResult result = results.next();
    System.out.println("DN: " + result.getNameInNamespace());
}
```

### Entry Change Notification Control

When returnECs=1, a control is sent with each SearchResultEntry:

| Property     | Value                              |
|--------------|------------------------------------|
| OID          | 2.16.840.1.113730.3.4.7            |
| changeType   | 1=add, 2=delete, 4=modify, 8=modDN |
| previousDN   | Previous DN (only for modDN)       |
| changeNumber | Change sequence number             |

## Go Internal API

If you're using Oba as a library in your Go application, you can use the Change Streams API directly.

### Basic Usage

```go
package main

import (
    "fmt"
    "github.com/KilimcininKorOglu/oba/internal/backend"
    "github.com/KilimcininKorOglu/oba/internal/storage/stream"
)

func main() {
    // Access backend (from server setup)
    be := getBackend()

    // Watch all changes
    sub := be.Watch(stream.MatchAll())
    defer be.Unwatch(sub.ID)

    for event := range sub.Channel {
        fmt.Printf("[%s] %s\n", event.Operation, event.DN)
        if event.Entry != nil {
            fmt.Printf("  Attributes: %v\n", event.Entry.Attributes)
        }
    }
}
```

### Using Filters

```go
// Watch a specific DN
sub := be.Watch(stream.MatchDN("cn=admin,dc=example,dc=com"))

// Watch a subtree
sub := be.Watch(stream.MatchSubtree("ou=users,dc=example,dc=com"))

// Custom filter
sub := be.Watch(stream.WatchFilter{
    BaseDN: "ou=users,dc=example,dc=com",
    Scope:  stream.ScopeOneLevel,  // Only direct children
    Operations: []stream.OperationType{
        stream.OpInsert,
        stream.OpDelete,
    },
})
```

### Resume (Continuing)

You can resume from where you left off when the connection drops:

```go
// Save the last received token
var lastToken uint64

sub := be.Watch(stream.MatchAll())
for event := range sub.Channel {
    lastToken = event.Token
    processEvent(event)
}

// Connection dropped, reconnect
sub, err := be.WatchWithResume(stream.MatchAll(), lastToken)
if err == stream.ErrTokenTooOld {
    // Token too old, start from beginning
    sub = be.Watch(stream.MatchAll())
}
```

### Event Structure

```go
type ChangeEvent struct {
    Token     uint64           // Unique sequence number
    Operation OperationType    // OpInsert, OpUpdate, OpDelete, OpModifyDN
    DN        string           // DN of the affected entry
    Entry     *storage.Entry   // Entry data (nil for delete)
    OldDN     string           // Previous DN (only for modifyDN)
    Timestamp time.Time        // Event timestamp
}
```

### Backpressure

If the subscriber buffer fills up, new events are dropped:

```go
sub := be.Watch(stream.MatchAll())

// How many events were dropped?
dropped := sub.DroppedCount()
if dropped > 0 {
    log.Printf("Warning: %d events dropped", dropped)
}

// Reset drop counter
dropped = sub.ResetDropped()
```

## Performance

### Limits

| Parameter     | Default | Description                        |
|---------------|---------|------------------------------------|
| Buffer Size   | 256     | Event buffer size per subscriber   |
| Replay Buffer | 4096    | Number of events stored for resume |

### Recommendations

1. **Use changesOnly=true** - If you don't need existing entries
2. **Choose narrow scope** - Watch specific subtrees instead of entire directory
3. **Use filters** - Watch only entries you're interested in
4. **Process events quickly** - Prevent buffer overflow

## Troubleshooting

### "persistent search not supported" Error

Persistent Search control was sent as critical but the server doesn't support it. Make sure you're using the correct version of Oba.

### Events Not Arriving

1. Make sure you're using the correct BaseDN
2. Check the changeTypes value
3. Test with changesOnly=false (should see existing entries)

### Connection Dropping

1. Check timeout settings
2. Investigate network issues
3. Use the resume mechanism

## Related Documents

- [RFC draft-ietf-ldapext-psearch](https://tools.ietf.org/html/draft-ietf-ldapext-psearch-03) - Persistent Search Control
- [RFC 4533](https://tools.ietf.org/html/rfc4533) - LDAP Content Synchronization (syncrepl)
