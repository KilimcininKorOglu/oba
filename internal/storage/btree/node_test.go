package btree

import (
	"bytes"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

func TestNewInternalNode(t *testing.T) {
	pageID := storage.PageID(42)
	node := NewInternalNode(pageID)

	if node.IsLeaf {
		t.Error("internal node should not be a leaf")
	}
	if node.PageID != pageID {
		t.Errorf("expected PageID %d, got %d", pageID, node.PageID)
	}
	if len(node.Keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(node.Keys))
	}
	if len(node.Children) != 0 {
		t.Errorf("expected 0 children, got %d", len(node.Children))
	}
	if node.Values != nil {
		t.Error("internal node should have nil Values")
	}
	if node.Next != InvalidPageID {
		t.Errorf("expected Next to be InvalidPageID, got %d", node.Next)
	}
	if node.Prev != InvalidPageID {
		t.Errorf("expected Prev to be InvalidPageID, got %d", node.Prev)
	}
}

func TestNewLeafNode(t *testing.T) {
	pageID := storage.PageID(100)
	node := NewLeafNode(pageID)

	if !node.IsLeaf {
		t.Error("leaf node should be a leaf")
	}
	if node.PageID != pageID {
		t.Errorf("expected PageID %d, got %d", pageID, node.PageID)
	}
	if len(node.Keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(node.Keys))
	}
	if len(node.Values) != 0 {
		t.Errorf("expected 0 values, got %d", len(node.Values))
	}
	if node.Children != nil {
		t.Error("leaf node should have nil Children")
	}
}

func TestBPlusNodeKeyCount(t *testing.T) {
	node := NewLeafNode(1)
	if node.KeyCount() != 0 {
		t.Errorf("expected 0 keys, got %d", node.KeyCount())
	}

	node.Keys = append(node.Keys, []byte("key1"))
	node.Values = append(node.Values, EntryRef{PageID: 1, SlotID: 0})
	if node.KeyCount() != 1 {
		t.Errorf("expected 1 key, got %d", node.KeyCount())
	}
}

func TestBPlusNodeIsFull(t *testing.T) {
	// Test leaf node
	leaf := NewLeafNode(1)
	if leaf.IsFull() {
		t.Error("empty leaf should not be full")
	}

	// Fill to capacity
	for i := 0; i < BPlusLeafCapacity; i++ {
		leaf.Keys = append(leaf.Keys, []byte{byte(i)})
		leaf.Values = append(leaf.Values, EntryRef{})
	}
	if !leaf.IsFull() {
		t.Error("leaf at capacity should be full")
	}

	// Test internal node
	internal := NewInternalNode(2)
	if internal.IsFull() {
		t.Error("empty internal node should not be full")
	}

	// Fill to capacity (BPlusOrder - 1 keys)
	for i := 0; i < BPlusOrder-1; i++ {
		internal.Keys = append(internal.Keys, []byte{byte(i)})
	}
	if !internal.IsFull() {
		t.Error("internal node at capacity should be full")
	}
}

func TestBPlusNodeIsUnderflow(t *testing.T) {
	// Test leaf node
	leaf := NewLeafNode(1)
	if !leaf.IsUnderflow() {
		t.Error("empty leaf should be in underflow")
	}

	// Add minimum keys
	for i := 0; i < MinLeafKeys; i++ {
		leaf.Keys = append(leaf.Keys, []byte{byte(i)})
		leaf.Values = append(leaf.Values, EntryRef{})
	}
	if leaf.IsUnderflow() {
		t.Error("leaf with minimum keys should not be in underflow")
	}

	// Test internal node
	internal := NewInternalNode(2)
	if !internal.IsUnderflow() {
		t.Error("empty internal node should be in underflow")
	}

	for i := 0; i < MinInternalKeys; i++ {
		internal.Keys = append(internal.Keys, []byte{byte(i)})
	}
	if internal.IsUnderflow() {
		t.Error("internal node with minimum keys should not be in underflow")
	}
}

func TestBPlusNodeCanBorrow(t *testing.T) {
	leaf := NewLeafNode(1)
	if leaf.CanBorrow() {
		t.Error("empty leaf should not be able to borrow")
	}

	// Add more than minimum keys
	for i := 0; i < MinLeafKeys+1; i++ {
		leaf.Keys = append(leaf.Keys, []byte{byte(i)})
		leaf.Values = append(leaf.Values, EntryRef{})
	}
	if !leaf.CanBorrow() {
		t.Error("leaf with more than minimum keys should be able to borrow")
	}
}

