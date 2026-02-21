# Cluster Mode (High Availability)

Oba supports high availability clustering using the Raft consensus algorithm. This enables automatic leader election, data replication, and failover without external dependencies.

## Overview

The cluster mode provides:

- Automatic leader election
- Strong consistency (linearizable reads/writes)
- Automatic failover when leader fails
- Data replication across all nodes
- Log database replication for audit trails

## Architecture

```
                    ┌─────────────┐
                    │   HAProxy   │
                    │ (Optional)  │
                    └──────┬──────┘
                           │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
         ▼                 ▼                 ▼
   ┌───────────┐    ┌───────────┐    ┌───────────┐
   │   Node 1  │◄──►│   Node 2  │◄──►│   Node 3  │
   │  (Leader) │    │ (Follower)│    │ (Follower)│
   └───────────┘    └───────────┘    └───────────┘
         │                 │                 │
         ▼                 ▼                 ▼
   ┌───────────┐    ┌───────────┐    ┌───────────┐
   │  ObaDB    │    │  ObaDB    │    │  ObaDB    │
   │ (Primary) │    │ (Replica) │    │ (Replica) │
   └───────────┘    └───────────┘    └───────────┘
```

## Quick Start with Docker

The easiest way to run a cluster is using Docker Compose:

```bash
# Start 3-node cluster with HAProxy
docker compose -f docker-compose.cluster.yml up -d

# Check cluster status
curl http://localhost:8080/api/v1/cluster/status

# View logs
docker compose -f docker-compose.cluster.yml logs -f
```

Access points:
| Service              | Port | URL                      |
|----------------------|------|--------------------------|
| LDAP (load balanced) | 389  | ldap://localhost:389     |
| REST (load balanced) | 8080 | http://localhost:8080    |
| HAProxy Stats        | 8404 | http://localhost:8404    |
| Web Admin            | 3000 | http://localhost:3000    |
| Node 1 REST          | 8081 | http://localhost:8081    |
| Node 2 REST          | 8082 | http://localhost:8082    |
| Node 3 REST          | 8083 | http://localhost:8083    |

## Configuration

### Cluster Settings

```yaml
cluster:
  enabled: true
  nodeID: 1                    # Unique node ID (1, 2, 3, ...)
  raftAddr: "0.0.0.0:4445"     # Raft RPC listen address
  peers:                       # All cluster members
    - id: 1
      addr: "node1:4445"
    - id: 2
      addr: "node2:4445"
    - id: 3
      addr: "node3:4445"
  electionTimeout: 150ms       # Leader election timeout
  heartbeatTimeout: 50ms       # Heartbeat interval
  snapshotInterval: 10000      # Entries before snapshot
  dataDir: "/var/lib/oba/raft" # Raft data directory
```

### Node Configuration Example

Each node needs its own config file with unique `nodeID`:

Node 1 (`docker-cluster/node1/config.yaml`):
```yaml
cluster:
  enabled: true
  nodeID: 1
  raftAddr: "0.0.0.0:4445"
  peers:
    - id: 1
      addr: "oba-node1:4445"
    - id: 2
      addr: "oba-node2:4445"
    - id: 3
      addr: "oba-node3:4445"
```

Node 2 (`docker-cluster/node2/config.yaml`):
```yaml
cluster:
  enabled: true
  nodeID: 2
  # ... same peers list
```

## REST API Endpoints

### Cluster Status

```bash
GET /api/v1/cluster/status

# Response
{
  "enabled": true,
  "mode": "cluster",
  "nodeId": 1,
  "state": "leader",
  "term": 5,
  "leaderId": 1,
  "leaderAddr": "node1:4445",
  "commitIndex": 1234,
  "lastApplied": 1234,
  "peers": [
    {"id": 1, "addr": "node1:4445"},
    {"id": 2, "addr": "node2:4445"},
    {"id": 3, "addr": "node3:4445"}
  ]
}
```

### Health Check (HAProxy Compatible)

```bash
GET /api/v1/cluster/health

# Leader returns 200 OK
# Followers return 503 Service Unavailable
```

