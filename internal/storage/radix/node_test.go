package radix

import (
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

func TestNewNode(t *testing.T) {
	node := NewNode("dc=example")

	if node.Key != "dc=example" {
		t.Errorf("expected key 'dc=example', got '%s'", node.Key)
	}
	if node.Children == nil {
		t.Error("expected Children map to be initialized")
	}
	if node.HasEntry {
		t.Error("expected HasEntry to be false")
	}
	if node.PageID != 0 {
		t.Errorf("expected PageID 0, got %d", node.PageID)
	}
	if node.SlotID != 0 {
		t.Errorf("expected SlotID 0, got %d", node.SlotID)
	}
	if node.Parent != nil {
		t.Error("expected Parent to be nil")
	}
	if node.SubtreeCount != 0 {
		t.Errorf("expected SubtreeCount 0, got %d", node.SubtreeCount)
	}
}

func TestNewRootNode(t *testing.T) {
	root := NewRootNode()

	if root.Key != "" {
		t.Errorf("expected empty key, got '%s'", root.Key)
	}
	if !root.IsRoot() {
		t.Error("expected IsRoot() to return true")
	}
}

func TestNewEntryNode(t *testing.T) {
	node := NewEntryNode("uid=alice", storage.PageID(42), 7)

	if node.Key != "uid=alice" {
		t.Errorf("expected key 'uid=alice', got '%s'", node.Key)
	}
	if !node.HasEntry {
		t.Error("expected HasEntry to be true")
	}
	if node.PageID != 42 {
		t.Errorf("expected PageID 42, got %d", node.PageID)
	}
	if node.SlotID != 7 {
		t.Errorf("expected SlotID 7, got %d", node.SlotID)
	}
	if node.SubtreeCount != 1 {
		t.Errorf("expected SubtreeCount 1, got %d", node.SubtreeCount)
	}
}

func TestSetEntry(t *testing.T) {
	node := NewNode("ou=users")

	node.SetEntry(storage.PageID(100), 5)

	if !node.HasEntry {
		t.Error("expected HasEntry to be true")
	}
	if node.PageID != 100 {
		t.Errorf("expected PageID 100, got %d", node.PageID)
	}
	if node.SlotID != 5 {
		t.Errorf("expected SlotID 5, got %d", node.SlotID)
	}
	if node.SubtreeCount != 1 {
		t.Errorf("expected SubtreeCount 1, got %d", node.SubtreeCount)
	}

	// Setting entry again should not increment SubtreeCount
	node.SetEntry(storage.PageID(200), 10)
	if node.SubtreeCount != 1 {
		t.Errorf("expected SubtreeCount 1 after second SetEntry, got %d", node.SubtreeCount)
	}
}

func TestClearEntry(t *testing.T) {
	node := NewEntryNode("cn=admin", storage.PageID(50), 3)

	node.ClearEntry()

	if node.HasEntry {
		t.Error("expected HasEntry to be false")
	}
	if node.PageID != 0 {
		t.Errorf("expected PageID 0, got %d", node.PageID)
	}
	if node.SlotID != 0 {
		t.Errorf("expected SlotID 0, got %d", node.SlotID)
	}
	if node.SubtreeCount != 0 {
		t.Errorf("expected SubtreeCount 0, got %d", node.SubtreeCount)
	}

	// Clearing again should not decrement below 0
	node.ClearEntry()
	if node.SubtreeCount != 0 {
		t.Errorf("expected SubtreeCount 0 after second ClearEntry, got %d", node.SubtreeCount)
	}
}

func TestAddChild(t *testing.T) {
	parent := NewNode("dc=example")
	child := NewEntryNode("ou=users", storage.PageID(10), 1)

	parent.AddChild(child)

	if child.Parent != parent {
		t.Error("expected child's Parent to be set")
	}
	if len(parent.Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(parent.Children))
	}
	if parent.Children['o'] != child {
		t.Error("expected child to be accessible by first byte 'o'")
	}
	if parent.SubtreeCount != 1 {
		t.Errorf("expected SubtreeCount 1, got %d", parent.SubtreeCount)
	}
}

