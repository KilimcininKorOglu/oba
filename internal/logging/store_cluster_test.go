package logging

import (
	"path/filepath"
	"testing"
	"time"
)

func TestWriteBuffersWhenClusterModeEnabledAndWriterNotSet(t *testing.T) {
	t.Parallel()

	store, err := NewLogStore(LogStoreConfig{
		Enabled:    true,
		DBPath:     filepath.Join(t.TempDir(), "logdb"),
		MaxEntries: 100,
	})
	if err != nil {
		t.Fatalf("NewLogStore failed: %v", err)
	}
	defer store.Close()

	store.EnableClusterMode()

	if err := store.Write("info", "startup event", "system", "", "", map[string]interface{}{"k": "v"}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if got := store.PendingCount(); got != 1 {
		t.Fatalf("PendingCount = %d, want 1", got)
	}

	entries, total, err := store.Query(QueryOptions{
		Offset: 0,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if total != 0 || len(entries) != 0 {
		t.Fatalf("expected no local entries before cluster writer is set, got total=%d len=%d", total, len(entries))
	}

	// Ensure buffered entries are not auto-written locally over a short period.
	time.Sleep(50 * time.Millisecond)
	entries, total, err = store.Query(QueryOptions{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if total != 0 || len(entries) != 0 {
		t.Fatalf("unexpected local write while cluster writer is nil, total=%d len=%d", total, len(entries))
	}
}
