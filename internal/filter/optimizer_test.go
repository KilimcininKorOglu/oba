package filter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/index"
)

// testSetup creates a temporary directory and page manager for testing.
func testSetup(t *testing.T) (*storage.PageManager, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "optimizer_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dataFile := filepath.Join(tmpDir, "test.oba")
	opts := storage.DefaultOptions()
	opts.CreateIfNew = true

	pm, err := storage.OpenPageManager(dataFile, opts)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create page manager: %v", err)
	}

	cleanup := func() {
		pm.Close()
		os.RemoveAll(tmpDir)
	}

	return pm, tmpDir, cleanup
}

// TestNewOptimizer tests the Optimizer constructor.
func TestNewOptimizer(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	opt := NewOptimizer(im)
	if opt == nil {
		t.Fatal("expected non-nil optimizer")
	}

	if opt.indexManager != im {
		t.Error("optimizer should reference the index manager")
	}
}

// TestNewOptimizerNilIndexManager tests optimizer with nil index manager.
func TestNewOptimizerNilIndexManager(t *testing.T) {
	opt := NewOptimizer(nil)
	if opt == nil {
		t.Fatal("expected non-nil optimizer even with nil index manager")
	}

	// Should return full scan for any filter
	filter := NewEqualityFilter("uid", []byte("alice"))
	plan := opt.Optimize(filter)

	if plan.UseIndex {
		t.Error("expected full scan when index manager is nil")
	}
}

// TestOptimizeNilFilter tests optimization of nil filter.
func TestOptimizeNilFilter(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	opt := NewOptimizer(im)
	plan := opt.Optimize(nil)

	if plan.UseIndex {
		t.Error("expected full scan for nil filter")
	}
}

// TestOptimizeEqualityWithIndex tests equality filter optimization with index.
func TestOptimizeEqualityWithIndex(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// "uid" is a default indexed attribute
	filter := NewEqualityFilter("uid", []byte("alice"))
	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if !plan.UseIndex {
		t.Error("expected index usage for equality filter on indexed attribute")
	}

	if plan.IndexAttr != "uid" {
		t.Errorf("expected IndexAttr 'uid', got '%s'", plan.IndexAttr)
	}

	if plan.IndexType != index.IndexEquality {
		t.Errorf("expected IndexEquality, got %v", plan.IndexType)
	}

	if string(plan.IndexLookup) != "alice" {
		t.Errorf("expected IndexLookup 'alice', got '%s'", plan.IndexLookup)
	}

	if plan.PostFilter != nil {
		t.Error("expected no post-filter for simple equality")
	}

	if plan.EstimatedCost != CostIndexLookup {
		t.Errorf("expected cost %d, got %d", CostIndexLookup, plan.EstimatedCost)
	}
}

// TestOptimizeEqualityWithoutIndex tests equality filter without index.
func TestOptimizeEqualityWithoutIndex(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// "description" is not indexed by default
	filter := NewEqualityFilter("description", []byte("test"))
	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if plan.UseIndex {
		t.Error("expected full scan for non-indexed attribute")
	}

	if plan.EstimatedCost != CostFullScan {
		t.Errorf("expected cost %d, got %d", CostFullScan, plan.EstimatedCost)
	}
}

// TestOptimizePresenceWithIndex tests presence filter with presence index.
func TestOptimizePresenceWithIndex(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Create a presence index
	err = im.CreateIndex("email", index.IndexPresence)
	if err != nil {
		t.Fatalf("failed to create presence index: %v", err)
	}

	filter := NewPresentFilter("email")
	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if !plan.UseIndex {
		t.Error("expected index usage for presence filter on indexed attribute")
	}

	if plan.IndexType != index.IndexPresence {
		t.Errorf("expected IndexPresence, got %v", plan.IndexType)
	}

	if plan.EstimatedCost != CostPresenceIndex {
		t.Errorf("expected cost %d, got %d", CostPresenceIndex, plan.EstimatedCost)
	}
}

// TestOptimizePresenceWithoutIndex tests presence filter without index.
func TestOptimizePresenceWithoutIndex(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	filter := NewPresentFilter("nonexistent")
	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if plan.UseIndex {
		t.Error("expected full scan for non-indexed presence filter")
	}
}

// TestOptimizeSubstringWithIndex tests substring filter with substring index.
func TestOptimizeSubstringWithIndex(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Create a substring index
	err = im.CreateIndex("description", index.IndexSubstring)
	if err != nil {
		t.Fatalf("failed to create substring index: %v", err)
	}

	sf := &SubstringFilter{
		Attribute: "description",
		Initial:   []byte("admin"),
		Any:       nil,
		Final:     nil,
	}
	filter := NewSubstringFilter(sf)
	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if !plan.UseIndex {
		t.Error("expected index usage for substring filter on indexed attribute")
	}

	if plan.IndexType != index.IndexSubstring {
		t.Errorf("expected IndexSubstring, got %v", plan.IndexType)
	}

	if plan.SubstringPattern != sf {
		t.Error("expected SubstringPattern to be set")
	}

	// Post-filter should be set for substring (to verify actual match)
	if plan.PostFilter == nil {
		t.Error("expected post-filter for substring index")
	}
}