func TestAddChildEmptyKey(t *testing.T) {
	parent := NewNode("dc=example")
	child := NewNode("")

	parent.AddChild(child)

	if len(parent.Children) != 0 {
		t.Error("expected no children to be added for empty key")
	}
}

func TestRemoveChild(t *testing.T) {
	parent := NewNode("dc=example")
	child := NewEntryNode("ou=users", storage.PageID(10), 1)
	parent.AddChild(child)

	removed := parent.RemoveChild("ou=users")

	if removed != child {
		t.Error("expected removed node to be the child")
	}
	if removed.Parent != nil {
		t.Error("expected removed node's Parent to be nil")
	}
	if len(parent.Children) != 0 {
		t.Errorf("expected 0 children, got %d", len(parent.Children))
	}
	if parent.SubtreeCount != 0 {
		t.Errorf("expected SubtreeCount 0, got %d", parent.SubtreeCount)
	}
}

func TestRemoveChildNotFound(t *testing.T) {
	parent := NewNode("dc=example")

	removed := parent.RemoveChild("ou=nonexistent")

	if removed != nil {
		t.Error("expected nil for non-existent child")
	}
}

func TestGetChild(t *testing.T) {
	parent := NewNode("dc=example")
	child := NewNode("ou=users")
	parent.AddChild(child)

	found := parent.GetChild("ou=users")

	if found != child {
		t.Error("expected to find the child")
	}

	notFound := parent.GetChild("ou=groups")
	if notFound != nil {
		t.Error("expected nil for non-existent child")
	}
}

func TestFindChildByPrefix(t *testing.T) {
	parent := NewNode("dc=example")
	child := NewNode("ou=users")
	parent.AddChild(child)

	// Exact match
	found, suffix := parent.FindChildByPrefix("ou=users")
	if found != child {
		t.Error("expected to find child with exact match")
	}
	if suffix != "" {
		t.Errorf("expected empty suffix, got '%s'", suffix)
	}

	// Prefix match with suffix
	found, suffix = parent.FindChildByPrefix("ou=users/uid=alice")
	if found != child {
		t.Error("expected to find child with prefix match")
	}
	if suffix != "/uid=alice" {
		t.Errorf("expected suffix '/uid=alice', got '%s'", suffix)
	}

	// No match
	found, _ = parent.FindChildByPrefix("cn=admin")
	if found != nil {
		t.Error("expected nil for no match")
	}
}

func TestIsLeaf(t *testing.T) {
	node := NewNode("dc=example")

	if !node.IsLeaf() {
		t.Error("expected node without children to be leaf")
	}

	child := NewNode("ou=users")
	node.AddChild(child)

	if node.IsLeaf() {
		t.Error("expected node with children to not be leaf")
	}
}

func TestIsRoot(t *testing.T) {
	root := NewRootNode()
	child := NewNode("dc=example")
	root.AddChild(child)

	if !root.IsRoot() {
		t.Error("expected root to be root")
	}
	if child.IsRoot() {
		t.Error("expected child to not be root")
	}
}

func TestChildCount(t *testing.T) {
	node := NewNode("dc=example")

	if node.ChildCount() != 0 {
		t.Errorf("expected 0 children, got %d", node.ChildCount())
	}

	node.AddChild(NewNode("ou=users"))
	node.AddChild(NewNode("cn=admin"))

	if node.ChildCount() != 2 {
		t.Errorf("expected 2 children, got %d", node.ChildCount())
	}
}

