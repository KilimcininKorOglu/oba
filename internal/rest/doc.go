// Package rest provides a RESTful HTTP API for the LDAP server.
//
// The REST API allows modern applications to interact with the LDAP directory
// using standard HTTP methods and JSON payloads. All REST operations are
// translated to LDAP operations via the backend interface.
//
// # Endpoints
//
// Authentication:
//
//	POST /api/v1/auth/bind - Authenticate and get JWT token
//
// Entries:
//
//	GET    /api/v1/entries/{dn} - Get single entry
//	POST   /api/v1/entries      - Create new entry
//	PUT    /api/v1/entries/{dn} - Update entry (full replace)
//	PATCH  /api/v1/entries/{dn} - Update entry (partial)
//	DELETE /api/v1/entries/{dn} - Delete entry
//	POST   /api/v1/entries/{dn}/move - Rename/move entry
//
// Search:
//
//	GET /api/v1/search        - Search with pagination
//	GET /api/v1/search/stream - Streaming search (NDJSON)
//
// Other:
//
//	POST /api/v1/bulk    - Bulk operations
//	POST /api/v1/compare - Compare attribute value
//	GET  /api/v1/health  - Health check
//
// # Authentication
//
// The API supports two authentication methods:
//
//   - JWT Bearer token: Obtained via /api/v1/auth/bind
//   - Basic Auth: Standard HTTP Basic Authentication
//
// # Example Usage
//
//	// Get JWT token
//	curl -X POST http://localhost:8080/api/v1/auth/bind \
//	  -H "Content-Type: application/json" \
//	  -d '{"dn": "cn=admin,dc=example,dc=com", "password": "secret"}'
//
//	// Search with token
//	curl -X GET "http://localhost:8080/api/v1/search?baseDN=dc=example,dc=com&scope=sub" \
//	  -H "Authorization: Bearer <token>"
package rest