// TestOptimizeSubstringShortPattern tests substring with pattern too short.
func TestOptimizeSubstringShortPattern(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	err = im.CreateIndex("description", index.IndexSubstring)
	if err != nil {
		t.Fatalf("failed to create substring index: %v", err)
	}

	// Pattern too short (less than 3 chars)
	sf := &SubstringFilter{
		Attribute: "description",
		Initial:   []byte("ab"),
		Any:       nil,
		Final:     nil,
	}
	filter := NewSubstringFilter(sf)
	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if plan.UseIndex {
		t.Error("expected full scan for short substring pattern")
	}
}

// TestOptimizeAndSelectsBestIndex tests AND filter selects best index.
func TestOptimizeAndSelectsBestIndex(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Create AND filter: (&(uid=alice)(description=test))
	// uid is indexed, description is not
	filter := NewAndFilter(
		NewEqualityFilter("uid", []byte("alice")),
		NewEqualityFilter("description", []byte("test")),
	)

	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if !plan.UseIndex {
		t.Error("expected index usage for AND with indexed child")
	}

	if plan.IndexAttr != "uid" {
		t.Errorf("expected IndexAttr 'uid', got '%s'", plan.IndexAttr)
	}

	// Should have post-filter for the non-indexed condition
	if plan.PostFilter == nil {
		t.Error("expected post-filter for remaining AND conditions")
	}
}

// TestOptimizeAndMultipleIndexes tests AND with multiple indexed attributes.
func TestOptimizeAndMultipleIndexes(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Both uid and cn are indexed by default
	filter := NewAndFilter(
		NewEqualityFilter("uid", []byte("alice")),
		NewEqualityFilter("cn", []byte("Alice Smith")),
	)

	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if !plan.UseIndex {
		t.Error("expected index usage")
	}

	// Should select one index and post-filter the other
	if plan.PostFilter == nil {
		t.Error("expected post-filter for second condition")
	}
}

// TestOptimizeAndNoIndexes tests AND with no indexed attributes.
func TestOptimizeAndNoIndexes(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	filter := NewAndFilter(
		NewEqualityFilter("description", []byte("test")),
		NewEqualityFilter("location", []byte("NYC")),
	)

	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if plan.UseIndex {
		t.Error("expected full scan when no children are indexed")
	}
}

// TestOptimizeAndEmpty tests empty AND filter.
func TestOptimizeAndEmpty(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	filter := NewAndFilter()
	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if plan.UseIndex {
		t.Error("expected full scan for empty AND")
	}
}

// TestOptimizeOrAllIndexed tests OR with all indexed children.
func TestOptimizeOrAllIndexed(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Both uid and cn are indexed
	filter := NewOrFilter(
		NewEqualityFilter("uid", []byte("alice")),
		NewEqualityFilter("cn", []byte("Bob")),
	)

	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	// OR filters currently fall back to full scan with optimized cost
	// because union execution is complex
	if plan.UseIndex {
		t.Error("OR filters should use full scan (union not implemented)")
	}
}

// TestOptimizeOrPartiallyIndexed tests OR with some non-indexed children.
func TestOptimizeOrPartiallyIndexed(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	filter := NewOrFilter(
		NewEqualityFilter("uid", []byte("alice")),
		NewEqualityFilter("description", []byte("test")), // not indexed
	)

	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if plan.UseIndex {
		t.Error("expected full scan when not all OR children are indexed")
	}
}

// TestOptimizeNot tests NOT filter optimization.
func TestOptimizeNot(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	filter := NewNotFilter(NewEqualityFilter("uid", []byte("alice")))
	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	// NOT filters always require full scan
	if plan.UseIndex {
		t.Error("NOT filters should always use full scan")
	}
}

// TestOptimizeRangeWithIndex tests range filter with index.
func TestOptimizeRangeWithIndex(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// uid is indexed
	filter := NewGreaterOrEqualFilter("uid", []byte("alice"))
	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if !plan.UseIndex {
		t.Error("expected index usage for range filter on indexed attribute")
	}

	// Range filters need post-filter to verify condition
	if plan.PostFilter == nil {
		t.Error("expected post-filter for range condition")
	}
}

