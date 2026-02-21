package rest

import (
	"context"
	"net/http"
	"strings"
)

// Route represents a single route.
type Route struct {
	Method  string
	Pattern string
	Handler http.HandlerFunc
}

// Router is a simple HTTP router.
type Router struct {
	routes     []Route
	middleware []Middleware
	notFound   http.HandlerFunc
}

// Middleware is a function that wraps an http.Handler.
type Middleware func(http.Handler) http.Handler

// NewRouter creates a new router.
func NewRouter() *Router {
	return &Router{
		routes:   make([]Route, 0),
		notFound: defaultNotFound,
	}
}

// Use adds middleware to the router.
func (r *Router) Use(mw Middleware) {
	r.middleware = append(r.middleware, mw)
}

// Handle registers a route.
func (r *Router) Handle(method, pattern string, handler http.HandlerFunc) {
	r.routes = append(r.routes, Route{
		Method:  method,
		Pattern: pattern,
		Handler: handler,
	})
}

// GET registers a GET route.
func (r *Router) GET(pattern string, handler http.HandlerFunc) {
	r.Handle(http.MethodGet, pattern, handler)
}

// POST registers a POST route.
func (r *Router) POST(pattern string, handler http.HandlerFunc) {
	r.Handle(http.MethodPost, pattern, handler)
}

// PUT registers a PUT route.
func (r *Router) PUT(pattern string, handler http.HandlerFunc) {
	r.Handle(http.MethodPut, pattern, handler)
}

// PATCH registers a PATCH route.
func (r *Router) PATCH(pattern string, handler http.HandlerFunc) {
	r.Handle(http.MethodPatch, pattern, handler)
}

// DELETE registers a DELETE route.
func (r *Router) DELETE(pattern string, handler http.HandlerFunc) {
	r.Handle(http.MethodDelete, pattern, handler)
}

// OPTIONS registers an OPTIONS route.
func (r *Router) OPTIONS(pattern string, handler http.HandlerFunc) {
	r.Handle(http.MethodOptions, pattern, handler)
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for _, route := range r.routes {
		if route.Method != req.Method {
			continue
		}

		params, ok := matchPattern(route.Pattern, req.URL.Path)
		if !ok {
			continue
		}

		ctx := withParams(req.Context(), params)
		req = req.WithContext(ctx)

		var handler http.Handler = route.Handler
		for i := len(r.middleware) - 1; i >= 0; i-- {
			handler = r.middleware[i](handler)
		}

		handler.ServeHTTP(w, req)
		return
	}

	r.notFound(w, req)
}

type paramsKey struct{}

func withParams(ctx context.Context, params map[string]string) context.Context {
	return context.WithValue(ctx, paramsKey{}, params)
}

// Param retrieves a URL parameter from context.
func Param(r *http.Request, name string) string {
	params, ok := r.Context().Value(paramsKey{}).(map[string]string)
	if !ok {
		return ""
	}
	return params[name]
}

// matchPattern matches a URL pattern with path parameters.
func matchPattern(pattern, path string) (map[string]string, bool) {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return nil, false
	}

	params := make(map[string]string)

	for i, part := range patternParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			paramName := part[1 : len(part)-1]
			params[paramName] = pathParts[i]
		} else if part != pathParts[i] {
			return nil, false
		}
	}

	return params, true
}

func defaultNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "endpoint not found")
}