### Get Current Leader

```bash
GET /api/v1/cluster/leader

# Response
{
  "leaderId": 1,
  "leaderAddr": "node1:4445"
}
```

## HAProxy Configuration

The included HAProxy config routes writes to the leader and reads to any node:

```
frontend ldap_front
    bind *:389
    default_backend ldap_back

backend ldap_back
    balance roundrobin
    server node1 oba-node1:1389 check
    server node2 oba-node2:1389 check
    server node3 oba-node3:1389 check

frontend rest_front
    bind *:8080
    default_backend rest_back

backend rest_back
    balance roundrobin
    option httpchk GET /api/v1/cluster/health
    http-check expect status 200
    server node1 oba-node1:8080 check
    server node2 oba-node2:8080 check backup
    server node3 oba-node3:8080 check backup
```

## Operations

### Checking Cluster Health

```bash
# Check all nodes
for port in 8081 8082 8083; do
  echo "Node on port $port:"
  curl -s http://localhost:$port/api/v1/cluster/status | jq '{nodeId, state, term}'
done
```

### Failover Testing

```bash
# Stop the leader
docker compose -f docker-compose.cluster.yml stop oba-node1

# Watch new leader election (within ~300ms)
curl http://localhost:8082/api/v1/cluster/status

# Restart the node (rejoins as follower)
docker compose -f docker-compose.cluster.yml start oba-node1
```

### Adding Data

All write operations are automatically forwarded to the leader:

```bash
# Add entry via any node (HAProxy routes to leader)
ldapadd -x -H ldap://localhost:389 -D "cn=admin,dc=example,dc=com" -w admin <<EOF
dn: cn=testuser,dc=example,dc=com
objectClass: inetOrgPerson
cn: testuser
sn: User
mail: testuser@example.com
EOF

# Verify replication on all nodes
for port in 1389 2389 3389; do
  echo "Node on port $port:"
  ldapsearch -x -H ldap://localhost:$port -b "dc=example,dc=com" "(cn=testuser)" cn
done
```

## Raft Consensus Details

### Leader Election

1. Nodes start as followers with random election timeout (150-300ms)
2. If no heartbeat received, follower becomes candidate
3. Candidate requests votes from peers
4. Node with majority votes becomes leader
5. Leader sends heartbeats to maintain authority

### Log Replication

1. Client sends write request to leader
2. Leader appends to local log
3. Leader replicates to followers via AppendEntries RPC
4. Once majority acknowledges, entry is committed
5. Leader applies to state machine and responds to client

### Consistency Guarantees

- All writes go through the leader
- Reads from leader are linearizable
- Reads from followers may be slightly stale
- Committed entries are never lost (as long as majority survives)

## Troubleshooting

### Node Won't Join Cluster

```bash
# Check network connectivity
docker exec oba-oba-node1-1 ping oba-node2

# Verify Raft port is open
docker exec oba-oba-node1-1 nc -zv oba-node2 4445

# Check logs for errors
docker compose -f docker-compose.cluster.yml logs oba-node1
```

### Split Brain Prevention

Raft requires majority (N/2 + 1) for leader election:
- 3 nodes: needs 2 for quorum
- 5 nodes: needs 3 for quorum

If network partitions, only the partition with majority can elect a leader.

### Data Not Replicating

```bash
# Check commit index on all nodes
for port in 8081 8082 8083; do
  curl -s http://localhost:$port/api/v1/cluster/status | jq '{nodeId, commitIndex, lastApplied}'
done

# Verify leader is receiving writes
docker compose -f docker-compose.cluster.yml logs oba-node1 | grep "entry committed"
```

## Production Recommendations

1. Use odd number of nodes (3, 5, 7) for clear majority
2. Deploy nodes in different availability zones
3. Use dedicated network for Raft traffic
4. Monitor cluster health with `/api/v1/cluster/health`
5. Set up alerting for leader changes
6. Regular backup of Raft snapshots
7. Use TLS for Raft RPC in production (configure `raftTLSCert`, `raftTLSKey`)