// TestOptimizeRangeWithoutIndex tests range filter without index.
func TestOptimizeRangeWithoutIndex(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	filter := NewLessOrEqualFilter("description", []byte("test"))
	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if plan.UseIndex {
		t.Error("expected full scan for range on non-indexed attribute")
	}
}

// TestCanUseIndex tests the canUseIndex method.
func TestCanUseIndex(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Create presence and substring indexes
	im.CreateIndex("email", index.IndexPresence)
	im.CreateIndex("description", index.IndexSubstring)

	opt := NewOptimizer(im)

	tests := []struct {
		name      string
		filter    *Filter
		wantAttr  string
		wantType  index.IndexType
		wantIndex bool
	}{
		{
			name:      "equality indexed",
			filter:    NewEqualityFilter("uid", []byte("alice")),
			wantAttr:  "uid",
			wantType:  index.IndexEquality,
			wantIndex: true,
		},
		{
			name:      "equality not indexed",
			filter:    NewEqualityFilter("location", []byte("NYC")),
			wantAttr:  "",
			wantType:  0,
			wantIndex: false,
		},
		{
			name:      "presence indexed",
			filter:    NewPresentFilter("email"),
			wantAttr:  "email",
			wantType:  index.IndexPresence,
			wantIndex: true,
		},
		{
			name:      "presence not indexed",
			filter:    NewPresentFilter("location"),
			wantAttr:  "",
			wantType:  0,
			wantIndex: false,
		},
		{
			name: "substring indexed",
			filter: NewSubstringFilter(&SubstringFilter{
				Attribute: "description",
				Initial:   []byte("test"),
			}),
			wantAttr:  "description",
			wantType:  index.IndexSubstring,
			wantIndex: true,
		},
		{
			name:      "nil filter",
			filter:    nil,
			wantAttr:  "",
			wantType:  0,
			wantIndex: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr, idxType, canUse := opt.canUseIndex(tt.filter)
			if canUse != tt.wantIndex {
				t.Errorf("canUseIndex() = %v, want %v", canUse, tt.wantIndex)
			}
			if attr != tt.wantAttr {
				t.Errorf("attr = %s, want %s", attr, tt.wantAttr)
			}
			if idxType != tt.wantType {
				t.Errorf("indexType = %v, want %v", idxType, tt.wantType)
			}
		})
	}
}

// TestEstimateCost tests cost estimation.
func TestEstimateCost(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	opt := NewOptimizer(im)

	tests := []struct {
		name     string
		filter   *Filter
		wantCost int
	}{
		{
			name:     "nil filter",
			filter:   nil,
			wantCost: 0,
		},
		{
			name:     "equality",
			filter:   NewEqualityFilter("uid", []byte("alice")),
			wantCost: CostPostFilter,
		},
		{
			name:     "presence",
			filter:   NewPresentFilter("uid"),
			wantCost: CostPostFilter / 2,
		},
		{
			name: "substring",
			filter: NewSubstringFilter(&SubstringFilter{
				Attribute: "cn",
				Initial:   []byte("test"),
			}),
			wantCost: CostPostFilter * 2,
		},
		{
			name:     "approx match",
			filter:   NewApproxMatchFilter("cn", []byte("test")),
			wantCost: CostPostFilter * 3,
		},
		{
			name: "and filter",
			filter: NewAndFilter(
				NewEqualityFilter("uid", []byte("alice")),
				NewPresentFilter("mail"),
			),
			wantCost: CostPostFilter + CostPostFilter/2,
		},
		{
			name: "or filter",
			filter: NewOrFilter(
				NewEqualityFilter("uid", []byte("alice")),
				NewEqualityFilter("cn", []byte("bob")),
			),
			wantCost: CostPostFilter * 2,
		},
		{
			name:     "not filter",
			filter:   NewNotFilter(NewEqualityFilter("uid", []byte("alice"))),
			wantCost: CostPostFilter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := opt.estimateCost(tt.filter)
			if cost != tt.wantCost {
				t.Errorf("estimateCost() = %d, want %d", cost, tt.wantCost)
			}
		})
	}
}