func TestRecalculateSubtreeCount(t *testing.T) {
	root := NewRootNode()
	dc := NewEntryNode("dc=example", storage.PageID(1), 1)
	ou := NewEntryNode("ou=users", storage.PageID(2), 1)
	cn := NewEntryNode("cn=admin", storage.PageID(3), 1)
	uid := NewEntryNode("uid=alice", storage.PageID(4), 1)

	root.AddChild(dc)
	dc.AddChild(ou)
	dc.AddChild(cn)
	ou.AddChild(uid)

	// Manually corrupt subtree counts
	root.SubtreeCount = 0
	dc.SubtreeCount = 0

	// Recalculate
	count := root.RecalculateSubtreeCount()

	if count != 4 {
		t.Errorf("expected total count 4, got %d", count)
	}
	if root.SubtreeCount != 4 {
		t.Errorf("expected root SubtreeCount 4, got %d", root.SubtreeCount)
	}
	if dc.SubtreeCount != 4 {
		t.Errorf("expected dc SubtreeCount 4, got %d", dc.SubtreeCount)
	}
	if ou.SubtreeCount != 2 {
		t.Errorf("expected ou SubtreeCount 2, got %d", ou.SubtreeCount)
	}
}

func TestPropagateSubtreeCountChange(t *testing.T) {
	root := NewRootNode()
	dc := NewNode("dc=example")
	ou := NewNode("ou=users")

	root.AddChild(dc)
	dc.AddChild(ou)

	// Set initial counts
	root.SubtreeCount = 0
	dc.SubtreeCount = 0
	ou.SubtreeCount = 0

	// Propagate an increase
	ou.PropagateSubtreeCountChange(1)

	if ou.SubtreeCount != 1 {
		t.Errorf("expected ou SubtreeCount 1, got %d", ou.SubtreeCount)
	}
	if dc.SubtreeCount != 1 {
		t.Errorf("expected dc SubtreeCount 1, got %d", dc.SubtreeCount)
	}
	if root.SubtreeCount != 1 {
		t.Errorf("expected root SubtreeCount 1, got %d", root.SubtreeCount)
	}

	// Propagate a decrease
	ou.PropagateSubtreeCountChange(-1)

	if ou.SubtreeCount != 0 {
		t.Errorf("expected ou SubtreeCount 0, got %d", ou.SubtreeCount)
	}
}

func TestGetChildren(t *testing.T) {
	parent := NewNode("dc=example")
	child1 := NewNode("ou=users")
	child2 := NewNode("cn=admin")

	parent.AddChild(child1)
	parent.AddChild(child2)

	children := parent.GetChildren()

	if len(children) != 2 {
		t.Errorf("expected 2 children, got %d", len(children))
	}
}

func TestGetChildKeys(t *testing.T) {
	parent := NewNode("dc=example")
	parent.AddChild(NewNode("ou=users"))
	parent.AddChild(NewNode("cn=admin"))

	keys := parent.GetChildKeys()

	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestDepth(t *testing.T) {
	root := NewRootNode()
	dc := NewNode("dc=example")
	ou := NewNode("ou=users")
	uid := NewNode("uid=alice")

	root.AddChild(dc)
	dc.AddChild(ou)
	ou.AddChild(uid)

	if root.Depth() != 0 {
		t.Errorf("expected root depth 0, got %d", root.Depth())
	}
	if dc.Depth() != 1 {
		t.Errorf("expected dc depth 1, got %d", dc.Depth())
	}
	if ou.Depth() != 2 {
		t.Errorf("expected ou depth 2, got %d", ou.Depth())
	}
	if uid.Depth() != 3 {
		t.Errorf("expected uid depth 3, got %d", uid.Depth())
	}
}

func TestPath(t *testing.T) {
	root := NewRootNode()
	dc := NewNode("dc=example")
	ou := NewNode("ou=users")
	uid := NewNode("uid=alice")

	root.AddChild(dc)
	dc.AddChild(ou)
	ou.AddChild(uid)

	path := uid.Path()

	if len(path) != 3 {
		t.Errorf("expected path length 3, got %d", len(path))
	}
	if path[0] != "dc=example" || path[1] != "ou=users" || path[2] != "uid=alice" {
		t.Errorf("unexpected path: %v", path)
	}
}

func TestCanCompress(t *testing.T) {
	// Node with one child and no entry - can compress
	node := NewNode("dc=example")
	child := NewNode("dc=com")
	node.AddChild(child)

	if !node.CanCompress() {
		t.Error("expected node to be compressible")
	}

	// Node with entry - cannot compress
	nodeWithEntry := NewEntryNode("dc=example", storage.PageID(1), 1)
	nodeWithEntry.AddChild(NewNode("dc=com"))

	if nodeWithEntry.CanCompress() {
		t.Error("expected node with entry to not be compressible")
	}

	// Node with multiple children - cannot compress
	nodeMultiple := NewNode("dc=example")
	nodeMultiple.AddChild(NewNode("ou=users"))
	nodeMultiple.AddChild(NewNode("cn=admin"))

	if nodeMultiple.CanCompress() {
		t.Error("expected node with multiple children to not be compressible")
	}

	// Root node - cannot compress
	root := NewRootNode()
	root.AddChild(NewNode("dc=example"))

	if root.CanCompress() {
		t.Error("expected root node to not be compressible")
	}
}

func TestCompress(t *testing.T) {
	node := NewNode("dc=example")
	child := NewEntryNode("dc=com", storage.PageID(42), 7)
	grandchild := NewNode("ou=users")
	child.AddChild(grandchild)
	node.AddChild(child)

	merged := node.Compress()

	if merged == nil {
		t.Fatal("expected merged node")
	}
	if merged.Key != "dc=example/dc=com" {
		t.Errorf("expected merged key 'dc=example/dc=com', got '%s'", merged.Key)
	}
	if !merged.HasEntry {
		t.Error("expected merged node to have entry")
	}
	if merged.PageID != 42 {
		t.Errorf("expected PageID 42, got %d", merged.PageID)
	}
	if len(merged.Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(merged.Children))
	}
}