func TestBPlusNodeInsertKeyAtLeaf(t *testing.T) {
	node := NewLeafNode(1)

	// Insert first key
	ref1 := EntryRef{PageID: 10, SlotID: 1}
	node.InsertKeyAt(0, []byte("key2"), &ref1, InvalidPageID)

	if len(node.Keys) != 1 || string(node.Keys[0]) != "key2" {
		t.Errorf("expected key 'key2', got %v", node.Keys)
	}
	if len(node.Values) != 1 || node.Values[0] != ref1 {
		t.Errorf("expected value %v, got %v", ref1, node.Values)
	}

	// Insert at beginning
	ref2 := EntryRef{PageID: 20, SlotID: 2}
	node.InsertKeyAt(0, []byte("key1"), &ref2, InvalidPageID)

	if len(node.Keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(node.Keys))
	}
	if string(node.Keys[0]) != "key1" || string(node.Keys[1]) != "key2" {
		t.Errorf("keys not in order: %v", node.Keys)
	}

	// Insert at end
	ref3 := EntryRef{PageID: 30, SlotID: 3}
	node.InsertKeyAt(2, []byte("key3"), &ref3, InvalidPageID)

	if len(node.Keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(node.Keys))
	}
	if string(node.Keys[2]) != "key3" {
		t.Errorf("expected 'key3' at index 2, got %s", node.Keys[2])
	}
}

func TestBPlusNodeInsertKeyAtInternal(t *testing.T) {
	node := NewInternalNode(1)

	// Add initial child
	node.Children = append(node.Children, storage.PageID(100))

	// Insert key with right child
	node.InsertKeyAt(0, []byte("key1"), nil, storage.PageID(200))

	if len(node.Keys) != 1 || string(node.Keys[0]) != "key1" {
		t.Errorf("expected key 'key1', got %v", node.Keys)
	}
	if len(node.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(node.Children))
	}
	if node.Children[0] != 100 || node.Children[1] != 200 {
		t.Errorf("children not correct: %v", node.Children)
	}
}

