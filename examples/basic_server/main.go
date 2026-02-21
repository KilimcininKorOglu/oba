// Example basic_server demonstrates a minimal LDAP server setup.
//
// This example shows how to:
//   - Create a backend with in-memory storage
//   - Configure basic authentication
//   - Handle bind and search operations
//   - Start the server on a custom port
//
// Run with: go run examples/basic_server/main.go
//
// Test with:
//
//	ldapsearch -x -H ldap://localhost:10389 -b "dc=example,dc=com" "(objectClass=*)"
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/KilimcininKorOglu/oba/internal/backend"
	"github.com/KilimcininKorOglu/oba/internal/config"
	"github.com/KilimcininKorOglu/oba/internal/filter"
	"github.com/KilimcininKorOglu/oba/internal/ldap"
	"github.com/KilimcininKorOglu/oba/internal/logging"
	"github.com/KilimcininKorOglu/oba/internal/server"
)

func main() {
	// Create logger
	logger := logging.New(logging.Config{
		Level:  "info",
		Format: "text",
		Output: "stdout",
	})

	// Create configuration
	cfg := &config.Config{
		Server: config.ServerConfig{
			Address: ":10389",
		},
		Directory: config.DirectoryConfig{
			BaseDN:       "dc=example,dc=com",
			RootDN:       "cn=admin,dc=example,dc=com",
			RootPassword: "secret",
		},
	}

	// Create a simple in-memory backend
	be := newSimpleBackend(cfg)

	// Add some sample entries
	addSampleEntries(be)

	// Create handler with custom bind and search handlers
	handler := server.NewHandler()

	handler.SetBindHandler(func(conn *server.Connection, req *ldap.BindRequest) *server.OperationResult {
		if err := be.Bind(req.Name, string(req.SimplePassword)); err != nil {
			return &server.OperationResult{
				ResultCode:        ldap.ResultInvalidCredentials,
				DiagnosticMessage: err.Error(),
			}
		}
		return &server.OperationResult{ResultCode: ldap.ResultSuccess}
	})

	handler.SetSearchHandler(func(conn *server.Connection, req *ldap.SearchRequest) *server.SearchResult {
		// Convert LDAP filter to internal filter
		f := convertFilter(req.Filter)

		entries, err := be.Search(req.BaseObject, int(req.Scope), f)
		if err != nil {
			return &server.SearchResult{
				OperationResult: server.OperationResult{
					ResultCode:        ldap.ResultOperationsError,
					DiagnosticMessage: err.Error(),
				},
			}
		}

		// Convert to search entries
		searchEntries := make([]*server.SearchEntry, len(entries))
		for i, e := range entries {
			searchEntries[i] = &server.SearchEntry{
				DN:         e.DN,
				Attributes: convertAttributes(e.Attributes),
			}
		}

		return &server.SearchResult{
			OperationResult: server.OperationResult{ResultCode: ldap.ResultSuccess},
			Entries:         searchEntries,
		}
	})

	// Start TCP listener
	listener, err := net.Listen("tcp", cfg.Server.Address)
	if err != nil {
		log.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	logger.Info("LDAP server started",
		"address", cfg.Server.Address,
		"base_dn", cfg.Directory.BaseDN,
	)

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Shutting down...")
		listener.Close()
	}()

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if we're shutting down
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				break
			}
			logger.Error("Accept error", "error", err)
			continue
		}

		// Handle connection in goroutine
		go func(c net.Conn) {
			srv := &server.Server{Handler: handler, Logger: logger}
			serverConn := server.NewConnection(c, srv)
			serverConn.Handle()
		}(conn)
	}

	logger.Info("Server stopped")
}

// simpleBackend is a minimal backend implementation for the example.
type simpleBackend struct {
	entries map[string]*backend.Entry
	rootDN  string
	rootPW  string
}

func newSimpleBackend(cfg *config.Config) *simpleBackend {
	return &simpleBackend{
		entries: make(map[string]*backend.Entry),
		rootDN:  cfg.Directory.RootDN,
		rootPW:  cfg.Directory.RootPassword,
	}
}

func (b *simpleBackend) Bind(dn, password string) error {
	// Anonymous bind
	if dn == "" {
		return nil
	}

	// Root bind
	if dn == b.rootDN && password == b.rootPW {
		return nil
	}

	return fmt.Errorf("invalid credentials")
}

func (b *simpleBackend) Search(baseDN string, scope int, f *filter.Filter) ([]*backend.Entry, error) {
	var results []*backend.Entry
	for _, entry := range b.entries {
		results = append(results, entry)
	}
	return results, nil
}

func (b *simpleBackend) Add(entry *backend.Entry) error {
	b.entries[entry.DN] = entry
	return nil
}

func addSampleEntries(be *simpleBackend) {
	// Add base entry
	base := backend.NewEntry("dc=example,dc=com")
	base.SetAttribute("objectclass", "domain", "top")
	base.SetAttribute("dc", "example")
	be.Add(base)

	// Add users OU
	users := backend.NewEntry("ou=users,dc=example,dc=com")
	users.SetAttribute("objectclass", "organizationalUnit", "top")
	users.SetAttribute("ou", "users")
	be.Add(users)

	// Add sample user
	alice := backend.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	alice.SetAttribute("objectclass", "inetOrgPerson", "person", "top")
	alice.SetAttribute("uid", "alice")
	alice.SetAttribute("cn", "Alice Smith")
	alice.SetAttribute("sn", "Smith")
	alice.SetAttribute("mail", "alice@example.com")
	be.Add(alice)
}

func convertFilter(f *ldap.SearchFilter) *filter.Filter {
	if f == nil {
		return nil
	}
	// Simplified filter conversion for example
	return filter.NewPresentFilter("objectClass")
}

func convertAttributes(attrs map[string][]string) []ldap.Attribute {
	var result []ldap.Attribute
	for name, values := range attrs {
		byteValues := make([][]byte, len(values))
		for i, v := range values {
			byteValues[i] = []byte(v)
		}
		result = append(result, ldap.Attribute{Type: name, Values: byteValues})
	}
	return result
}