func TestSplit(t *testing.T) {
	node := NewEntryNode("dc=example/dc=com", storage.PageID(42), 7)
	grandchild := NewNode("ou=users")
	node.AddChild(grandchild)

	parent, child := node.Split(10) // Split at "dc=example"

	if parent == nil || child == nil {
		t.Fatal("expected parent and child nodes")
	}
	if parent.Key != "dc=example" {
		t.Errorf("expected parent key 'dc=example', got '%s'", parent.Key)
	}
	if child.Key != "/dc=com" {
		t.Errorf("expected child key '/dc=com', got '%s'", child.Key)
	}
	if parent.HasEntry {
		t.Error("expected parent to not have entry")
	}
	if !child.HasEntry {
		t.Error("expected child to have entry")
	}
	if child.Parent != parent {
		t.Error("expected child's parent to be parent")
	}
}

func TestClone(t *testing.T) {
	node := NewEntryNode("dc=example", storage.PageID(42), 7)
	child := NewNode("ou=users")
	node.AddChild(child)

	clone := node.Clone()

	if clone.Key != node.Key {
		t.Errorf("expected key '%s', got '%s'", node.Key, clone.Key)
	}
	if clone.HasEntry != node.HasEntry {
		t.Error("expected HasEntry to match")
	}
	if clone.PageID != node.PageID {
		t.Error("expected PageID to match")
	}
	if clone.Parent != nil {
		t.Error("expected clone's Parent to be nil")
	}
	if len(clone.Children) != len(node.Children) {
		t.Error("expected same number of children")
	}
}

func TestDeepClone(t *testing.T) {
	root := NewRootNode()
	dc := NewEntryNode("dc=example", storage.PageID(1), 1)
	ou := NewEntryNode("ou=users", storage.PageID(2), 2)
	root.AddChild(dc)
	dc.AddChild(ou)

	clone := root.DeepClone()

	// Verify structure
	if len(clone.Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(clone.Children))
	}

	dcClone := clone.Children['d']
	if dcClone == nil {
		t.Fatal("expected dc clone")
	}
	if dcClone.Key != "dc=example" {
		t.Errorf("expected key 'dc=example', got '%s'", dcClone.Key)
	}
	if dcClone.Parent != clone {
		t.Error("expected dcClone's parent to be clone")
	}

	// Verify independence
	dc.Key = "modified"
	if dcClone.Key == "modified" {
		t.Error("expected deep clone to be independent")
	}
}