func TestBPlusNodeRemoveKeyAtLeaf(t *testing.T) {
	node := NewLeafNode(1)

	// Setup node with 3 keys
	node.Keys = [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	node.Values = []EntryRef{
		{PageID: 10, SlotID: 1},
		{PageID: 20, SlotID: 2},
		{PageID: 30, SlotID: 3},
	}

	// Remove middle key
	key, value, child := node.RemoveKeyAt(1)

	if string(key) != "key2" {
		t.Errorf("expected removed key 'key2', got %s", key)
	}
	if value == nil || value.PageID != 20 || value.SlotID != 2 {
		t.Errorf("expected removed value {20, 2}, got %v", value)
	}
	if child != InvalidPageID {
		t.Errorf("expected InvalidPageID for leaf, got %d", child)
	}
	if len(node.Keys) != 2 {
		t.Errorf("expected 2 keys remaining, got %d", len(node.Keys))
	}
}

func TestBPlusNodeRemoveKeyAtInternal(t *testing.T) {
	node := NewInternalNode(1)

	// Setup node with 2 keys and 3 children
	node.Keys = [][]byte{[]byte("key1"), []byte("key2")}
	node.Children = []storage.PageID{100, 200, 300}

	// Remove first key (removes child to the right of key)
	key, value, child := node.RemoveKeyAt(0)

	if string(key) != "key1" {
		t.Errorf("expected removed key 'key1', got %s", key)
	}
	if value != nil {
		t.Error("expected nil value for internal node")
	}
	if child != 200 {
		t.Errorf("expected removed child 200, got %d", child)
	}
	if len(node.Keys) != 1 {
		t.Errorf("expected 1 key remaining, got %d", len(node.Keys))
	}
	if len(node.Children) != 2 {
		t.Errorf("expected 2 children remaining, got %d", len(node.Children))
	}
}

func TestBPlusNodeFindKeyIndex(t *testing.T) {
	node := NewLeafNode(1)
	node.Keys = [][]byte{
		[]byte("apple"),
		[]byte("banana"),
		[]byte("cherry"),
		[]byte("date"),
	}

	tests := []struct {
		key       string
		wantIndex int
		wantFound bool
	}{
		{"apple", 0, true},
		{"banana", 1, true},
		{"cherry", 2, true},
		{"date", 3, true},
		{"aardvark", 0, false},  // Before first
		{"blueberry", 2, false}, // Between banana and cherry
		{"zebra", 4, false},     // After last
	}

	for _, tt := range tests {
		index, found := node.FindKeyIndex([]byte(tt.key))
		if index != tt.wantIndex || found != tt.wantFound {
			t.Errorf("FindKeyIndex(%s) = (%d, %v), want (%d, %v)",
				tt.key, index, found, tt.wantIndex, tt.wantFound)
		}
	}
}

func TestBPlusNodeGetChildForKey(t *testing.T) {
	node := NewInternalNode(1)
	node.Keys = [][]byte{[]byte("m"), []byte("t")}
	node.Children = []storage.PageID{100, 200, 300}

	// In B+ tree internal nodes:
	// - Children[0] contains keys < Keys[0]
	// - Children[1] contains keys >= Keys[0] and < Keys[1]
	// - Children[2] contains keys >= Keys[1]
	tests := []struct {
		key      string
		wantPage storage.PageID
	}{
		{"a", 100},  // Before 'm' -> Children[0]
		{"m", 200},  // At 'm' (>= Keys[0], < Keys[1]) -> Children[1]
		{"p", 200},  // Between 'm' and 't' -> Children[1]
		{"t", 300},  // At 't' (>= Keys[1]) -> Children[2]
		{"z", 300},  // After 't' -> Children[2]
	}

	for _, tt := range tests {
		got := node.GetChildForKey([]byte(tt.key))
		if got != tt.wantPage {
			t.Errorf("GetChildForKey(%s) = %d, want %d", tt.key, got, tt.wantPage)
		}
	}
}

func TestBPlusNodeSetLink(t *testing.T) {
	node := NewLeafNode(1)
	node.SetLink(storage.PageID(50), storage.PageID(100))

	if node.Prev != 50 {
		t.Errorf("expected Prev 50, got %d", node.Prev)
	}
	if node.Next != 100 {
		t.Errorf("expected Next 100, got %d", node.Next)
	}
}

func TestBPlusNodeGetFirstLastKey(t *testing.T) {
	node := NewLeafNode(1)

	// Empty node
	if node.GetFirstKey() != nil {
		t.Error("expected nil for empty node's first key")
	}
	if node.GetLastKey() != nil {
		t.Error("expected nil for empty node's last key")
	}

	// Add keys
	node.Keys = [][]byte{[]byte("first"), []byte("middle"), []byte("last")}

	if string(node.GetFirstKey()) != "first" {
		t.Errorf("expected 'first', got %s", node.GetFirstKey())
	}
	if string(node.GetLastKey()) != "last" {
		t.Errorf("expected 'last', got %s", node.GetLastKey())
	}
}

func TestCompareKeys(t *testing.T) {
	tests := []struct {
		a, b []byte
		want int
	}{
		{[]byte("a"), []byte("b"), -1},
		{[]byte("b"), []byte("a"), 1},
		{[]byte("a"), []byte("a"), 0},
		{[]byte("ab"), []byte("a"), 1},
		{[]byte("a"), []byte("ab"), -1},
		{[]byte{}, []byte{}, 0},
		{[]byte{}, []byte("a"), -1},
		{[]byte("a"), []byte{}, 1},
		{[]byte{0x00}, []byte{0x01}, -1},
		{[]byte{0xFF}, []byte{0x00}, 1},
	}

	for _, tt := range tests {
		got := CompareKeys(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("CompareKeys(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestEntryRef(t *testing.T) {
	ref := EntryRef{PageID: 12345, SlotID: 42}

	if ref.PageID != 12345 {
		t.Errorf("expected PageID 12345, got %d", ref.PageID)
	}
	if ref.SlotID != 42 {
		t.Errorf("expected SlotID 42, got %d", ref.SlotID)
	}
}

// Test key copy isolation
func TestInsertKeyAtCopiesKey(t *testing.T) {
	node := NewLeafNode(1)
	key := []byte("original")
	ref := EntryRef{PageID: 1, SlotID: 0}

	node.InsertKeyAt(0, key, &ref, InvalidPageID)

	// Modify original key
	key[0] = 'X'

	// Node's key should be unchanged
	if string(node.Keys[0]) != "original" {
		t.Errorf("key was not copied, got %s", node.Keys[0])
	}
}

// Serialization tests

func TestLeafNodeSerializeDeserialize(t *testing.T) {
	node := NewLeafNode(storage.PageID(42))
	node.Keys = [][]byte{
		[]byte("key1"),
		[]byte("key2"),
		[]byte("key3"),
	}
	node.Values = []EntryRef{
		{PageID: 100, SlotID: 1},
		{PageID: 200, SlotID: 2},
		{PageID: 300, SlotID: 3},
	}
	node.Next = storage.PageID(50)
	node.Prev = storage.PageID(30)

	// Serialize
	buf := make([]byte, storage.PageSize)
	n, err := node.Serialize(buf)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}
	if n <= 0 {
		t.Error("expected positive bytes written")
	}

	// Deserialize
	restored := &BPlusNode{}
	err = restored.Deserialize(buf, storage.PageID(42))
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	// Verify
	if !restored.IsLeaf {
		t.Error("restored node should be a leaf")
	}
	if restored.PageID != 42 {
		t.Errorf("expected PageID 42, got %d", restored.PageID)
	}
	if restored.Next != 50 {
		t.Errorf("expected Next 50, got %d", restored.Next)
	}
	if restored.Prev != 30 {
		t.Errorf("expected Prev 30, got %d", restored.Prev)
	}
	if len(restored.Keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(restored.Keys))
	}
	for i, key := range node.Keys {
		if !bytes.Equal(restored.Keys[i], key) {
			t.Errorf("key %d mismatch: got %v, want %v", i, restored.Keys[i], key)
		}
	}
	if len(restored.Values) != 3 {
		t.Errorf("expected 3 values, got %d", len(restored.Values))
	}
	for i, val := range node.Values {
		if restored.Values[i] != val {
			t.Errorf("value %d mismatch: got %v, want %v", i, restored.Values[i], val)
		}
	}
}

func TestInternalNodeSerializeDeserialize(t *testing.T) {
	node := NewInternalNode(storage.PageID(99))
	node.Keys = [][]byte{
		[]byte("separator1"),
		[]byte("separator2"),
	}
	node.Children = []storage.PageID{10, 20, 30}

	// Serialize
	buf := make([]byte, storage.PageSize)
	n, err := node.Serialize(buf)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}
	if n <= 0 {
		t.Error("expected positive bytes written")
	}

	// Deserialize
	restored := &BPlusNode{}
	err = restored.Deserialize(buf, storage.PageID(99))
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	// Verify
	if restored.IsLeaf {
		t.Error("restored node should not be a leaf")
	}
	if restored.PageID != 99 {
		t.Errorf("expected PageID 99, got %d", restored.PageID)
	}
	if len(restored.Keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(restored.Keys))
	}
	for i, key := range node.Keys {
		if !bytes.Equal(restored.Keys[i], key) {
			t.Errorf("key %d mismatch: got %v, want %v", i, restored.Keys[i], key)
		}
	}
	if len(restored.Children) != 3 {
		t.Errorf("expected 3 children, got %d", len(restored.Children))
	}
	for i, child := range node.Children {
		if restored.Children[i] != child {
			t.Errorf("child %d mismatch: got %d, want %d", i, restored.Children[i], child)
		}
	}
}