// TestQueryPlanString tests QueryPlan string representation.
func TestQueryPlanString(t *testing.T) {
	tests := []struct {
		name string
		plan *QueryPlan
		want string
	}{
		{
			name: "full scan",
			plan: NewFullScanPlan(NewEqualityFilter("uid", []byte("alice"))),
			want: "FULL_SCAN",
		},
		{
			name: "index lookup",
			plan: NewIndexPlan("uid", index.IndexEquality, []byte("alice"), nil, 10, nil),
			want: "INDEX_LOOKUP(uid, equality)",
		},
		{
			name: "index with post-filter",
			plan: NewIndexPlan("uid", index.IndexEquality, []byte("alice"),
				NewEqualityFilter("cn", []byte("test")), 10, nil),
			want: "INDEX_LOOKUP(uid, equality) + POST_FILTER",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.plan.String()
			if got != tt.want {
				t.Errorf("String() = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestQueryPlanHelpers tests QueryPlan helper methods.
func TestQueryPlanHelpers(t *testing.T) {
	t.Run("IsFullScan", func(t *testing.T) {
		fullScan := NewFullScanPlan(nil)
		if !fullScan.IsFullScan() {
			t.Error("expected IsFullScan() = true")
		}

		indexPlan := NewIndexPlan("uid", index.IndexEquality, []byte("alice"), nil, 10, nil)
		if indexPlan.IsFullScan() {
			t.Error("expected IsFullScan() = false")
		}
	})

	t.Run("HasPostFilter", func(t *testing.T) {
		noPost := NewIndexPlan("uid", index.IndexEquality, []byte("alice"), nil, 10, nil)
		if noPost.HasPostFilter() {
			t.Error("expected HasPostFilter() = false")
		}

		withPost := NewIndexPlan("uid", index.IndexEquality, []byte("alice"),
			NewEqualityFilter("cn", []byte("test")), 10, nil)
		if !withPost.HasPostFilter() {
			t.Error("expected HasPostFilter() = true")
		}
	})
}

// TestOptimizeCaseInsensitiveAttribute tests case-insensitive attribute matching.
func TestOptimizeCaseInsensitiveAttribute(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	opt := NewOptimizer(im)

	// Test various case variations of "uid"
	cases := []string{"UID", "Uid", "uId", "uid"}
	for _, attr := range cases {
		t.Run(attr, func(t *testing.T) {
			filter := NewEqualityFilter(attr, []byte("alice"))
			plan := opt.Optimize(filter)

			if !plan.UseIndex {
				t.Errorf("expected index usage for attribute '%s'", attr)
			}
		})
	}
}

// TestOptimizeComplexAndFilter tests complex AND filter optimization.
func TestOptimizeComplexAndFilter(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Complex AND: (&(uid=alice)(cn=Alice)(objectClass=person)(description=test))
	filter := NewAndFilter(
		NewEqualityFilter("uid", []byte("alice")),
		NewEqualityFilter("cn", []byte("Alice")),
		NewEqualityFilter("objectclass", []byte("person")),
		NewEqualityFilter("description", []byte("test")), // not indexed
	)

	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if !plan.UseIndex {
		t.Error("expected index usage")
	}

	// Should have post-filter for remaining conditions
	if plan.PostFilter == nil {
		t.Error("expected post-filter")
	}

	// Post-filter should be an AND of remaining conditions
	if plan.PostFilter.Type != FilterAnd {
		t.Errorf("expected AND post-filter, got %v", plan.PostFilter.Type)
	}

	// Should have 3 children in post-filter (all except the indexed one)
	if len(plan.PostFilter.Children) != 3 {
		t.Errorf("expected 3 post-filter children, got %d", len(plan.PostFilter.Children))
	}
}

// TestOptimizeSubstringWithAnyComponent tests substring with middle component.
func TestOptimizeSubstringWithAnyComponent(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	err = im.CreateIndex("description", index.IndexSubstring)
	if err != nil {
		t.Fatalf("failed to create substring index: %v", err)
	}

	// Pattern: *admin*user*
	sf := &SubstringFilter{
		Attribute: "description",
		Initial:   nil,
		Any:       [][]byte{[]byte("admin"), []byte("user")},
		Final:     nil,
	}
	filter := NewSubstringFilter(sf)
	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if !plan.UseIndex {
		t.Error("expected index usage for substring with Any component")
	}

	// Should use "admin" as lookup (first Any component >= 3 chars)
	if string(plan.IndexLookup) != "admin" {
		t.Errorf("expected IndexLookup 'admin', got '%s'", plan.IndexLookup)
	}
}

// TestOptimizeSubstringWithFinalComponent tests substring with final component.
func TestOptimizeSubstringWithFinalComponent(t *testing.T) {
	pm, _, cleanup := testSetup(t)
	defer cleanup()

	im, err := index.NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	err = im.CreateIndex("description", index.IndexSubstring)
	if err != nil {
		t.Fatalf("failed to create substring index: %v", err)
	}

	// Pattern: *admin
	sf := &SubstringFilter{
		Attribute: "description",
		Initial:   nil,
		Any:       nil,
		Final:     []byte("admin"),
	}
	filter := NewSubstringFilter(sf)
	opt := NewOptimizer(im)
	plan := opt.Optimize(filter)

	if !plan.UseIndex {
		t.Error("expected index usage for substring with Final component")
	}

	if string(plan.IndexLookup) != "admin" {
		t.Errorf("expected IndexLookup 'admin', got '%s'", plan.IndexLookup)
	}
}