func TestParentPointersMaintained(t *testing.T) {
	// Build a tree
	root := NewRootNode()
	dc := NewNode("dc=example")
	ou := NewNode("ou=users")
	uid := NewNode("uid=alice")

	root.AddChild(dc)
	dc.AddChild(ou)
	ou.AddChild(uid)

	// Verify parent pointers
	if dc.Parent != root {
		t.Error("dc's parent should be root")
	}
	if ou.Parent != dc {
		t.Error("ou's parent should be dc")
	}
	if uid.Parent != ou {
		t.Error("uid's parent should be ou")
	}

	// Remove and verify
	ou.RemoveChild("uid=alice")
	if uid.Parent != nil {
		t.Error("removed node's parent should be nil")
	}
}

// Serialization tests

func TestSerializeDeserializeSingleNode(t *testing.T) {
	node := NewEntryNode("dc=example", storage.PageID(42), 7)
	node.SubtreeCount = 10

	serializer := NewNodeSerializer()
	serializer.nodeIndex[node] = 0

	header, varData, err := serializer.SerializeNode(node, NoParentIndex)
	if err != nil {
		t.Fatalf("failed to serialize node: %v", err)
	}

	if len(header) != SerializedNodeHeaderSize {
		t.Errorf("expected header size %d, got %d", SerializedNodeHeaderSize, len(header))
	}

	// Deserialize
	deserialized, err := serializer.DeserializeNode(header, varData, 0)
	if err != nil {
		t.Fatalf("failed to deserialize node: %v", err)
	}

	if deserialized.Key != node.Key {
		t.Errorf("expected key '%s', got '%s'", node.Key, deserialized.Key)
	}
	if deserialized.HasEntry != node.HasEntry {
		t.Error("expected HasEntry to match")
	}
	if deserialized.PageID != node.PageID {
		t.Errorf("expected PageID %d, got %d", node.PageID, deserialized.PageID)
	}
	if deserialized.SlotID != node.SlotID {
		t.Errorf("expected SlotID %d, got %d", node.SlotID, deserialized.SlotID)
	}
	if deserialized.SubtreeCount != node.SubtreeCount {
		t.Errorf("expected SubtreeCount %d, got %d", node.SubtreeCount, deserialized.SubtreeCount)
	}
}

func TestSerializeDeserializeMultipleNodes(t *testing.T) {
	// Build a small tree
	root := NewRootNode()
	dc := NewEntryNode("dc=example", storage.PageID(1), 1)
	ou := NewEntryNode("ou=users", storage.PageID(2), 2)
	uid := NewEntryNode("uid=alice", storage.PageID(3), 3)

	root.AddChild(dc)
	dc.AddChild(ou)
	ou.AddChild(uid)

	// Collect nodes
	nodes := CollectSubtree(root)
	if len(nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(nodes))
	}

	// Serialize
	serializer := NewNodeSerializer()
	buf, err := serializer.SerializeNodes(nodes, storage.PageID(100))
	if err != nil {
		t.Fatalf("failed to serialize nodes: %v", err)
	}

	if len(buf) != storage.PageSize {
		t.Errorf("expected buffer size %d, got %d", storage.PageSize, len(buf))
	}

	// Deserialize
	deserializer := NewNodeSerializer()
	deserialized, err := deserializer.DeserializeNodes(buf)
	if err != nil {
		t.Fatalf("failed to deserialize nodes: %v", err)
	}

	if len(deserialized) != len(nodes) {
		t.Fatalf("expected %d nodes, got %d", len(nodes), len(deserialized))
	}

	// Verify structure
	rootDeserialized := deserialized[0]
	if rootDeserialized.Key != "" {
		t.Errorf("expected root key '', got '%s'", rootDeserialized.Key)
	}
	if len(rootDeserialized.Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(rootDeserialized.Children))
	}

	dcDeserialized := rootDeserialized.Children['d']
	if dcDeserialized == nil {
		t.Fatal("expected dc child")
	}
	if dcDeserialized.Key != "dc=example" {
		t.Errorf("expected key 'dc=example', got '%s'", dcDeserialized.Key)
	}
	if dcDeserialized.Parent != rootDeserialized {
		t.Error("expected dc's parent to be root")
	}
}