func TestSerializedSize(t *testing.T) {
	// Empty leaf node
	leaf := NewLeafNode(1)
	size := leaf.SerializedSize()
	if size != BPlusNodeHeaderSize {
		t.Errorf("empty leaf size should be %d, got %d", BPlusNodeHeaderSize, size)
	}

	// Leaf with keys
	leaf.Keys = [][]byte{[]byte("test")}
	leaf.Values = []EntryRef{{PageID: 1, SlotID: 0}}
	expectedSize := BPlusNodeHeaderSize + KeyLengthSize + 4 + EntryRefSize
	if leaf.SerializedSize() != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, leaf.SerializedSize())
	}

	// Internal node with keys
	internal := NewInternalNode(2)
	internal.Keys = [][]byte{[]byte("key")}
	internal.Children = []storage.PageID{10, 20}
	expectedSize = BPlusNodeHeaderSize + KeyLengthSize + 3 + 2*PageIDSize
	if internal.SerializedSize() != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, internal.SerializedSize())
	}
}

func TestFitsInPage(t *testing.T) {
	node := NewLeafNode(1)

	// Empty node should fit
	if !node.FitsInPage() {
		t.Error("empty node should fit in page")
	}

	// Add many keys to exceed page size
	for i := 0; i < 1000; i++ {
		largeKey := make([]byte, 100)
		node.Keys = append(node.Keys, largeKey)
		node.Values = append(node.Values, EntryRef{})
	}

	if node.FitsInPage() {
		t.Error("node with 1000 large keys should not fit in page")
	}
}

