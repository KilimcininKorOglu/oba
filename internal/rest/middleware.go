package rest

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/logging"
)

type bindDNKey struct{}

// BindDN retrieves the authenticated DN from context.
func BindDN(r *http.Request) string {
	dn, _ := r.Context().Value(bindDNKey{}).(string)
	return dn
}

// loggingResponseWriter wraps ResponseWriter and captures user from context
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	user       string
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// SetUser sets the user for logging
func (w *loggingResponseWriter) SetUser(user string) {
	w.user = user
}

// LoggingMiddleware logs HTTP requests.
func LoggingMiddleware(logger logging.Logger) Middleware {
	restLogger := logger.WithSource("rest")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			wrapped := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			reqLogger := restLogger
			if wrapped.user != "" {
				reqLogger = reqLogger.WithUser(wrapped.user)
			}

			// Generate meaningful message based on path and method
			msg := getAuditMessage(r.Method, r.URL.Path)

			// Skip logging for health checks
			if msg == "" {
				return
			}

			reqLogger.Info(msg,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration", time.Since(start).String(),
				"remoteAddr", r.RemoteAddr,
			)
		})
	}
}

// getAuditMessage returns a meaningful audit message based on path and method
func getAuditMessage(method, path string) string {
	// Auth
	if strings.HasPrefix(path, "/api/v1/auth/bind") {
		return "REST login"
	}

	// Entry operations
	if strings.HasPrefix(path, "/api/v1/entries") {
		switch method {
		case "GET":
			return "REST get entry"
		case "POST":
			return "REST add entry"
		case "PUT", "PATCH":
			return "REST modify entry"
		case "DELETE":
			return "REST delete entry"
		}
		if strings.HasSuffix(path, "/move") {
			return "REST move entry"
		}
	}

	// Search
	if strings.HasPrefix(path, "/api/v1/search") {
		return "REST search"
	}

	// Bulk
	if strings.HasPrefix(path, "/api/v1/bulk") {
		return "REST bulk operation"
	}

	// Compare
	if strings.HasPrefix(path, "/api/v1/compare") {
		return "REST compare"
	}

	// ACL
	if strings.HasPrefix(path, "/api/v1/acl") {
		switch method {
		case "GET":
			return "REST get ACL"
		case "POST":
			if strings.HasSuffix(path, "/rules") {
				return "REST add ACL rule"
			}
			if strings.HasSuffix(path, "/reload") {
				return "REST reload ACL"
			}
			if strings.HasSuffix(path, "/save") {
				return "REST save ACL"
			}
			return "REST ACL operation"
		case "PUT":
			if strings.Contains(path, "/rules/") {
				return "REST update ACL rule"
			}
			if strings.HasSuffix(path, "/default") {
				return "REST set default policy"
			}
			return "REST ACL operation"
		case "DELETE":
			return "REST delete ACL rule"
		}
	}

	// Config
	if strings.HasPrefix(path, "/api/v1/config") {
		if strings.HasSuffix(path, "/public") {
			return "REST get public config"
		}
		switch method {
		case "GET":
			return "REST get config"
		case "PATCH":
			return "REST update config"
		case "POST":
			if strings.HasSuffix(path, "/reload") {
				return "REST reload config"
			}
			if strings.HasSuffix(path, "/save") {
				return "REST save config"
			}
			return "REST config operation"
		}
	}

	// Logs
	if strings.HasPrefix(path, "/api/v1/logs") {
		switch method {
		case "GET":
			if strings.HasSuffix(path, "/stats") {
				return "REST get log stats"
			}
			if strings.HasSuffix(path, "/export") {
				return "REST export logs"
			}
			return "REST get logs"
		case "DELETE":
			return "REST clear logs"
		}
	}

	// Health check - don't log
	if path == "/api/v1/health" {
		return ""
	}

	return "REST request"
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// CORSMiddleware handles CORS headers.
func CORSMiddleware(allowedOrigins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			allowed := false
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if allowed && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RecoveryMiddleware recovers from panics.
func RecoveryMiddleware(logger logging.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered", "error", err, "path", r.URL.Path)
					writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitMiddleware limits request rate per IP.
func RateLimitMiddleware(requestsPerSecond int) Middleware {
	buckets := make(map[string]*tokenBucket)
	var mu sync.Mutex

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			mu.Lock()
			bucket, ok := buckets[ip]
			if !ok {
				bucket = newTokenBucket(requestsPerSecond)
				buckets[ip] = bucket
			}
			mu.Unlock()

			if !bucket.Allow() {
				w.Header().Set("Retry-After", "1")
				writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
	mu         sync.Mutex
}

func newTokenBucket(rps int) *tokenBucket {
	return &tokenBucket{
		tokens:     float64(rps),
		maxTokens:  float64(rps),
		refillRate: float64(rps),
		lastRefill: time.Now(),
	}
}

func (b *tokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = min(b.maxTokens, b.tokens+elapsed*b.refillRate)
	b.lastRefill = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// ConnectionTrackingMiddleware tracks active connections.
func ConnectionTrackingMiddleware(handlers *Handlers) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlers.IncrementConnections()
			defer handlers.DecrementConnections()
			next.ServeHTTP(w, r)
		})
	}
}

// AuthMiddleware validates JWT or Basic auth.
func AuthMiddleware(auth *Authenticator, excludePaths []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, path := range excludePaths {
				if strings.HasPrefix(r.URL.Path, path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "unauthorized", "missing authorization header")
				return
			}

			var bindDN string

			if strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				claims, err := auth.ValidateToken(token)
				if err != nil {
					writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
					return
				}
				bindDN = claims.DN
			} else if strings.HasPrefix(authHeader, "Basic ") {
				dn, password, ok := r.BasicAuth()
				if !ok {
					writeError(w, http.StatusUnauthorized, "unauthorized", "invalid basic auth")
					return
				}

				if err := auth.backend.Bind(dn, password); err != nil {
					writeError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
					return
				}
				bindDN = dn
			} else {
				writeError(w, http.StatusUnauthorized, "unauthorized", "unsupported authorization type")
				return
			}

			ctx := context.WithValue(r.Context(), bindDNKey{}, bindDN)

			// Set user on logging response writer if available
			if lrw, ok := w.(*loggingResponseWriter); ok {
				lrw.SetUser(bindDN)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminOnlyMiddleware restricts access to admin users only.
// adminDNs is a list of DNs that are considered admins.
// excludePaths are paths that should not require admin access even if they match adminPaths prefix.
func AdminOnlyMiddleware(adminDNs []string, adminPaths []string, excludePaths []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path is excluded
			for _, path := range excludePaths {
				if r.URL.Path == path || strings.HasPrefix(r.URL.Path, path+"/") {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check if this path requires admin access
			requiresAdmin := false
			for _, path := range adminPaths {
				if strings.HasPrefix(r.URL.Path, path) {
					requiresAdmin = true
					break
				}
			}

			if !requiresAdmin {
				next.ServeHTTP(w, r)
				return
			}

			// Get authenticated DN from context
			bindDN := BindDN(r)
			if bindDN == "" {
				writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}

			// Check if user is admin
			isAdmin := false
			normalizedBindDN := strings.ToLower(bindDN)
			for _, adminDN := range adminDNs {
				if strings.ToLower(adminDN) == normalizedBindDN {
					isAdmin = true
					break
				}
			}

			if !isAdmin {
				writeError(w, http.StatusForbidden, "forbidden", "admin access required")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