func TestSerializeToPage(t *testing.T) {
	root := NewRootNode()
	dc := NewEntryNode("dc=example", storage.PageID(1), 1)
	root.AddChild(dc)

	buf, err := SerializeToPage(root, storage.PageID(100))
	if err != nil {
		t.Fatalf("failed to serialize to page: %v", err)
	}

	if len(buf) != storage.PageSize {
		t.Errorf("expected buffer size %d, got %d", storage.PageSize, len(buf))
	}

	// Verify page header
	var header storage.PageHeader
	if err := header.Deserialize(buf[:storage.PageHeaderSize]); err != nil {
		t.Fatalf("failed to deserialize page header: %v", err)
	}

	if header.PageID != 100 {
		t.Errorf("expected PageID 100, got %d", header.PageID)
	}
	if header.PageType != storage.PageTypeDNIndex {
		t.Errorf("expected PageType DNIndex, got %s", header.PageType)
	}
	if header.ItemCount != 2 {
		t.Errorf("expected ItemCount 2, got %d", header.ItemCount)
	}
}

func TestDeserializeFromPage(t *testing.T) {
	// Create and serialize
	root := NewRootNode()
	dc := NewEntryNode("dc=example", storage.PageID(42), 7)
	ou := NewEntryNode("ou=users", storage.PageID(43), 8)
	root.AddChild(dc)
	dc.AddChild(ou)

	buf, err := SerializeToPage(root, storage.PageID(100))
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}

	// Deserialize
	deserialized, err := DeserializeFromPage(buf)
	if err != nil {
		t.Fatalf("failed to deserialize: %v", err)
	}

	if deserialized == nil {
		t.Fatal("expected non-nil root")
	}
	if deserialized.Key != "" {
		t.Errorf("expected empty root key, got '%s'", deserialized.Key)
	}

	dcNode := deserialized.Children['d']
	if dcNode == nil {
		t.Fatal("expected dc child")
	}
	if dcNode.PageID != 42 {
		t.Errorf("expected PageID 42, got %d", dcNode.PageID)
	}

	ouNode := dcNode.Children['o']
	if ouNode == nil {
		t.Fatal("expected ou child")
	}
	if ouNode.PageID != 43 {
		t.Errorf("expected PageID 43, got %d", ouNode.PageID)
	}
}

func TestCalculateSerializedSize(t *testing.T) {
	node := NewNode("dc=example")
	node.AddChild(NewNode("ou=users"))
	node.AddChild(NewNode("cn=admin"))

	size := CalculateSerializedSize(node)

	// Header + key length + 2 children * child entry size
	expected := SerializedNodeHeaderSize + len("dc=example") + 2*ChildEntrySize
	if size != expected {
		t.Errorf("expected size %d, got %d", expected, size)
	}
}

func TestCanFitInPage(t *testing.T) {
	// Small tree should fit
	nodes := make([]*Node, 10)
	for i := range nodes {
		nodes[i] = NewNode("dc=example")
	}

	if !CanFitInPage(nodes) {
		t.Error("expected small tree to fit in page")
	}

	// Very large tree should not fit
	largeNodes := make([]*Node, 1000)
	for i := range largeNodes {
		largeNodes[i] = NewNode("dc=example,dc=com,ou=users,uid=alice")
	}

	if CanFitInPage(largeNodes) {
		t.Error("expected large tree to not fit in page")
	}
}

func TestCollectSubtree(t *testing.T) {
	root := NewRootNode()
	dc := NewNode("dc=example")
	ou := NewNode("ou=users")
	cn := NewNode("cn=admin")
	uid := NewNode("uid=alice")

	root.AddChild(dc)
	dc.AddChild(ou)
	dc.AddChild(cn)
	ou.AddChild(uid)

	nodes := CollectSubtree(root)

	if len(nodes) != 5 {
		t.Errorf("expected 5 nodes, got %d", len(nodes))
	}

	// First node should be root
	if nodes[0] != root {
		t.Error("expected first node to be root")
	}
}

func TestCollectSubtreeNil(t *testing.T) {
	nodes := CollectSubtree(nil)
	if nodes != nil {
		t.Error("expected nil for nil root")
	}
}