func TestSerializeToPage(t *testing.T) {
	node := NewLeafNode(storage.PageID(42))
	node.Keys = [][]byte{[]byte("key1"), []byte("key2")}
	node.Values = []EntryRef{
		{PageID: 10, SlotID: 1},
		{PageID: 20, SlotID: 2},
	}
	node.Next = storage.PageID(100)
	node.Prev = storage.PageID(50)

	page := storage.NewPage(storage.PageID(42), storage.PageTypeFree)
	err := node.SerializeToPage(page)
	if err != nil {
		t.Fatalf("SerializeToPage failed: %v", err)
	}

	// Verify page header
	if page.Header.PageType != storage.PageTypeAttrIndex {
		t.Errorf("expected PageTypeAttrIndex, got %v", page.Header.PageType)
	}
	if page.Header.ItemCount != 2 {
		t.Errorf("expected ItemCount 2, got %d", page.Header.ItemCount)
	}
	if !page.Header.IsLeaf() {
		t.Error("page should have leaf flag set")
	}
	if !page.Header.IsDirty() {
		t.Error("page should be marked dirty")
	}
}

func TestDeserializeFromPage(t *testing.T) {
	// Create and serialize a node
	original := NewLeafNode(storage.PageID(42))
	original.Keys = [][]byte{[]byte("test")}
	original.Values = []EntryRef{{PageID: 100, SlotID: 5}}

	page := storage.NewPage(storage.PageID(42), storage.PageTypeFree)
	err := original.SerializeToPage(page)
	if err != nil {
		t.Fatalf("SerializeToPage failed: %v", err)
	}

	// Deserialize
	restored := &BPlusNode{}
	err = restored.DeserializeFromPage(page)
	if err != nil {
		t.Fatalf("DeserializeFromPage failed: %v", err)
	}

	// Verify
	if !restored.IsLeaf {
		t.Error("restored node should be a leaf")
	}
	if len(restored.Keys) != 1 || string(restored.Keys[0]) != "test" {
		t.Errorf("key mismatch: got %v", restored.Keys)
	}
	if len(restored.Values) != 1 || restored.Values[0].PageID != 100 {
		t.Errorf("value mismatch: got %v", restored.Values)
	}
}

func TestNewNodeFromPage(t *testing.T) {
	// Create and serialize a node
	original := NewInternalNode(storage.PageID(55))
	original.Keys = [][]byte{[]byte("sep")}
	original.Children = []storage.PageID{10, 20}

	page := storage.NewPage(storage.PageID(55), storage.PageTypeFree)
	err := original.SerializeToPage(page)
	if err != nil {
		t.Fatalf("SerializeToPage failed: %v", err)
	}

	// Create node from page
	restored, err := NewNodeFromPage(page)
	if err != nil {
		t.Fatalf("NewNodeFromPage failed: %v", err)
	}

	if restored.IsLeaf {
		t.Error("restored node should not be a leaf")
	}
	if len(restored.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(restored.Children))
	}
}

func TestCreatePage(t *testing.T) {
	node := NewLeafNode(storage.PageID(77))
	node.Keys = [][]byte{[]byte("data")}
	node.Values = []EntryRef{{PageID: 1, SlotID: 0}}

	page, err := node.CreatePage()
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	if page.Header.PageID != 77 {
		t.Errorf("expected PageID 77, got %d", page.Header.PageID)
	}
	if page.Header.PageType != storage.PageTypeAttrIndex {
		t.Errorf("expected PageTypeAttrIndex, got %v", page.Header.PageType)
	}
}

func TestEncodeDecodeKey(t *testing.T) {
	testKeys := [][]byte{
		[]byte("simple"),
		[]byte(""),
		[]byte{0x00, 0x01, 0x02},
		make([]byte, 100),
	}

	for _, key := range testKeys {
		encoded, err := EncodeKey(key)
		if err != nil {
			t.Fatalf("EncodeKey failed: %v", err)
		}

		decoded, n, err := DecodeKey(encoded)
		if err != nil {
			t.Fatalf("DecodeKey failed: %v", err)
		}

		if n != len(encoded) {
			t.Errorf("expected %d bytes consumed, got %d", len(encoded), n)
		}

		if !bytes.Equal(decoded, key) {
			t.Errorf("key mismatch: got %v, want %v", decoded, key)
		}
	}
}

