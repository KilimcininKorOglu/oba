package filter

import (
	"testing"
)

func TestMatchEquality(t *testing.T) {
	tests := []struct {
		name     string
		a        []byte
		b        []byte
		expected bool
	}{
		{"exact match", []byte("hello"), []byte("hello"), true},
		{"case insensitive", []byte("Hello"), []byte("hello"), true},
		{"case insensitive reverse", []byte("hello"), []byte("HELLO"), true},
		{"no match", []byte("hello"), []byte("world"), false},
		{"empty strings", []byte(""), []byte(""), true},
		{"one empty", []byte("hello"), []byte(""), false},
		{"unicode match", []byte("héllo"), []byte("HÉLLO"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchEquality(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMatchEqualityExact(t *testing.T) {
	tests := []struct {
		name     string
		a        []byte
		b        []byte
		expected bool
	}{
		{"exact match", []byte("hello"), []byte("hello"), true},
		{"case sensitive no match", []byte("Hello"), []byte("hello"), false},
		{"no match", []byte("hello"), []byte("world"), false},
		{"empty strings", []byte(""), []byte(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchEqualityExact(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMatchSubstring(t *testing.T) {
	tests := []struct {
		name     string
		value    []byte
		initial  []byte
		any      [][]byte
		final    []byte
		expected bool
	}{
		{"initial match", []byte("hello world"), []byte("hello"), nil, nil, true},
		{"initial no match", []byte("hello world"), []byte("world"), nil, nil, false},
		{"final match", []byte("hello world"), nil, nil, []byte("world"), true},
		{"final no match", []byte("hello world"), nil, nil, []byte("hello"), false},
		{"any match", []byte("hello world"), nil, [][]byte{[]byte("lo wo")}, nil, true},
		{"any no match", []byte("hello world"), nil, [][]byte{[]byte("xyz")}, nil, false},
		{"all components match", []byte("hello world"), []byte("hello"), [][]byte{[]byte(" ")}, []byte("world"), true},
		{"case insensitive initial", []byte("Hello World"), []byte("hello"), nil, nil, true},
		{"case insensitive final", []byte("Hello World"), nil, nil, []byte("WORLD"), true},
		{"case insensitive any", []byte("Hello World"), nil, [][]byte{[]byte("LO WO")}, nil, true},
		{"empty initial", []byte("hello"), []byte(""), nil, nil, true},
		{"empty final", []byte("hello"), nil, nil, []byte(""), true},
		{"empty any", []byte("hello"), nil, [][]byte{[]byte("")}, nil, true},
		{"multiple any", []byte("a-b-c-d"), nil, [][]byte{[]byte("-"), []byte("-"), []byte("-")}, nil, true},
		{"multiple any partial match", []byte("a-b-c"), nil, [][]byte{[]byte("-"), []byte("-"), []byte("-")}, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchSubstring(tt.value, tt.initial, tt.any, tt.final)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMatchGreaterOrEqual(t *testing.T) {
	tests := []struct {
		name      string
		value     []byte
		threshold []byte
		expected  bool
	}{
		{"equal", []byte("hello"), []byte("hello"), true},
		{"greater", []byte("world"), []byte("hello"), true},
		{"less", []byte("apple"), []byte("hello"), false},
		{"case insensitive equal", []byte("Hello"), []byte("hello"), true},
		{"empty strings", []byte(""), []byte(""), true},
		{"value empty", []byte(""), []byte("a"), false},
		{"threshold empty", []byte("a"), []byte(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchGreaterOrEqual(tt.value, tt.threshold)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMatchLessOrEqual(t *testing.T) {
	tests := []struct {
		name      string
		value     []byte
		threshold []byte
		expected  bool
	}{
		{"equal", []byte("hello"), []byte("hello"), true},
		{"less", []byte("apple"), []byte("hello"), true},
		{"greater", []byte("world"), []byte("hello"), false},
		{"case insensitive equal", []byte("Hello"), []byte("hello"), true},
		{"empty strings", []byte(""), []byte(""), true},
		{"value empty", []byte(""), []byte("a"), true},
		{"threshold empty", []byte("a"), []byte(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchLessOrEqual(tt.value, tt.threshold)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMatchApprox(t *testing.T) {
	tests := []struct {
		name     string
		a        []byte
		b        []byte
		expected bool
	}{
		{"exact match", []byte("hello"), []byte("hello"), true},
		{"case insensitive", []byte("Hello"), []byte("hello"), true},
		{"whitespace normalized", []byte("hello  world"), []byte("hello world"), true},
		{"multiple whitespace", []byte("hello   world"), []byte("hello world"), true},
		{"tabs normalized", []byte("hello\tworld"), []byte("hello world"), true},
		{"newlines normalized", []byte("hello\nworld"), []byte("hello world"), true},
		{"leading/trailing whitespace", []byte("  hello  "), []byte("hello"), true},
		{"no match", []byte("hello"), []byte("world"), false},
		{"empty strings", []byte(""), []byte(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchApprox(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNormalizeForApprox(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{"simple", []byte("hello"), []byte("hello")},
		{"uppercase", []byte("HELLO"), []byte("hello")},
		{"multiple spaces", []byte("hello  world"), []byte("hello world")},
		{"tabs", []byte("hello\tworld"), []byte("hello world")},
		{"newlines", []byte("hello\nworld"), []byte("hello world")},
		{"leading space", []byte("  hello"), []byte("hello")},
		{"trailing space", []byte("hello  "), []byte("hello")},
		{"mixed whitespace", []byte("  hello \t world  \n"), []byte("hello world")},
		{"empty", []byte(""), []byte("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeForApprox(tt.input)
			if string(result) != string(tt.expected) {
				t.Errorf("expected %q, got %q", string(tt.expected), string(result))
			}
		})
	}
}

func TestNormalizeAttributeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase", "uid", "uid"},
		{"uppercase", "UID", "uid"},
		{"mixed case", "objectClass", "objectclass"},
		{"with numbers", "cn2", "cn2"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeAttributeName(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
