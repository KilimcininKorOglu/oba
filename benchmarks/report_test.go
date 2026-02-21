// Package benchmarks provides tools for running and reporting benchmark results.
package benchmarks

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseBenchmarkOutput(t *testing.T) {
	input := `goos: darwin
goarch: arm64
pkg: github.com/KilimcininKorOglu/oba/internal/ber
BenchmarkBEREncodeInteger-10    2406133    51.32 ns/op    8 B/op    1 allocs/op
BenchmarkBERDecodeInteger-10    16797212   8.809 ns/op    0 B/op    0 allocs/op
PASS
ok  	github.com/KilimcininKorOglu/oba/internal/ber	8.035s`

	results, err := ParseBenchmarkOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseBenchmarkOutput failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Check first result
	if results[0].Name != "BenchmarkBEREncodeInteger" {
		t.Errorf("Expected name 'BenchmarkBEREncodeInteger', got '%s'", results[0].Name)
	}
	if results[0].Iterations != 2406133 {
		t.Errorf("Expected iterations 2406133, got %d", results[0].Iterations)
	}
	if results[0].NsPerOp < 51.0 || results[0].NsPerOp > 52.0 {
		t.Errorf("Expected ns/op ~51.32, got %f", results[0].NsPerOp)
	}
	if results[0].BytesPerOp != 8 {
		t.Errorf("Expected bytes/op 8, got %d", results[0].BytesPerOp)
	}
	if results[0].AllocsPerOp != 1 {
		t.Errorf("Expected allocs/op 1, got %d", results[0].AllocsPerOp)
	}
}

func TestNewReport(t *testing.T) {
	report := NewReport()

	if report == nil {
		t.Fatal("NewReport returned nil")
	}

	if report.Timestamp.IsZero() {
		t.Error("Report timestamp should not be zero")
	}

	if len(report.PRDTargets) == 0 {
		t.Error("Report should have PRD targets")
	}
}

func TestReportAddResults(t *testing.T) {
	report := NewReport()

	results := []BenchmarkResult{
		{Name: "BenchmarkTest1", NsPerOp: 100.0},
		{Name: "BenchmarkTest2", NsPerOp: 200.0},
	}

	report.AddResults(results)

	if len(report.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(report.Results))
	}
}

func TestReportSetSystemInfo(t *testing.T) {
	report := NewReport()
	report.SetSystemInfo("go1.22", "darwin", "arm64")

	if report.GoVersion != "go1.22" {
		t.Errorf("Expected GoVersion 'go1.22', got '%s'", report.GoVersion)
	}
	if report.OS != "darwin" {
		t.Errorf("Expected OS 'darwin', got '%s'", report.OS)
	}
	if report.Arch != "arm64" {
		t.Errorf("Expected Arch 'arm64', got '%s'", report.Arch)
	}
}

func TestReportCheckPRDTargets(t *testing.T) {
	report := NewReport()

	// Add a result that should pass the DN lookup target
	report.AddResults([]BenchmarkResult{
		{Name: "BenchmarkDNLookup", NsPerOp: 5000.0}, // 5 us < 10 us target
	})

	checks := report.CheckPRDTargets()

	if len(checks) != 1 {
		t.Fatalf("Expected 1 check, got %d", len(checks))
	}

	if !checks[0].Passed {
		t.Error("DN lookup check should pass (5 us < 10 us)")
	}

	// Test failing case
	report2 := NewReport()
	report2.AddResults([]BenchmarkResult{
		{Name: "BenchmarkDNLookup", NsPerOp: 15000.0}, // 15 us > 10 us target
	})

	checks2 := report2.CheckPRDTargets()
	if len(checks2) != 1 {
		t.Fatalf("Expected 1 check, got %d", len(checks2))
	}

	if checks2[0].Passed {
		t.Error("DN lookup check should fail (15 us > 10 us)")
	}
}

func TestGenerateTextReport(t *testing.T) {
	report := NewReport()
	report.SetSystemInfo("go1.22", "darwin", "arm64")
	report.AddResults([]BenchmarkResult{
		{Name: "BenchmarkTest", Package: "test/pkg", Iterations: 1000, NsPerOp: 100.0, BytesPerOp: 8, AllocsPerOp: 1},
	})

	var buf bytes.Buffer
	err := report.GenerateTextReport(&buf)
	if err != nil {
		t.Fatalf("GenerateTextReport failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "OBA Performance Benchmark Report") {
		t.Error("Report should contain title")
	}
	if !strings.Contains(output, "go1.22") {
		t.Error("Report should contain Go version")
	}
	if !strings.Contains(output, "BenchmarkTest") {
		t.Error("Report should contain benchmark name")
	}
}

func TestGenerateMarkdownReport(t *testing.T) {
	report := NewReport()
	report.SetSystemInfo("go1.22", "darwin", "arm64")
	report.AddResults([]BenchmarkResult{
		{Name: "BenchmarkTest", Package: "test/pkg", Iterations: 1000, NsPerOp: 100.0, BytesPerOp: 8, AllocsPerOp: 1},
	})

	var buf bytes.Buffer
	err := report.GenerateMarkdownReport(&buf)
	if err != nil {
		t.Fatalf("GenerateMarkdownReport failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "# OBA Performance Benchmark Report") {
		t.Error("Report should contain markdown title")
	}
	if !strings.Contains(output, "| Benchmark |") {
		t.Error("Report should contain markdown table")
	}
}

func TestGenerateJSONReport(t *testing.T) {
	report := NewReport()
	report.SetSystemInfo("go1.22", "darwin", "arm64")
	report.AddResults([]BenchmarkResult{
		{Name: "BenchmarkTest", Package: "test/pkg", Iterations: 1000, NsPerOp: 100.0, BytesPerOp: 8, AllocsPerOp: 1},
	})

	var buf bytes.Buffer
	err := report.GenerateJSONReport(&buf)
	if err != nil {
		t.Fatalf("GenerateJSONReport failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, `"goVersion": "go1.22"`) {
		t.Error("Report should contain Go version in JSON")
	}
	if !strings.Contains(output, `"name": "BenchmarkTest"`) {
		t.Error("Report should contain benchmark name in JSON")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		ns       float64
		expected string
	}{
		{100.0, "100.00 ns"},
		{1500.0, "1.50 us"},
		{1500000.0, "1.50 ms"},
		{1500000000.0, "1.50 s"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.ns)
		if result != tt.expected {
			t.Errorf("formatDuration(%f) = %s, expected %s", tt.ns, result, tt.expected)
		}
	}
}

func TestFormatOpsPerSec(t *testing.T) {
	tests := []struct {
		ops      float64
		expected string
	}{
		{500.0, "500.00/s"},
		{5000.0, "5.00K/s"},
		{5000000.0, "5.00M/s"},
	}

	for _, tt := range tests {
		result := formatOpsPerSec(tt.ops)
		if result != tt.expected {
			t.Errorf("formatOpsPerSec(%f) = %s, expected %s", tt.ops, result, tt.expected)
		}
	}
}

func TestReportSummary(t *testing.T) {
	report := NewReport()
	report.AddResults([]BenchmarkResult{
		{Name: "BenchmarkTest1", NsPerOp: 100.0, AllocsPerOp: 1},
		{Name: "BenchmarkTest2", NsPerOp: 200.0, AllocsPerOp: 2},
	})

	summary := report.Summary()

	if !strings.Contains(summary, "Total benchmarks: 2") {
		t.Error("Summary should contain total benchmarks count")
	}
	if !strings.Contains(summary, "Average ns/op: 150.00") {
		t.Error("Summary should contain average ns/op")
	}
}