func TestEncodeKeyTooLarge(t *testing.T) {
	largeKey := make([]byte, MaxKeySize+1)
	_, err := EncodeKey(largeKey)
	if err != ErrKeyTooLarge {
		t.Errorf("expected ErrKeyTooLarge, got %v", err)
	}
}

func TestEncodeDecodeEntryRef(t *testing.T) {
	ref := EntryRef{PageID: 12345678, SlotID: 999}

	encoded := EncodeEntryRef(ref)
	if len(encoded) != EntryRefSize {
		t.Errorf("expected %d bytes, got %d", EntryRefSize, len(encoded))
	}

	decoded, err := DecodeEntryRef(encoded)
	if err != nil {
		t.Fatalf("DecodeEntryRef failed: %v", err)
	}

	if decoded != ref {
		t.Errorf("EntryRef mismatch: got %v, want %v", decoded, ref)
	}
}

func TestSerializeErrors(t *testing.T) {
	// Buffer too small
	node := NewLeafNode(1)
	node.Keys = [][]byte{[]byte("key")}
	node.Values = []EntryRef{{PageID: 1, SlotID: 0}}

	smallBuf := make([]byte, 5)
	_, err := node.Serialize(smallBuf)
	if err != ErrBufferTooSmall {
		t.Errorf("expected ErrBufferTooSmall, got %v", err)
	}

	// Key too large
	node.Keys = [][]byte{make([]byte, MaxKeySize+1)}
	largeBuf := make([]byte, storage.PageSize)
	_, err = node.Serialize(largeBuf)
	if err != ErrKeyTooLarge {
		t.Errorf("expected ErrKeyTooLarge, got %v", err)
	}
}

func TestDeserializeErrors(t *testing.T) {
	node := &BPlusNode{}

	// Buffer too small
	smallBuf := make([]byte, 5)
	err := node.Deserialize(smallBuf, 1)
	if err != ErrBufferTooSmall {
		t.Errorf("expected ErrBufferTooSmall, got %v", err)
	}
}

func TestDeserializeFromPageWrongType(t *testing.T) {
	page := storage.NewPage(1, storage.PageTypeData)
	node := &BPlusNode{}

	err := node.DeserializeFromPage(page)
	if err != ErrInvalidNodeData {
		t.Errorf("expected ErrInvalidNodeData, got %v", err)
	}
}

func TestCalculateMaxKeysForSize(t *testing.T) {
	bufSize := storage.PageSize - storage.PageHeaderSize
	avgKeySize := 20

	leafMax := CalculateMaxKeysForSize(bufSize, avgKeySize, true)
	internalMax := CalculateMaxKeysForSize(bufSize, avgKeySize, false)

	if leafMax <= 0 {
		t.Error("leaf max keys should be positive")
	}
	if internalMax <= 0 {
		t.Error("internal max keys should be positive")
	}

	// Internal nodes should fit more keys (no entry refs, just page IDs)
	// Actually, entry refs are 10 bytes, page IDs are 8 bytes, so similar
	t.Logf("Leaf max keys: %d, Internal max keys: %d", leafMax, internalMax)
}

func TestEmptyNodeSerializeDeserialize(t *testing.T) {
	// Empty leaf
	leaf := NewLeafNode(storage.PageID(1))
	buf := make([]byte, storage.PageSize)
	_, err := leaf.Serialize(buf)
	if err != nil {
		t.Fatalf("Serialize empty leaf failed: %v", err)
	}

	restored := &BPlusNode{}
	err = restored.Deserialize(buf, 1)
	if err != nil {
		t.Fatalf("Deserialize empty leaf failed: %v", err)
	}

	if !restored.IsLeaf {
		t.Error("restored should be leaf")
	}
	if len(restored.Keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(restored.Keys))
	}
}

