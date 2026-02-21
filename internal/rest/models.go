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

// StatsResponse represents server statistics.
type StatsResponse struct {
	// Server stats
	Status      string    `json:"status"`
	Version     string    `json:"version"`
	Uptime      string    `json:"uptime"`
	UptimeSecs  int64     `json:"uptimeSecs"`
	StartTime   time.Time `json:"startTime"`
	Connections int       `json:"connections"`
	Requests    int64     `json:"requests"`
	Timezone    string    `json:"timezone"`

	// Storage stats
	Storage StorageStats `json:"storage"`

	// Security stats
	Security SecurityStats `json:"security"`

	// System stats
	System SystemStats `json:"system"`

	// LDAP operation stats
	Operations OperationStats `json:"operations"`
}

// StorageStats contains storage-related statistics.
type StorageStats struct {
	EntryCount         uint64 `json:"entryCount"`
	IndexCount         int    `json:"indexCount"`
	TotalPages         uint64 `json:"totalPages"`
	UsedPages          uint64 `json:"usedPages"`
	FreePages          uint64 `json:"freePages"`
	BufferPoolSize     int    `json:"bufferPoolSize"`
	DirtyPages         int    `json:"dirtyPages"`
	ActiveTransactions int    `json:"activeTransactions"`
	WALSize            uint64 `json:"walSize"`
	DatabaseSizeBytes  int64  `json:"databaseSizeBytes"`
}

// SecurityStats contains security-related statistics.
type SecurityStats struct {
	LockedAccounts   int `json:"lockedAccounts"`
	DisabledAccounts int `json:"disabledAccounts"`
	FailedLogins24h  int `json:"failedLogins24h"`
}

// SystemStats contains system-related statistics.
type SystemStats struct {
	GoRoutines  int    `json:"goRoutines"`
	MemoryAlloc uint64 `json:"memoryAlloc"`
	MemoryTotal uint64 `json:"memoryTotal"`
	MemorySys   uint64 `json:"memorySys"`
	NumGC       uint32 `json:"numGC"`
	NumCPU      int    `json:"numCPU"`
}

// OperationStats contains LDAP operation statistics.
type OperationStats struct {
	Binds    int64 `json:"binds"`
	Searches int64 `json:"searches"`
	Adds     int64 `json:"adds"`
	Modifies int64 `json:"modifies"`
	Deletes  int64 `json:"deletes"`
	Compares int64 `json:"compares"`
}

// ActivityEntry represents a recent activity log entry.
type ActivityEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	User      string    `json:"user,omitempty"`
	Target    string    `json:"target,omitempty"`
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"`
}
