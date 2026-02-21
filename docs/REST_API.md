# Oba REST API Documentation

This document provides comprehensive documentation for the Oba LDAP Server REST API. The REST API provides a JSON-based HTTP interface to perform LDAP operations, making it easier to integrate with web applications and services that don't have native LDAP support.

## Table of Contents

1. [Overview](#overview)
2. [Configuration](#configuration)
3. [Authentication](#authentication)
4. [Endpoints](#endpoints)
   - [Health Check](#health-check)
   - [Bind (Authentication)](#bind-authentication)
   - [Get Entry](#get-entry)
   - [Search](#search)
   - [Streaming Search](#streaming-search)
   - [Add Entry](#add-entry)
   - [Modify Entry](#modify-entry)
   - [Delete Entry](#delete-entry)
   - [Modify DN (Move/Rename)](#modify-dn-moverename)
   - [Compare](#compare)
   - [Bulk Operations](#bulk-operations)
   - [ACL Management](#acl-management)
   - [Config Management](#config-management)
   - [Log Management](#log-management)
5. [Error Handling](#error-handling)
6. [Rate Limiting](#rate-limiting)
7. [CORS Configuration](#cors-configuration)
8. [TLS/HTTPS Support](#tlshttps-support)

---

## Overview

The REST API runs as an optional HTTP server alongside the main LDAP server. It provides:

- JSON request/response format
- JWT and HTTP Basic authentication
- RESTful endpoints for all LDAP operations
- Pagination support for search results
- Streaming search with NDJSON format
- Bulk operations for batch processing
- Rate limiting per IP address
- CORS support for browser-based applications

### Base URL

All API endpoints are prefixed with `/api/v1/`. For example:

```
http://localhost:8080/api/v1/health
http://localhost:8080/api/v1/search
```

### Content Type

All requests and responses use JSON format:

```
Content-Type: application/json
```

Exception: Streaming search uses NDJSON (Newline Delimited JSON):

```
Content-Type: application/x-ndjson
```

---

## Configuration

Enable and configure the REST API in your `config.yaml`:

```yaml
rest:
  # Enable REST API
  enabled: true
  
  # HTTP listen address
  address: ":8080"
  
  # HTTPS listen address (optional, uses server.tlsCert and server.tlsKey)
  tlsAddress: ":8443"
  
  # JWT secret for token signing (required for JWT auth)
  # Generate with: openssl rand -hex 32
  jwtSecret: "your-secret-key-at-least-32-characters-long"
  
  # Token TTL (time-to-live)
  tokenTTL: 24h
  
  # Rate limit (requests per second per IP, 0 = disabled)
  rateLimit: 100
  
  # CORS allowed origins
  corsOrigins:
    - "*"
```

### Configuration Options

| Option        | Type     | Default | Description                                   |
|---------------|----------|---------|-----------------------------------------------|
| `enabled`     | bool     | `false` | Enable or disable the REST API                |
| `address`     | string   | `:8080` | HTTP listen address                           |
| `tlsAddress`  | string   | `""`    | HTTPS listen address (empty to disable)       |
| `jwtSecret`   | string   | `""`    | Secret key for JWT token signing              |
| `tokenTTL`    | duration | `24h`   | JWT token validity period                     |
| `rateLimit`   | int      | `100`   | Max requests per second per IP (0 = disabled) |
| `corsOrigins` | []string | `["*"]` | Allowed CORS origins                          |

---

## Authentication

The REST API supports two authentication methods:

### 1. JWT Bearer Token (Recommended)

First, obtain a token by authenticating with the bind endpoint:

```bash
curl -X POST http://localhost:8080/api/v1/auth/bind \
  -H "Content-Type: application/json" \
  -d '{"dn": "cn=admin,dc=example,dc=com", "password": "admin"}'
```

Response:

```json
{
  "success": true,
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

Then use the token in subsequent requests:

```bash
curl http://localhost:8080/api/v1/search?baseDN=dc=example,dc=com \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

#### JWT Token Structure

The JWT token contains the following claims:

| Claim | Description                          |
|-------|--------------------------------------|
| `dn`  | Distinguished Name of the bound user |
| `iat` | Issued At timestamp (Unix)           |
| `exp` | Expiration timestamp (Unix)          |

### 2. HTTP Basic Authentication

You can also use HTTP Basic Auth for each request:

```bash
curl http://localhost:8080/api/v1/search?baseDN=dc=example,dc=com \
  -u "cn=admin,dc=example,dc=com:admin"
```

Or with explicit header:

```bash
curl http://localhost:8080/api/v1/search?baseDN=dc=example,dc=com \
  -H "Authorization: Basic Y249YWRtaW4sZGM9ZXhhbXBsZSxkYz1jb206YWRtaW4="
```

### Public Endpoints (No Authentication Required)

The following endpoints do not require authentication:

- `GET /api/v1/health` - Health check
- `POST /api/v1/auth/bind` - Authentication

---

## Endpoints

### Health Check

Check the server status and get runtime statistics.

#### Request

```
GET /api/v1/health
```

No authentication required.

#### Response

```json
{
  "status": "ok",
  "version": "1.0.0",
  "uptime": "2h30m15s",
  "uptimeSecs": 9015,
  "startTime": "2024-01-15T10:30:00Z",
  "connections": 5,
  "requests": 1234
}
```

#### Response Fields

| Field         | Type   | Description                  |
|---------------|--------|------------------------------|
| `status`      | string | Server status (`ok`)         |
| `version`     | string | API version                  |
| `uptime`      | string | Human-readable uptime        |
| `uptimeSecs`  | int    | Uptime in seconds            |
| `startTime`   | string | Server start time (ISO 8601) |
| `connections` | int    | Current active connections   |
| `requests`    | int    | Total requests processed     |

#### Example

```bash
curl http://localhost:8080/api/v1/health
```

---

### Bind (Authentication)

Authenticate with LDAP credentials and obtain a JWT token.

#### Request

```
POST /api/v1/auth/bind
```

No authentication required.

#### Request Body

```json
{
  "dn": "cn=admin,dc=example,dc=com",
  "password": "admin"
}
```

| Field      | Type   | Required | Description                   |
|------------|--------|----------|-------------------------------|
| `dn`       | string | Yes      | Distinguished Name to bind as |
| `password` | string | Yes      | Password for authentication   |

#### Response (Success)

```json
{
  "success": true,
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJkbiI6ImNuPWFkbWluLGRjPWV4YW1wbGUsZGM9Y29tIiwiaWF0IjoxNzA1MzEyMjAwLCJleHAiOjE3MDUzOTg2MDB9.signature"
}
```

#### Response (Failure)

```json
{
  "success": false,
  "message": "invalid credentials"
}
```

#### Example

```bash
curl -X POST http://localhost:8080/api/v1/auth/bind \
  -H "Content-Type: application/json" \
  -d '{
    "dn": "cn=admin,dc=example,dc=com",
    "password": "admin"
  }'
```

---

### Get Entry

Retrieve a single LDAP entry by its DN.

#### Request

```
GET /api/v1/entries/{dn}
```

The `{dn}` parameter must be URL-encoded.

#### Response

```json
{
  "dn": "cn=john,ou=users,dc=example,dc=com",
  "attributes": {
    "cn": ["john"],
    "sn": ["Doe"],
    "mail": ["john@example.com"],
    "objectClass": ["person", "inetOrgPerson"]
  }
}
```

#### Example

```bash
# Get entry for cn=john,ou=users,dc=example,dc=com
curl "http://localhost:8080/api/v1/entries/cn%3Djohn%2Cou%3Dusers%2Cdc%3Dexample%2Cdc%3Dcom" \
  -H "Authorization: Bearer $TOKEN"
```

---

### Search

Search for LDAP entries with pagination and attribute filtering.

#### Request

```
GET /api/v1/search
```

#### Query Parameters

| Parameter    | Type   | Default | Description                                  |
|--------------|--------|---------|----------------------------------------------|
| `baseDN`     | string | -       | Base DN for the search (required)            |
| `scope`      | string | `sub`   | Search scope: `base`, `one`, or `sub`        |
| `filter`     | string | -       | LDAP filter (not yet implemented)            |
| `attributes` | string | -       | Comma-separated list of attributes to return |
| `offset`     | int    | `0`     | Number of entries to skip (pagination)       |
| `limit`      | int    | `0`     | Maximum entries to return (0 = unlimited)    |
| `timeLimit`  | int    | `0`     | Search timeout in seconds (0 = no limit)     |

#### Search Scopes

| Scope  | Description                                 |
|--------|---------------------------------------------|
| `base` | Search only the base DN entry               |
| `one`  | Search one level below the base DN          |
| `sub`  | Search the entire subtree below the base DN |

#### Response

```json
{
  "entries": [
    {
      "dn": "cn=john,ou=users,dc=example,dc=com",
      "attributes": {
        "cn": ["john"],
        "sn": ["Doe"],
        "mail": ["john@example.com"]
      }
    },
    {
      "dn": "cn=jane,ou=users,dc=example,dc=com",
      "attributes": {
        "cn": ["jane"],
        "sn": ["Smith"],
        "mail": ["jane@example.com"]
      }
    }
  ],
  "totalCount": 50,
  "offset": 0,
  "limit": 10,
  "hasMore": true
}
```

#### Response Fields

| Field        | Type  | Description                             |
|--------------|-------|-----------------------------------------|
| `entries`    | array | Array of matching entries               |
| `totalCount` | int   | Total number of matching entries        |
| `offset`     | int   | Current offset                          |
| `limit`      | int   | Current limit                           |
| `hasMore`    | bool  | Whether more entries exist beyond limit |

#### Examples

Basic search:

```bash
curl "http://localhost:8080/api/v1/search?baseDN=dc=example,dc=com&scope=sub" \
  -H "Authorization: Bearer $TOKEN"
```

With pagination:

```bash
curl "http://localhost:8080/api/v1/search?baseDN=dc=example,dc=com&offset=10&limit=10" \
  -H "Authorization: Bearer $TOKEN"
```

With attribute filtering:

```bash
curl "http://localhost:8080/api/v1/search?baseDN=dc=example,dc=com&attributes=cn,mail,sn" \
  -H "Authorization: Bearer $TOKEN"
```

With time limit:

```bash
curl "http://localhost:8080/api/v1/search?baseDN=dc=example,dc=com&timeLimit=5" \
  -H "Authorization: Bearer $TOKEN"
```

---

### Streaming Search

Stream search results as NDJSON (Newline Delimited JSON) for large result sets. Each entry is sent as a separate JSON object on its own line, allowing clients to process results incrementally.

#### Request

```
GET /api/v1/search/stream
```

#### Query Parameters

| Parameter | Type   | Default | Description                           |
|-----------|--------|---------|---------------------------------------|
| `baseDN`  | string | -       | Base DN for the search (required)     |
| `scope`   | string | `sub`   | Search scope: `base`, `one`, or `sub` |
| `filter`  | string | -       | LDAP filter (not yet implemented)     |

#### Response Headers

```
Content-Type: application/x-ndjson
Transfer-Encoding: chunked
X-Content-Type-Options: nosniff
```

#### Response Format

Each line is a separate JSON object:

```
{"dn":"cn=john,ou=users,dc=example,dc=com","attributes":{"cn":["john"],"sn":["Doe"]}}
{"dn":"cn=jane,ou=users,dc=example,dc=com","attributes":{"cn":["jane"],"sn":["Smith"]}}
{"done":true,"count":2}
```

The final line contains `"done": true` and the total count.

#### Example

```bash
curl "http://localhost:8080/api/v1/search/stream?baseDN=dc=example,dc=com&scope=sub" \
  -H "Authorization: Bearer $TOKEN"
```

#### Processing NDJSON in Different Languages

JavaScript:

```javascript
const response = await fetch('/api/v1/search/stream?baseDN=dc=example,dc=com', {
  headers: { 'Authorization': `Bearer ${token}` }
});

const reader = response.body.getReader();
const decoder = new TextDecoder();
let buffer = '';

while (true) {
  const { done, value } = await reader.read();
  if (done) break;
  
  buffer += decoder.decode(value, { stream: true });
  const lines = buffer.split('\n');
  buffer = lines.pop();
  
  for (const line of lines) {
    if (line.trim()) {
      const entry = JSON.parse(line);
      if (entry.done) {
        console.log(`Total entries: ${entry.count}`);
      } else {
        console.log(`Entry: ${entry.dn}`);
      }
    }
  }
}
```

Python:

```python
import requests
import json

response = requests.get(
    'http://localhost:8080/api/v1/search/stream',
    params={'baseDN': 'dc=example,dc=com'},
    headers={'Authorization': f'Bearer {token}'},
    stream=True
)

for line in response.iter_lines():
    if line:
        entry = json.loads(line)
        if entry.get('done'):
            print(f"Total entries: {entry['count']}")
        else:
            print(f"Entry: {entry['dn']}")
```

---

### Add Entry

Create a new LDAP entry.

#### Request

```
POST /api/v1/entries
```

#### Request Body

```json
{
  "dn": "cn=newuser,ou=users,dc=example,dc=com",
  "attributes": {
    "objectClass": ["person", "inetOrgPerson"],
    "cn": ["newuser"],
    "sn": ["User"],
    "mail": ["newuser@example.com"]
  }
}
```

| Field        | Type   | Required | Description                     |
|--------------|--------|----------|---------------------------------|
| `dn`         | string | Yes      | Distinguished Name for entry    |
| `attributes` | object | Yes      | Map of attribute name to values |

#### Response

HTTP Status: `201 Created`

```json
{
  "dn": "cn=newuser,ou=users,dc=example,dc=com",
  "attributes": {
    "objectClass": ["person", "inetOrgPerson"],
    "cn": ["newuser"],
    "sn": ["User"],
    "mail": ["newuser@example.com"],
    "createTimestamp": ["20240115T103000Z"],
    "creatorsName": ["cn=admin,dc=example,dc=com"]
  }
}
```

The response includes a `Location` header with the URL of the new entry.

#### Example

```bash
curl -X POST http://localhost:8080/api/v1/entries \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "dn": "cn=newuser,ou=users,dc=example,dc=com",
    "attributes": {
      "objectClass": ["person", "inetOrgPerson"],
      "cn": ["newuser"],
      "sn": ["User"],
      "mail": ["newuser@example.com"]
    }
  }'
```

---

### Modify Entry

Modify an existing LDAP entry. Supports both PUT and PATCH methods.

#### Request

```
PUT /api/v1/entries/{dn}
PATCH /api/v1/entries/{dn}
```

The `{dn}` parameter must be URL-encoded.

#### Request Body

```json
{
  "changes": [
    {
      "operation": "replace",
      "attribute": "mail",
      "values": ["newemail@example.com"]
    },
    {
      "operation": "add",
      "attribute": "telephoneNumber",
      "values": ["+1-555-1234"]
    },
    {
      "operation": "delete",
      "attribute": "description",
      "values": []
    }
  ]
}
```

#### Modification Operations

| Operation | Description                                           |
|-----------|-------------------------------------------------------|
| `add`     | Add values to an attribute (creates if not exists)    |
| `delete`  | Delete values from an attribute (or entire attribute) |
| `replace` | Replace all values of an attribute                    |

#### Change Object Fields

| Field       | Type     | Required | Description                                |
|-------------|----------|----------|--------------------------------------------|
| `operation` | string   | Yes      | Operation type: `add`, `delete`, `replace` |
| `attribute` | string   | Yes      | Attribute name to modify                   |
| `values`    | []string | Yes      | Values to add/delete/replace               |

#### Response

HTTP Status: `200 OK`

Returns the modified entry:

```json
{
  "dn": "cn=john,ou=users,dc=example,dc=com",
  "attributes": {
    "cn": ["john"],
    "mail": ["newemail@example.com"],
    "telephoneNumber": ["+1-555-1234"],
    "modifyTimestamp": ["20240115T110000Z"],
    "modifiersName": ["cn=admin,dc=example,dc=com"]
  }
}
```

#### Examples

Replace an attribute:

```bash
curl -X PATCH "http://localhost:8080/api/v1/entries/cn%3Djohn%2Cou%3Dusers%2Cdc%3Dexample%2Cdc%3Dcom" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "changes": [
      {"operation": "replace", "attribute": "mail", "values": ["john.doe@example.com"]}
    ]
  }'
```

Add multiple values:

```bash
curl -X PATCH "http://localhost:8080/api/v1/entries/cn%3Djohn%2Cou%3Dusers%2Cdc%3Dexample%2Cdc%3Dcom" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "changes": [
      {"operation": "add", "attribute": "telephoneNumber", "values": ["+1-555-1234", "+1-555-5678"]}
    ]
  }'
```

Delete an attribute:

```bash
curl -X PATCH "http://localhost:8080/api/v1/entries/cn%3Djohn%2Cou%3Dusers%2Cdc%3Dexample%2Cdc%3Dcom" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "changes": [
      {"operation": "delete", "attribute": "description", "values": []}
    ]
  }'
```

---

### Delete Entry

Delete an LDAP entry.

#### Request

```
DELETE /api/v1/entries/{dn}
```

The `{dn}` parameter must be URL-encoded.

#### Response

HTTP Status: `204 No Content`

No response body is returned on successful deletion.

#### Example

```bash
curl -X DELETE "http://localhost:8080/api/v1/entries/cn%3Djohn%2Cou%3Dusers%2Cdc%3Dexample%2Cdc%3Dcom" \
  -H "Authorization: Bearer $TOKEN"
```

#### Error Response

If the entry has children (non-leaf entry):

```json
{
  "error": "not_allowed_on_non_leaf",
  "code": 409,
  "message": "operation not allowed on non-leaf entry"
}
```

---

### Modify DN (Move/Rename)

Rename an entry or move it to a different location in the directory tree.

#### Request

```
POST /api/v1/entries/{dn}/move
```

The `{dn}` parameter must be URL-encoded.

#### Request Body

```json
{
  "newRDN": "cn=newname",
  "deleteOldRDN": true,
  "newSuperior": "ou=newparent,dc=example,dc=com"
}
```

| Field          | Type   | Required | Description                                     |
|----------------|--------|----------|-------------------------------------------------|
| `newRDN`       | string | Yes      | New Relative Distinguished Name                 |
| `deleteOldRDN` | bool   | No       | Delete old RDN attribute value (default: false) |
| `newSuperior`  | string | No       | New parent DN (for moving entry)                |

#### Response

HTTP Status: `200 OK`

Returns the entry with its new DN:

```json
{
  "dn": "cn=newname,ou=newparent,dc=example,dc=com",
  "attributes": {
    "cn": ["newname"],
    "sn": ["User"],
    "mail": ["user@example.com"]
  }
}
```

The response includes a `Location` header with the URL of the entry at its new location.

#### Examples

Rename an entry:

```bash
curl -X POST "http://localhost:8080/api/v1/entries/cn%3Djohn%2Cou%3Dusers%2Cdc%3Dexample%2Cdc%3Dcom/move" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "newRDN": "cn=john.doe",
    "deleteOldRDN": true
  }'
```

Move an entry to a different OU:

```bash
curl -X POST "http://localhost:8080/api/v1/entries/cn%3Djohn%2Cou%3Dusers%2Cdc%3Dexample%2Cdc%3Dcom/move" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "newRDN": "cn=john",
    "newSuperior": "ou=managers,dc=example,dc=com"
  }'
```

---

### Compare

Compare an attribute value in an entry.

#### Request

```
POST /api/v1/compare
```

#### Request Body

```json
{
  "dn": "cn=john,ou=users,dc=example,dc=com",
  "attribute": "mail",
  "value": "john@example.com"
}
```

| Field       | Type   | Required | Description                |
|-------------|--------|----------|----------------------------|
| `dn`        | string | Yes      | DN of the entry to compare |
| `attribute` | string | Yes      | Attribute name to compare  |
| `value`     | string | Yes      | Value to compare against   |

#### Response

```json
{
  "match": true
}
```

| Field   | Type | Description                                |
|---------|------|--------------------------------------------|
| `match` | bool | `true` if value matches, `false` otherwise |

#### Example

```bash
curl -X POST http://localhost:8080/api/v1/compare \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "dn": "cn=john,ou=users,dc=example,dc=com",
    "attribute": "mail",
    "value": "john@example.com"
  }'
```

---

### Bulk Operations

Perform multiple LDAP operations in a single request. Useful for batch processing and data migration.

#### Request

```
POST /api/v1/bulk
```

#### Request Body

```json
{
  "stopOnError": false,
  "operations": [
    {
      "operation": "add",
      "dn": "cn=user1,ou=users,dc=example,dc=com",
      "attributes": {
        "objectClass": ["person"],
        "cn": ["user1"],
        "sn": ["User One"]
      }
    },
    {
      "operation": "add",
      "dn": "cn=user2,ou=users,dc=example,dc=com",
      "attributes": {
        "objectClass": ["person"],
        "cn": ["user2"],
        "sn": ["User Two"]
      }
    },
    {
      "operation": "modify",
      "dn": "cn=existing,ou=users,dc=example,dc=com",
      "changes": [
        {"operation": "replace", "attribute": "mail", "values": ["new@example.com"]}
      ]
    },
    {
      "operation": "delete",
      "dn": "cn=olduser,ou=users,dc=example,dc=com"
    }
  ]
}
```

#### Request Fields

| Field         | Type  | Required | Description                                     |
|---------------|-------|----------|-------------------------------------------------|
| `stopOnError` | bool  | No       | Stop processing on first error (default: false) |
| `operations`  | array | Yes      | Array of operations to perform                  |

#### Operation Types

| Operation | Required Fields    | Description        |
|-----------|--------------------|--------------------|
| `add`     | `dn`, `attributes` | Create a new entry |
| `modify`  | `dn`, `changes`    | Modify an entry    |
| `delete`  | `dn`               | Delete an entry    |

#### Response

HTTP Status varies based on results:
- `200 OK` - All operations succeeded
- `207 Multi-Status` - Some operations failed
- `400 Bad Request` - All operations failed

```json
{
  "success": true,
  "totalCount": 4,
  "succeeded": 4,
  "failed": 0,
  "results": [
    {
      "index": 0,
      "dn": "cn=user1,ou=users,dc=example,dc=com",
      "operation": "add",
      "success": true
    },
    {
      "index": 1,
      "dn": "cn=user2,ou=users,dc=example,dc=com",
      "operation": "add",
      "success": true
    },
    {
      "index": 2,
      "dn": "cn=existing,ou=users,dc=example,dc=com",
      "operation": "modify",
      "success": true
    },
    {
      "index": 3,
      "dn": "cn=olduser,ou=users,dc=example,dc=com",
      "operation": "delete",
      "success": true
    }
  ]
}
```

#### Response Fields

| Field        | Type  | Description                        |
|--------------|-------|------------------------------------|
| `success`    | bool  | `true` if all operations succeeded |
| `totalCount` | int   | Total number of operations         |
| `succeeded`  | int   | Number of successful operations    |
| `failed`     | int   | Number of failed operations        |
| `results`    | array | Detailed result for each operation |

#### Result Object Fields

| Field        | Type   | Description                  |
|--------------|--------|------------------------------|
| `index`      | int    | Operation index (0-based)    |
| `dn`         | string | DN of the entry              |
| `operation`  | string | Operation type               |
| `success`    | bool   | Whether operation succeeded  |
| `error`      | string | Error message (if failed)    |
| `resultCode` | int    | LDAP result code (if failed) |

#### Example with Mixed Results

Request:

```bash
curl -X POST http://localhost:8080/api/v1/bulk \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "stopOnError": false,
    "operations": [
      {
        "operation": "add",
        "dn": "cn=newuser,ou=users,dc=example,dc=com",
        "attributes": {
          "objectClass": ["person"],
          "cn": ["newuser"],
          "sn": ["New User"]
        }
      },
      {
        "operation": "delete",
        "dn": "cn=nonexistent,ou=users,dc=example,dc=com"
      }
    ]
  }'
```

Response (207 Multi-Status):

```json
{
  "success": false,
  "totalCount": 2,
  "succeeded": 1,
  "failed": 1,
  "results": [
    {
      "index": 0,
      "dn": "cn=newuser,ou=users,dc=example,dc=com",
      "operation": "add",
      "success": true
    },
    {
      "index": 1,
      "dn": "cn=nonexistent,ou=users,dc=example,dc=com",
      "operation": "delete",
      "success": false,
      "error": "entry not found",
      "resultCode": 32
    }
  ]
}
```

#### Stop on Error Behavior

When `stopOnError` is `true`, processing stops at the first error and remaining operations are marked as skipped:

```json
{
  "success": false,
  "totalCount": 3,
  "succeeded": 1,
  "failed": 2,
  "results": [
    {
      "index": 0,
      "dn": "cn=user1,dc=example,dc=com",
      "operation": "add",
      "success": true
    },
    {
      "index": 1,
      "dn": "cn=duplicate,dc=example,dc=com",
      "operation": "add",
      "success": false,
      "error": "entry already exists",
      "resultCode": 68
    },
    {
      "index": 2,
      "dn": "cn=user3,dc=example,dc=com",
      "operation": "add",
      "success": false,
      "error": "skipped due to previous error"
    }
  ]
}
```

---

### ACL Management

Manage Access Control List (ACL) rules via REST API. These endpoints require admin authentication (rootDN).

#### Get ACL Configuration

Retrieve the complete ACL configuration including rules and statistics.

```
GET /api/v1/acl
```

Response:

```json
{
  "defaultPolicy": "deny",
  "rules": [
    {
      "target": "*",
      "subject": "cn=admin,dc=example,dc=com",
      "scope": "subtree",
      "rights": ["all"],
      "deny": false
    },
    {
      "target": "dc=example,dc=com",
      "subject": "authenticated",
      "scope": "subtree",
      "rights": ["read", "search"],
      "deny": false
    }
  ],
  "stats": {
    "ruleCount": 2,
    "lastReload": "2026-02-21T10:00:00Z",
    "reloadCount": 5,
    "filePath": "/var/lib/oba/acl.yaml"
  }
}
```

#### List ACL Rules

```
GET /api/v1/acl/rules
```

Response:

```json
{
  "rules": [...],
  "count": 3
}
```

#### Get Single Rule

```
GET /api/v1/acl/rules/{index}
```

Response:

```json
{
  "target": "dc=example,dc=com",
  "subject": "authenticated",
  "scope": "subtree",
  "rights": ["read", "search"],
  "deny": false
}
```

#### Add ACL Rule

```
POST /api/v1/acl/rules
```

Request:

```json
{
  "rule": {
    "target": "ou=users,dc=example,dc=com",
    "subject": "cn=readonly,dc=example,dc=com",
    "scope": "subtree",
    "rights": ["read", "search"],
    "deny": false
  },
  "index": -1
}
```

The `index` field is optional. Use `-1` or omit to append at the end.

Response:

```json
{
  "message": "rule added",
  "rule": {...}
}
```

#### Update ACL Rule

```
PUT /api/v1/acl/rules/{index}
```

Request:

```json
{
  "target": "ou=users,dc=example,dc=com",
  "subject": "authenticated",
  "scope": "subtree",
  "rights": ["read", "write", "search"],
  "deny": false
}
```

#### Delete ACL Rule

```
DELETE /api/v1/acl/rules/{index}
```

Response:

```json
{
  "message": "rule deleted"
}
```

#### Set Default Policy

```
PUT /api/v1/acl/default
```

Request:

```json
{
  "policy": "deny"
}
```

Valid values: `allow`, `deny`

#### Reload ACL from File

```
POST /api/v1/acl/reload
```

Response:

```json
{
  "message": "ACL reloaded",
  "ruleCount": 3,
  "reloadCount": 6
}
```

#### Save ACL to File

```
POST /api/v1/acl/save
```

Response:

```json
{
  "message": "ACL saved",
  "filePath": "/var/lib/oba/acl.yaml"
}
```

#### Validate ACL Configuration

```
POST /api/v1/acl/validate
```

Request:

```json
{
  "defaultPolicy": "deny",
  "rules": [
    {
      "target": "dc=example,dc=com",
      "subject": "authenticated",
      "scope": "subtree",
      "rights": ["read"]
    }
  ]
}
```

Response (valid):

```json
{
  "valid": true,
  "message": "ACL configuration is valid"
}
```

Response (invalid):

```json
{
  "valid": false,
  "errors": ["rule 0: missing subject"]
}
```

#### ACL Rule Fields

| Field        | Type     | Required | Description                                                                 |
|--------------|----------|----------|-----------------------------------------------------------------------------|
| `target`     | string   | Yes      | DN pattern this rule applies to (`*` for all)                               |
| `subject`    | string   | Yes      | Who this rule applies to (DN, `authenticated`, `anonymous`, `self`)         |
| `scope`      | string   | No       | `base`, `one`, or `subtree` (default: `subtree`)                            |
| `rights`     | []string | Yes      | Access rights: `read`, `write`, `add`, `delete`, `search`, `compare`, `all` |
| `attributes` | []string | No       | Specific attributes (empty = all)                                           |
| `deny`       | bool     | No       | `true` for deny rule, `false` for allow                                     |

---

### Config Management

Manage server configuration via REST API. These endpoints require admin authentication (rootDN). Sensitive data (passwords, secrets) are masked in responses.

#### Get Full Configuration

```
GET /api/v1/config
```

Response:

```json
{
  "server": {
    "address": ":1389",
    "maxConnections": 1000,
    "readTimeout": "30s",
    "writeTimeout": "30s"
  },
  "logging": {
    "level": "info",
    "format": "json",
    "output": "stdout"
  },
  "security": {
    "rateLimit": {
      "enabled": true,
      "maxAttempts": 5,
      "lockoutDuration": "15m"
    },
    "passwordPolicy": {
      "enabled": true,
      "minLength": 8,
      "requireUppercase": true,
      "requireLowercase": true,
      "requireDigit": true,
      "requireSpecial": false
    },
    "encryption": {
      "enabled": true,
      "keyFile": "/var/lib/oba/encryption.key"
    }
  },
  "rest": {
    "enabled": true,
    "address": ":8080",
    "rateLimit": 100,
    "tokenTTL": "24h",
    "corsOrigins": ["*"],
    "jwtSecret": "********"
  },
  "storage": {
    "dataDir": "/var/lib/oba",
    "pageSize": 4096,
    "bufferPoolSize": "256MB",
    "checkpointInterval": "5m"
  }
}
```

#### Get Config Section

```
GET /api/v1/config/{section}
```

Available sections: `server`, `logging`, `security`, `rest`, `storage`

Example:

```bash
curl http://localhost:8080/api/v1/config/logging \
  -H "Authorization: Bearer $TOKEN"
```

Response:

```json
{
  "level": "info",
  "format": "json",
  "output": "stdout"
}
```

#### Update Config Section

Update a config section with hot-reload support. Changes are applied immediately without server restart.

```
PATCH /api/v1/config/{section}
```

Hot-reloadable sections and fields:

| Section                   | Fields                                                               |
|---------------------------|----------------------------------------------------------------------|
| `logging`                 | `level`, `format`                                                    |
| `server`                  | `maxConnections`, `readTimeout`, `writeTimeout`, `tlsCert`, `tlsKey` |
| `security.ratelimit`      | `enabled`, `maxAttempts`, `lockoutDuration`                          |
| `security.passwordpolicy` | All fields                                                           |
| `rest`                    | `rateLimit`, `tokenTTL`, `corsOrigins`                               |

Example - Update log level:

```bash
curl -X PATCH http://localhost:8080/api/v1/config/logging \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"level": "debug"}'
```

Response:

```json
{
  "message": "config section updated",
  "section": "logging",
  "config": {
    "level": "debug",
    "format": "json",
    "output": "stdout"
  }
}
```

#### Reload Config from File

```
POST /api/v1/config/reload
```

Response:

```json
{
  "message": "config reloaded"
}
```

#### Save Config to File

```
POST /api/v1/config/save
```

Response:

```json
{
  "message": "config saved",
  "filePath": "/var/lib/oba/config.yaml"
}
```

#### Validate Config

```
POST /api/v1/config/validate
```

Request: Full config JSON structure

Response:

```json
{
  "valid": true,
  "message": "config structure is valid"
}
```

---

### Log Management

Query and export server logs. Requires persistent log storage to be enabled (`logging.store.enabled: true`). These endpoints require authentication.

#### Query Logs

Retrieve logs with filtering and pagination.

```
GET /api/v1/logs
```

#### Query Parameters

| Parameter | Type   | Default | Description                                   |
|-----------|--------|---------|-----------------------------------------------|
| `level`   | string | -       | Filter by log level: debug, info, warn, error |
| `source`  | string | -       | Filter by source: ldap, rest                  |
| `user`    | string | -       | Filter by username                            |
| `from`    | string | -       | Start time (RFC3339 format)                   |
| `to`      | string | -       | End time (RFC3339 format)                     |
| `limit`   | int    | 100     | Maximum entries to return                     |
| `offset`  | int    | 0       | Number of entries to skip                     |

#### Response

```json
{
  "logs": [
    {
      "id": "1708531200000001",
      "timestamp": "2026-02-21T12:00:00Z",
      "level": "info",
      "message": "bind successful",
      "source": "ldap",
      "user": "admin",
      "fields": {
        "client": "192.168.1.100:54321",
        "dn": "cn=admin,dc=example,dc=com"
      }
    }
  ],
  "total": 1500,
  "offset": 0,
  "limit": 100,
  "hasMore": true
}
```

#### Examples

```bash
# Get recent logs
curl "http://localhost:8080/api/v1/logs?limit=50" \
  -H "Authorization: Bearer $TOKEN"

# Filter by level
curl "http://localhost:8080/api/v1/logs?level=error" \
  -H "Authorization: Bearer $TOKEN"

# Filter by source
curl "http://localhost:8080/api/v1/logs?source=ldap" \
  -H "Authorization: Bearer $TOKEN"

# Filter by time range
curl "http://localhost:8080/api/v1/logs?from=2026-02-21T00:00:00Z&to=2026-02-21T23:59:59Z" \
  -H "Authorization: Bearer $TOKEN"

# Filter by user
curl "http://localhost:8080/api/v1/logs?user=admin" \
  -H "Authorization: Bearer $TOKEN"
```

#### Export Logs

Export logs in various formats for external analysis.

```
GET /api/v1/logs/export
```

#### Query Parameters

Same as Query Logs, plus:

| Parameter | Type   | Default | Description                      |
|-----------|--------|---------|----------------------------------|
| `format`  | string | "json"  | Export format: json, csv, ndjson |

#### Response

Returns logs in the requested format with appropriate Content-Type header:
- `json`: `application/json`
- `csv`: `text/csv`
- `ndjson`: `application/x-ndjson`

#### Examples

```bash
# Export as JSON
curl "http://localhost:8080/api/v1/logs/export?format=json" \
  -H "Authorization: Bearer $TOKEN" \
  -o logs.json

# Export as CSV
curl "http://localhost:8080/api/v1/logs/export?format=csv" \
  -H "Authorization: Bearer $TOKEN" \
  -o logs.csv

# Export as NDJSON (streaming)
curl "http://localhost:8080/api/v1/logs/export?format=ndjson&level=error" \
  -H "Authorization: Bearer $TOKEN" \
  -o errors.ndjson
```

#### Log Entry Fields

| Field       | Type   | Description                           |
|-------------|--------|---------------------------------------|
| `id`        | string | Unique log entry identifier           |
| `timestamp` | string | Log timestamp (RFC3339)               |
| `level`     | string | Log level: debug, info, warn, error   |
| `message`   | string | Log message                           |
| `source`    | string | Log source: ldap, rest                |
| `user`      | string | Username (if authenticated operation) |
| `fields`    | object | Additional structured fields          |

---

## Error Handling

All errors are returned as JSON with consistent structure.

### Error Response Format

```json
{
  "error": "error_code",
  "code": 400,
  "message": "Human-readable error message",
  "resultCode": 32
}
```

| Field        | Type   | Description                        |
|--------------|--------|------------------------------------|
| `error`      | string | Machine-readable error code        |
| `code`       | int    | HTTP status code                   |
| `message`    | string | Human-readable error description   |
| `resultCode` | int    | LDAP result code (when applicable) |

### Error Codes

| Error Code                | HTTP Status | Description                              |
|---------------------------|-------------|------------------------------------------|
| `invalid_request`         | 400         | Malformed JSON or missing required field |
| `missing_dn`              | 400         | DN parameter is required                 |
| `missing_base_dn`         | 400         | baseDN query parameter is required       |
| `invalid_dn`              | 400         | Invalid DN syntax or encoding            |
| `invalid_scope`           | 400         | Invalid search scope                     |
| `invalid_operation`       | 400         | Invalid modify operation                 |
| `missing_new_rdn`         | 400         | newRDN is required for modifyDN          |
| `empty_operations`        | 400         | Bulk request has no operations           |
| `unauthorized`            | 401         | Missing or invalid authentication        |
| `invalid_credentials`     | 401         | Invalid DN or password                   |
| `forbidden`               | 403         | Insufficient access rights               |
| `not_found`               | 404         | Entry not found                          |
| `entry_exists`            | 409         | Entry already exists                     |
| `not_allowed_on_non_leaf` | 409         | Cannot delete entry with children        |
| `time_limit_exceeded`     | 408         | Search time limit exceeded               |
| `rate_limited`            | 429         | Too many requests                        |
| `internal_error`          | 500         | Internal server error                    |

### LDAP Result Code to HTTP Status Mapping

| LDAP Result Code                  | HTTP Status | Description           |
|-----------------------------------|-------------|-----------------------|
| 0 (Success)                       | 200         | Operation successful  |
| 1 (Operations Error)              | 500         | Internal server error |
| 2 (Protocol Error)                | 400         | Bad request           |
| 3 (Time Limit Exceeded)           | 408         | Request timeout       |
| 4 (Size Limit Exceeded)           | 413         | Payload too large     |
| 7 (Auth Method Not Supported)     | 501         | Not implemented       |
| 8 (Stronger Auth Required)        | 401         | Unauthorized          |
| 16 (No Such Attribute)            | 400         | Bad request           |
| 17 (Undefined Attribute Type)     | 400         | Bad request           |
| 19 (Constraint Violation)         | 400         | Bad request           |
| 20 (Attribute Or Value Exists)    | 409         | Conflict              |
| 21 (Invalid Attribute Syntax)     | 400         | Bad request           |
| 32 (No Such Object)               | 404         | Not found             |
| 34 (Invalid DN Syntax)            | 400         | Bad request           |
| 48 (Inappropriate Authentication) | 401         | Unauthorized          |
| 49 (Invalid Credentials)          | 401         | Unauthorized          |
| 50 (Insufficient Access Rights)   | 403         | Forbidden             |
| 51 (Busy)                         | 503         | Service unavailable   |
| 52 (Unavailable)                  | 503         | Service unavailable   |
| 53 (Unwilling To Perform)         | 400         | Bad request           |
| 54 (Loop Detect)                  | 508         | Loop detected         |
| 64 (Naming Violation)             | 400         | Bad request           |
| 65 (Object Class Violation)       | 400         | Bad request           |
| 66 (Not Allowed On Non-Leaf)      | 409         | Conflict              |
| 67 (Not Allowed On RDN)           | 400         | Bad request           |
| 68 (Entry Already Exists)         | 409         | Conflict              |
| 69 (Object Class Mods Prohibited) | 400         | Bad request           |
| 80 (Other)                        | 500         | Internal server error |

---

## Rate Limiting

The REST API implements token bucket rate limiting per IP address.

### Configuration

```yaml
rest:
  rateLimit: 100  # requests per second per IP
```

Set to `0` to disable rate limiting.

### Behavior

- Each IP address has its own token bucket
- Tokens refill at the configured rate per second
- When tokens are exhausted, requests receive `429 Too Many Requests`

### Rate Limit Response

```
HTTP/1.1 429 Too Many Requests
Retry-After: 1
Content-Type: application/json

{
  "error": "rate_limited",
  "code": 429,
  "message": "too many requests"
}
```

The `Retry-After` header indicates how many seconds to wait before retrying.

### Client IP Detection

The server determines client IP in the following order:

1. `X-Forwarded-For` header (first IP in the list)
2. `X-Real-IP` header
3. Remote address from the connection

---

## CORS Configuration

Cross-Origin Resource Sharing (CORS) is configured to allow browser-based applications to access the API.

### Configuration

```yaml
rest:
  corsOrigins:
    - "https://app.example.com"
    - "https://admin.example.com"
```

Use `"*"` to allow all origins (not recommended for production):

```yaml
rest:
  corsOrigins:
    - "*"
```

### CORS Headers

For allowed origins, the following headers are set:

```
Access-Control-Allow-Origin: https://app.example.com
Access-Control-Allow-Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization
Access-Control-Allow-Credentials: true
Access-Control-Max-Age: 86400
```

### Preflight Requests

OPTIONS requests are handled automatically and return `204 No Content` with appropriate CORS headers.

---

## TLS/HTTPS Support

The REST API supports HTTPS using the same TLS certificates as the LDAP server.

### Configuration

```yaml
server:
  tlsCert: "/etc/oba/server.crt"
  tlsKey: "/etc/oba/server.key"

rest:
  enabled: true
  address: ":8080"      # HTTP
  tlsAddress: ":8443"   # HTTPS
```

### TLS Settings

- Minimum TLS version: TLS 1.2
- Certificates are loaded from the paths specified in `server.tlsCert` and `server.tlsKey`

### Example with HTTPS

```bash
curl -k https://localhost:8443/api/v1/health
```

Or with certificate verification:

```bash
curl --cacert /etc/oba/ca.crt https://localhost:8443/api/v1/health
```

---

## Complete Examples

### Example 1: User Management Workflow

```bash
# Set up authentication
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/bind \
  -H "Content-Type: application/json" \
  -d '{"dn": "cn=admin,dc=example,dc=com", "password": "admin"}' \
  | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

# Create organizational unit
curl -X POST http://localhost:8080/api/v1/entries \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "dn": "ou=users,dc=example,dc=com",
    "attributes": {
      "objectClass": ["organizationalUnit"],
      "ou": ["users"]
    }
  }'

# Create a user
curl -X POST http://localhost:8080/api/v1/entries \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "dn": "cn=john,ou=users,dc=example,dc=com",
    "attributes": {
      "objectClass": ["person", "inetOrgPerson"],
      "cn": ["john"],
      "sn": ["Doe"],
      "mail": ["john@example.com"],
      "userPassword": ["secret123"]
    }
  }'

# Search for users
curl "http://localhost:8080/api/v1/search?baseDN=ou=users,dc=example,dc=com&scope=one" \
  -H "Authorization: Bearer $TOKEN"

# Update user email
curl -X PATCH "http://localhost:8080/api/v1/entries/cn%3Djohn%2Cou%3Dusers%2Cdc%3Dexample%2Cdc%3Dcom" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "changes": [
      {"operation": "replace", "attribute": "mail", "values": ["john.doe@example.com"]}
    ]
  }'

# Delete user
curl -X DELETE "http://localhost:8080/api/v1/entries/cn%3Djohn%2Cou%3Dusers%2Cdc%3Dexample%2Cdc%3Dcom" \
  -H "Authorization: Bearer $TOKEN"
```

### Example 2: Bulk Import Users

```bash
curl -X POST http://localhost:8080/api/v1/bulk \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "stopOnError": false,
    "operations": [
      {
        "operation": "add",
        "dn": "cn=alice,ou=users,dc=example,dc=com",
        "attributes": {
          "objectClass": ["person", "inetOrgPerson"],
          "cn": ["alice"],
          "sn": ["Anderson"],
          "mail": ["alice@example.com"]
        }
      },
      {
        "operation": "add",
        "dn": "cn=bob,ou=users,dc=example,dc=com",
        "attributes": {
          "objectClass": ["person", "inetOrgPerson"],
          "cn": ["bob"],
          "sn": ["Brown"],
          "mail": ["bob@example.com"]
        }
      },
      {
        "operation": "add",
        "dn": "cn=charlie,ou=users,dc=example,dc=com",
        "attributes": {
          "objectClass": ["person", "inetOrgPerson"],
          "cn": ["charlie"],
          "sn": ["Clark"],
          "mail": ["charlie@example.com"]
        }
      }
    ]
  }'
```

### Example 3: Paginated Search

```bash
# Get first page (10 entries)
curl "http://localhost:8080/api/v1/search?baseDN=dc=example,dc=com&limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN"

# Get second page
curl "http://localhost:8080/api/v1/search?baseDN=dc=example,dc=com&limit=10&offset=10" \
  -H "Authorization: Bearer $TOKEN"

# Get third page
curl "http://localhost:8080/api/v1/search?baseDN=dc=example,dc=com&limit=10&offset=20" \
  -H "Authorization: Bearer $TOKEN"
```

### Example 4: Move User Between OUs

```bash
# Move user from users to managers
curl -X POST "http://localhost:8080/api/v1/entries/cn%3Djohn%2Cou%3Dusers%2Cdc%3Dexample%2Cdc%3Dcom/move" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "newRDN": "cn=john",
    "newSuperior": "ou=managers,dc=example,dc=com"
  }'
```

### Example 5: Verify User Password

```bash
# Compare userPassword attribute
curl -X POST http://localhost:8080/api/v1/compare \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "dn": "cn=john,ou=users,dc=example,dc=com",
    "attribute": "userPassword",
    "value": "secret123"
  }'
```

---

## API Reference Summary

| Method | Endpoint                    | Description                    | Auth Required |
|--------|-----------------------------|--------------------------------|---------------|
| GET    | `/api/v1/health`            | Health check                   | No            |
| POST   | `/api/v1/auth/bind`         | Authenticate and get JWT       | No            |
| GET    | `/api/v1/entries/{dn}`      | Get single entry               | Yes           |
| GET    | `/api/v1/search`            | Search entries with pagination | Yes           |
| GET    | `/api/v1/search/stream`     | Stream search results (NDJSON) | Yes           |
| POST   | `/api/v1/entries`           | Create new entry               | Yes           |
| PUT    | `/api/v1/entries/{dn}`      | Modify entry                   | Yes           |
| PATCH  | `/api/v1/entries/{dn}`      | Modify entry                   | Yes           |
| DELETE | `/api/v1/entries/{dn}`      | Delete entry                   | Yes           |
| POST   | `/api/v1/entries/{dn}/move` | Rename/move entry              | Yes           |
| POST   | `/api/v1/compare`           | Compare attribute value        | Yes           |
| POST   | `/api/v1/bulk`              | Bulk operations                | Yes           |
| GET    | `/api/v1/acl`               | Get ACL configuration          | Admin         |
| GET    | `/api/v1/acl/rules`         | List ACL rules                 | Admin         |
| GET    | `/api/v1/acl/rules/{index}` | Get single ACL rule            | Admin         |
| POST   | `/api/v1/acl/rules`         | Add ACL rule                   | Admin         |
| PUT    | `/api/v1/acl/rules/{index}` | Update ACL rule                | Admin         |
| DELETE | `/api/v1/acl/rules/{index}` | Delete ACL rule                | Admin         |
| PUT    | `/api/v1/acl/default`       | Set default ACL policy         | Admin         |
| POST   | `/api/v1/acl/reload`        | Reload ACL from file           | Admin         |
| POST   | `/api/v1/acl/save`          | Save ACL to file               | Admin         |
| POST   | `/api/v1/acl/validate`      | Validate ACL configuration     | Admin         |
| GET    | `/api/v1/config`            | Get full configuration         | Admin         |
| GET    | `/api/v1/config/{section}`  | Get config section             | Admin         |
| PATCH  | `/api/v1/config/{section}`  | Update config section          | Admin         |
| POST   | `/api/v1/config/reload`     | Reload config from file        | Admin         |
| POST   | `/api/v1/config/save`       | Save config to file            | Admin         |
| POST   | `/api/v1/config/validate`   | Validate configuration         | Admin         |
| GET    | `/api/v1/logs`              | Query logs with filtering      | Yes           |
| GET    | `/api/v1/logs/export`       | Export logs (json/csv/ndjson)  | Yes           |
