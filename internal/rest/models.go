package rest

import (
	"time"
)

// Entry represents an LDAP entry in JSON format.
type Entry struct {
	DN         string              `json:"dn"`
	Attributes map[string][]string `json:"attributes"`
}

// BindRequest represents an authentication request.
type BindRequest struct {
	DN       string `json:"dn"`
	Password string `json:"password"`
}

// BindResponse represents an authentication response.
type BindResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Message string `json:"message,omitempty"`
}

// SearchRequest represents a search request (for POST-based search).
type SearchRequest struct {
	BaseDN     string   `json:"baseDN"`
	Scope      string   `json:"scope"`
	Filter     string   `json:"filter"`
	Attributes []string `json:"attributes"`
	SizeLimit  int      `json:"sizeLimit"`
	TimeLimit  int      `json:"timeLimit"`
	Offset     int      `json:"offset"`
	Limit      int      `json:"limit"`
}

// SearchResponse represents a search response.
type SearchResponse struct {
	Entries    []*Entry `json:"entries"`
	TotalCount int      `json:"totalCount"`
	Offset     int      `json:"offset"`
	Limit      int      `json:"limit"`
	HasMore    bool     `json:"hasMore"`
}

// AddRequest represents an add request.
type AddRequest struct {
	DN         string              `json:"dn"`
	Attributes map[string][]string `json:"attributes"`
}

// ModifyRequest represents a modify request.
type ModifyRequest struct {
	Changes []ModifyChange `json:"changes"`
}

// ModifyChange represents a single modification.
type ModifyChange struct {
	Operation string   `json:"operation"`
	Attribute string   `json:"attribute"`
	Values    []string `json:"values"`
}

// ModifyDNRequest represents a modifyDN request.
type ModifyDNRequest struct {
	NewRDN       string `json:"newRDN"`
	DeleteOldRDN bool   `json:"deleteOldRDN"`
	NewSuperior  string `json:"newSuperior,omitempty"`
}

// CompareRequest represents a compare request.
type CompareRequest struct {
	DN        string `json:"dn"`
	Attribute string `json:"attribute"`
	Value     string `json:"value"`
}

// CompareResponse represents a compare response.
type CompareResponse struct {
	Match bool `json:"match"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error      string `json:"error"`
	Code       int    `json:"code"`
	Message    string `json:"message"`
	ResultCode int    `json:"resultCode,omitempty"`
}

// HealthResponse represents a health check response.
type HealthResponse struct {
	Status      string    `json:"status"`
	Version     string    `json:"version"`
	Uptime      string    `json:"uptime"`
	UptimeSecs  int64     `json:"uptimeSecs"`
	StartTime   time.Time `json:"startTime"`
	Connections int       `json:"connections"`
	Requests    int64     `json:"requests"`
}

// BulkRequest represents a bulk operation request.
type BulkRequest struct {
	Operations  []BulkOperation `json:"operations"`
	StopOnError bool            `json:"stopOnError"`
}

// BulkOperation represents a single operation in a bulk request.
type BulkOperation struct {
	Operation  string              `json:"operation"`
	DN         string              `json:"dn"`
	Attributes map[string][]string `json:"attributes,omitempty"`
	Changes    []ModifyChange      `json:"changes,omitempty"`
}

// BulkResponse represents a bulk operation response.
type BulkResponse struct {
	Success    bool                  `json:"success"`
	TotalCount int                   `json:"totalCount"`
	Succeeded  int                   `json:"succeeded"`
	Failed     int                   `json:"failed"`
	Results    []BulkOperationResult `json:"results"`
}

// BulkOperationResult represents the result of a single bulk operation.
type BulkOperationResult struct {
	Index      int    `json:"index"`
	DN         string `json:"dn"`
	Operation  string `json:"operation"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	ResultCode int    `json:"resultCode,omitempty"`
}