func TestVariableLengthKeys(t *testing.T) {
	node := NewLeafNode(storage.PageID(1))

	// Add keys of varying lengths
	keys := [][]byte{
		[]byte("a"),
		[]byte("medium-length-key"),
		[]byte("this-is-a-much-longer-key-that-tests-variable-length-encoding"),
		[]byte{}, // Empty key
	}

	for i, key := range keys {
		ref := EntryRef{PageID: storage.PageID(i), SlotID: uint16(i)}
		node.InsertKeyAt(i, key, &ref, InvalidPageID)
	}

	// Serialize and deserialize
	buf := make([]byte, storage.PageSize)
	_, err := node.Serialize(buf)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	restored := &BPlusNode{}
	err = restored.Deserialize(buf, 1)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	// Verify all keys
	if len(restored.Keys) != len(keys) {
		t.Fatalf("expected %d keys, got %d", len(keys), len(restored.Keys))
	}

	for i, key := range keys {
		if !bytes.Equal(restored.Keys[i], key) {
			t.Errorf("key %d mismatch: got %v, want %v", i, restored.Keys[i], key)
		}
	}
}

func TestLeafLinkingMaintained(t *testing.T) {
	// Create a chain of leaf nodes
	leaf1 := NewLeafNode(storage.PageID(1))
	leaf2 := NewLeafNode(storage.PageID(2))
	leaf3 := NewLeafNode(storage.PageID(3))

	// Link them
	leaf1.SetLink(InvalidPageID, storage.PageID(2))
	leaf2.SetLink(storage.PageID(1), storage.PageID(3))
	leaf3.SetLink(storage.PageID(2), InvalidPageID)

	// Add some data
	leaf1.Keys = [][]byte{[]byte("a")}
	leaf1.Values = []EntryRef{{PageID: 10, SlotID: 0}}
	leaf2.Keys = [][]byte{[]byte("b")}
	leaf2.Values = []EntryRef{{PageID: 20, SlotID: 0}}
	leaf3.Keys = [][]byte{[]byte("c")}
	leaf3.Values = []EntryRef{{PageID: 30, SlotID: 0}}

	// Serialize all
	buf1 := make([]byte, storage.PageSize)
	buf2 := make([]byte, storage.PageSize)
	buf3 := make([]byte, storage.PageSize)

	leaf1.Serialize(buf1)
	leaf2.Serialize(buf2)
	leaf3.Serialize(buf3)

	// Deserialize and verify links
	r1, r2, r3 := &BPlusNode{}, &BPlusNode{}, &BPlusNode{}
	r1.Deserialize(buf1, 1)
	r2.Deserialize(buf2, 2)
	r3.Deserialize(buf3, 3)

	// Verify forward links
	if r1.Next != 2 {
		t.Errorf("leaf1.Next should be 2, got %d", r1.Next)
	}
	if r2.Next != 3 {
		t.Errorf("leaf2.Next should be 3, got %d", r2.Next)
	}
	if r3.Next != InvalidPageID {
		t.Errorf("leaf3.Next should be InvalidPageID, got %d", r3.Next)
	}

	// Verify backward links
	if r1.Prev != InvalidPageID {
		t.Errorf("leaf1.Prev should be InvalidPageID, got %d", r1.Prev)
	}
	if r2.Prev != 1 {
		t.Errorf("leaf2.Prev should be 1, got %d", r2.Prev)
	}
	if r3.Prev != 2 {
		t.Errorf("leaf3.Prev should be 2, got %d", r3.Prev)
	}
}

func TestSerializationFitsWithinPageSize(t *testing.T) {
	// Test that a reasonably filled node fits within page size
	node := NewLeafNode(storage.PageID(1))

	// Add entries until we approach capacity
	for i := 0; i < 100; i++ {
		key := []byte("key-" + string(rune('a'+i%26)))
		ref := EntryRef{PageID: storage.PageID(i), SlotID: uint16(i)}
		node.Keys = append(node.Keys, key)
		node.Values = append(node.Values, ref)
	}

	if !node.FitsInPage() {
		t.Error("node with 100 small keys should fit in page")
	}

	// Serialize to page
	page := storage.NewPage(1, storage.PageTypeFree)
	err := node.SerializeToPage(page)
	if err != nil {
		t.Fatalf("SerializeToPage failed: %v", err)
	}

	// Verify page data doesn't exceed bounds
	serializedSize := node.SerializedSize()
	availableSpace := storage.PageSize - storage.PageHeaderSize
	if serializedSize > availableSpace {
		t.Errorf("serialized size %d exceeds available space %d", serializedSize, availableSpace)
	}
}
