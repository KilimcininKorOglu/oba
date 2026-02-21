# Installation Guide

This guide covers various methods to install and deploy Oba LDAP server.

## Overview

Oba is a zero-dependency LDAP server written in pure Go. It uses only the Go standard library, making it easy to build and deploy without external dependencies.

## System Requirements

### Hardware Requirements

| Component | Minimum | Recommended      |
|-----------|---------|------------------|
| CPU       | 1 core  | 2+ cores         |
| Memory    | 256 MB  | 1 GB+            |
| Disk      | 100 MB  | 1 GB+ (for data) |

### Software Requirements

- Go 1.22+ (for building from source)
- Linux, macOS (including Apple Silicon), or Windows
- Docker (optional, for containerized deployment)

### Supported Platforms

| Platform | Architecture  | Status    |
|----------|---------------|-----------|
| Linux    | amd64         | Supported |
| Linux    | arm64         | Supported |
| macOS    | amd64         | Supported |
| macOS    | arm64 (M1/M2) | Supported |
| Windows  | amd64         | Supported |

## Installation Methods

### Using Makefile (Recommended)

The easiest way to build Oba:

```bash
git clone https://github.com/KilimcininKorOglu/oba.git
cd oba

# Build the binary
make build

# The binary will be at bin/oba
./bin/oba version
```

Available Makefile targets:

| Target       | Description              |
|--------------|--------------------------|
| `make build` | Build the binary to bin/ |
| `make clean` | Remove build artifacts   |
| `make test`  | Run all tests            |
| `make bench` | Run benchmarks           |
| `make run`   | Build and run the server |

### Building from Source (Manual)

1. Clone the repository:

```bash
git clone https://github.com/KilimcininKorOglu/oba.git
cd oba
```

2. Build the binary:

```bash
go build -o bin/oba ./cmd/oba
```

3. (Optional) Build with version information:

```bash
go build -ldflags "-X main.version=1.0.0 -X main.commit=$(git rev-parse HEAD) -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/oba ./cmd/oba
```

4. Verify the installation:

```bash
./bin/oba version
```

### Installing as a System Service

#### Linux (systemd)

1. Copy the binary to a system location:

```bash
sudo cp oba /usr/local/bin/
sudo chmod +x /usr/local/bin/oba
```

2. Create a system user:

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin oba
```

3. Create required directories:

```bash
sudo mkdir -p /var/lib/oba /var/log/oba /etc/oba
sudo chown oba:oba /var/lib/oba /var/log/oba
```

4. Create the systemd service file `/etc/systemd/system/oba.service`:

```ini
[Unit]
Description=Oba LDAP Server
After=network.target

[Service]
Type=simple
User=oba
Group=oba
ExecStart=/usr/local/bin/oba serve --config /etc/oba/config.yaml
ExecReload=/bin/kill -SIGHUP $MAINPID
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

5. Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable oba
sudo systemctl start oba
```

6. Check the service status:

```bash
sudo systemctl status oba
```

7. Reload ACL without restart (if using external ACL file):

```bash
sudo systemctl reload oba
# or
oba reload acl
```

### Docker Installation

1. Create a `Dockerfile`:

```dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY . .
RUN go build -o oba ./cmd/oba

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/oba /usr/local/bin/

RUN adduser -D -H -s /sbin/nologin oba && \
    mkdir -p /var/lib/oba /var/log/oba && \
    chown -R oba:oba /var/lib/oba /var/log/oba

USER oba
EXPOSE 389 636

ENTRYPOINT ["oba"]
CMD ["serve", "--config", "/etc/oba/config.yaml"]
```

2. Build the Docker image:

```bash
docker build -t oba:latest .
```

3. Run the container:

```bash
docker run -d \
  --name oba \
  -p 389:389 \
  -p 636:636 \
  -v /path/to/config.yaml:/etc/oba/config.yaml:ro \
  -v oba-data:/var/lib/oba \
  oba:latest
```

### Docker Compose

The repository includes Docker Compose files for both standalone and cluster modes:

#### Standalone Mode

```bash
# Start single-node deployment
docker compose up -d

# Access points:
# - LDAP: ldap://localhost:1389
# - REST API: http://localhost:8080
# - Web Admin: http://localhost:3000
```

#### Cluster Mode (High Availability)

```bash
# Start 3-node Raft cluster with HAProxy
docker compose -f docker-compose.cluster.yml up -d

# Access points:
# - LDAP (load balanced): ldap://localhost:389
# - REST API (load balanced): http://localhost:8080
# - HAProxy Stats: http://localhost:8404
# - Web Admin: http://localhost:3000
# - Individual nodes: ports 8081-8083 (REST), 1389/2389/3389 (LDAP)
```

Configuration files:
- `docker-single/` - Standalone mode configs
- `docker-cluster/` - Cluster mode configs (node1-3, haproxy.cfg)

## Post-Installation Steps

### 1. Generate Configuration

```bash
oba config init > /etc/oba/config.yaml
```

### 2. Configure TLS (Recommended)

Generate or obtain TLS certificates and update the configuration:

```yaml
server:
  tlsCert: "/etc/oba/certs/server.crt"
  tlsKey: "/etc/oba/certs/server.key"
```

### 3. Set Directory Credentials

Edit the configuration to set your base DN and admin credentials:

```yaml
directory:
  baseDN: "dc=example,dc=com"
  rootDN: "cn=admin,dc=example,dc=com"
  rootPassword: "${OBA_ROOT_PASSWORD}"
```

### 4. Validate Configuration

```bash
oba config validate --config /etc/oba/config.yaml
```

### 5. Start the Server

```bash
oba serve --config /etc/oba/config.yaml
```

## Verifying the Installation

### Check Server Status

```bash
# Using ldapsearch
ldapsearch -x -H ldap://localhost:389 -b "" -s base "(objectClass=*)" namingContexts

# Check version
oba version
```

### Test Authentication

```bash
ldapwhoami -x -H ldap://localhost:389 -D "cn=admin,dc=example,dc=com" -W
```

## Upgrading

### From Binary

1. Stop the running server
2. Replace the binary with the new version
3. Start the server

```bash
sudo systemctl stop oba
sudo cp oba-new /usr/local/bin/oba
sudo systemctl start oba
```

### From Docker

```bash
docker-compose pull
docker-compose up -d
```

## Uninstallation

### Binary Installation

```bash
sudo systemctl stop oba
sudo systemctl disable oba
sudo rm /etc/systemd/system/oba.service
sudo systemctl daemon-reload
sudo rm /usr/local/bin/oba
sudo userdel oba
```

### Docker Installation

```bash
docker-compose down -v
docker rmi oba:latest
```