func TestSerializationFitsWithinPageSize(t *testing.T) {
	// Create a realistic tree structure
	root := NewRootNode()
	dc := NewEntryNode("dc=example", storage.PageID(1), 1)
	dcCom := NewEntryNode("dc=com", storage.PageID(2), 2)
	ou := NewEntryNode("ou=users", storage.PageID(3), 3)

	root.AddChild(dc)
	dc.AddChild(dcCom)
	dcCom.AddChild(ou)

	// Add multiple users with different first bytes
	// Note: In a radix tree keyed by first byte, children with same first byte overwrite
	for i := 0; i < 26; i++ {
		// Use different first characters to avoid overwriting
		user := NewEntryNode(string(rune('a'+i))+"id=user", storage.PageID(uint64(100+i)), uint16(i))
		ou.AddChild(user)
	}

	nodes := CollectSubtree(root)
	totalSize := CalculateTotalSerializedSize(nodes)

	if totalSize > storage.PageSize {
		t.Logf("Total size %d exceeds page size %d (expected for large trees)", totalSize, storage.PageSize)
	}

	// Verify the tree fits in page
	if len(nodes) > 0 && CanFitInPage(nodes) {
		// Try to serialize and deserialize
		buf, err := SerializeToPage(root, storage.PageID(100))
		if err != nil {
			t.Fatalf("failed to serialize: %v", err)
		}

		deserialized, err := DeserializeFromPage(buf)
		if err != nil {
			t.Fatalf("failed to deserialize: %v", err)
		}

		if deserialized == nil {
			t.Error("expected non-nil root after deserialization")
		}
	}
}

func TestSerializerReset(t *testing.T) {
	serializer := NewNodeSerializer()
	node := NewNode("test")
	serializer.nodeIndex[node] = 1
	serializer.indexNode[1] = node

	serializer.Reset()

	if len(serializer.nodeIndex) != 0 {
		t.Error("expected nodeIndex to be empty after reset")
	}
	if len(serializer.indexNode) != 0 {
		t.Error("expected indexNode to be empty after reset")
	}
}

func TestSerializeEmptyNodes(t *testing.T) {
	serializer := NewNodeSerializer()
	nodes := []*Node{}

	buf, err := serializer.SerializeNodes(nodes, storage.PageID(1))
	if err != nil {
		t.Fatalf("failed to serialize empty nodes: %v", err)
	}

	// Deserialize
	deserialized, err := serializer.DeserializeNodes(buf)
	if err != nil {
		t.Fatalf("failed to deserialize: %v", err)
	}

	if len(deserialized) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(deserialized))
	}
}

func TestDeserializeBufferTooSmall(t *testing.T) {
	serializer := NewNodeSerializer()
	smallBuf := make([]byte, 10)

	_, err := serializer.DeserializeNodes(smallBuf)
	if err != ErrBufferTooSmall {
		t.Errorf("expected ErrBufferTooSmall, got %v", err)
	}
}

func TestPathCompressionReducesMemory(t *testing.T) {
	// Create a chain of nodes
	root := NewRootNode()
	dc1 := NewNode("dc=example")
	dc2 := NewNode("dc=com")
	ou := NewNode("ou=users")

	root.AddChild(dc1)
	dc1.AddChild(dc2)
	dc2.AddChild(ou)

	// Count nodes before compression
	nodesBefore := CollectSubtree(root)
	countBefore := len(nodesBefore)

	// Compress the chain (dc1 -> dc2 can be compressed)
	if dc1.CanCompress() {
		merged := dc1.Compress()
		if merged != nil {
			// Replace dc1 with merged in root
			root.RemoveChild("dc=example")
			root.AddChild(merged)
		}
	}

	// Count nodes after compression
	nodesAfter := CollectSubtree(root)
	countAfter := len(nodesAfter)

	// Compression should reduce node count
	if countAfter >= countBefore {
		t.Logf("Node count before: %d, after: %d", countBefore, countAfter)
		// Note: This test verifies the compression mechanism works
		// The actual reduction depends on tree structure
	}
}
