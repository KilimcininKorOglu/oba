// Package benchmarks provides tools for running and reporting benchmark results.
package benchmarks

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// BenchmarkResult represents a single benchmark result.
type BenchmarkResult struct {
	// Name is the benchmark name (e.g., "BenchmarkBEREncodeInteger")
	Name string
	// Package is the package containing the benchmark
	Package string
	// Iterations is the number of iterations run
	Iterations int
	// NsPerOp is nanoseconds per operation
	NsPerOp float64
	// BytesPerOp is bytes allocated per operation
	BytesPerOp int64
	// AllocsPerOp is allocations per operation
	AllocsPerOp int64
}

// Report represents a complete benchmark report.
type Report struct {
	// Timestamp is when the report was generated
	Timestamp time.Time
	// GoVersion is the Go version used
	GoVersion string
	// OS is the operating system
	OS string
	// Arch is the CPU architecture
	Arch string
	// Results contains all benchmark results
	Results []BenchmarkResult
	// PRDTargets contains PRD performance targets
	PRDTargets map[string]PRDTarget
}

// PRDTarget represents a performance target from the PRD.
type PRDTarget struct {
	// Name is the target name
	Name string
	// Description is a human-readable description
	Description string
	// MaxNsPerOp is the maximum allowed nanoseconds per operation
	MaxNsPerOp float64
	// MinOpsPerSec is the minimum required operations per second
	MinOpsPerSec float64
}

// NewReport creates a new benchmark report.
func NewReport() *Report {
	return &Report{
		Timestamp:  time.Now(),
		Results:    make([]BenchmarkResult, 0),
		PRDTargets: defaultPRDTargets(),
	}
}

// defaultPRDTargets returns the PRD performance targets.
func defaultPRDTargets() map[string]PRDTarget {
	return map[string]PRDTarget{
		"DNLookup": {
			Name:        "Point lookup (by DN)",
			Description: "DN-based entry lookup",
			MaxNsPerOp:  10000, // < 10 us = 10,000 ns
		},
		"SearchOps": {
			Name:         "Search operations/sec",
			Description:  "Simple search operations",
			MinOpsPerSec: 50000, // 50,000+ ops/s
		},
		"WriteOps": {
			Name:         "Write throughput",
			Description:  "Write operations per second",
			MinOpsPerSec: 10000, // 10,000+ ops/s
		},
		"WALSync": {
			Name:        "WAL fsync latency",
			Description: "Write-ahead log sync latency",
			MaxNsPerOp:  1000000, // < 1 ms = 1,000,000 ns
		},
	}
}

// ParseBenchmarkOutput parses Go benchmark output and returns results.
func ParseBenchmarkOutput(r io.Reader) ([]BenchmarkResult, error) {
	var results []BenchmarkResult

	// Regex to match benchmark output lines
	// Format: BenchmarkName-N    iterations    ns/op    B/op    allocs/op
	benchRegex := regexp.MustCompile(`^(Benchmark\w+)(?:-\d+)?\s+(\d+)\s+([\d.]+)\s+ns/op(?:\s+(\d+)\s+B/op)?(?:\s+(\d+)\s+allocs/op)?`)

	scanner := bufio.NewScanner(r)
	currentPkg := ""

	for scanner.Scan() {
		line := scanner.Text()

		// Check for package line
		if strings.HasPrefix(line, "pkg:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				currentPkg = parts[1]
			}
			continue
		}

		// Try to match benchmark result
		matches := benchRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		result := BenchmarkResult{
			Name:    matches[1],
			Package: currentPkg,
		}

		// Parse iterations
		if iterations, err := strconv.Atoi(matches[2]); err == nil {
			result.Iterations = iterations
		}

		// Parse ns/op
		if nsPerOp, err := strconv.ParseFloat(matches[3], 64); err == nil {
			result.NsPerOp = nsPerOp
		}

		// Parse B/op (optional)
		if len(matches) > 4 && matches[4] != "" {
			if bytesPerOp, err := strconv.ParseInt(matches[4], 10, 64); err == nil {
				result.BytesPerOp = bytesPerOp
			}
		}

		// Parse allocs/op (optional)
		if len(matches) > 5 && matches[5] != "" {
			if allocsPerOp, err := strconv.ParseInt(matches[5], 10, 64); err == nil {
				result.AllocsPerOp = allocsPerOp
			}
		}

		results = append(results, result)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading benchmark output: %w", err)
	}

	return results, nil
}

// AddResults adds benchmark results to the report.
func (r *Report) AddResults(results []BenchmarkResult) {
	r.Results = append(r.Results, results...)
}

// SetSystemInfo sets the system information for the report.
func (r *Report) SetSystemInfo(goVersion, os, arch string) {
	r.GoVersion = goVersion
	r.OS = os
	r.Arch = arch
}

// CheckPRDTargets checks benchmark results against PRD targets.
func (r *Report) CheckPRDTargets() []TargetCheck {
	var checks []TargetCheck

	// Map benchmark names to PRD targets
	targetMappings := map[string]string{
		"BenchmarkDNLookup":   "DNLookup",
		"BenchmarkSearchBase": "SearchOps",
		"BenchmarkAdd":        "WriteOps",
		"BenchmarkWALSync":    "WALSync",
	}

	for _, result := range r.Results {
		targetKey, ok := targetMappings[result.Name]
		if !ok {
			continue
		}

		target, ok := r.PRDTargets[targetKey]
		if !ok {
			continue
		}

		check := TargetCheck{
			BenchmarkName: result.Name,
			TargetName:    target.Name,
			Description:   target.Description,
			ActualNsPerOp: result.NsPerOp,
		}

		if target.MaxNsPerOp > 0 {
			check.TargetNsPerOp = target.MaxNsPerOp
			check.Passed = result.NsPerOp <= target.MaxNsPerOp
		} else if target.MinOpsPerSec > 0 {
			actualOpsPerSec := 1e9 / result.NsPerOp
			check.ActualOpsPerSec = actualOpsPerSec
			check.TargetOpsPerSec = target.MinOpsPerSec
			check.Passed = actualOpsPerSec >= target.MinOpsPerSec
		}

		checks = append(checks, check)
	}

	return checks
}

// TargetCheck represents the result of checking a benchmark against a PRD target.
type TargetCheck struct {
	BenchmarkName   string
	TargetName      string
	Description     string
	Passed          bool
	ActualNsPerOp   float64
	TargetNsPerOp   float64
	ActualOpsPerSec float64
	TargetOpsPerSec float64
}

// GenerateTextReport generates a text report.
func (r *Report) GenerateTextReport(w io.Writer) error {
	fmt.Fprintf(w, "=== OBA Performance Benchmark Report ===\n\n")
	fmt.Fprintf(w, "Generated: %s\n", r.Timestamp.Format(time.RFC3339))
	if r.GoVersion != "" {
		fmt.Fprintf(w, "Go Version: %s\n", r.GoVersion)
	}
	if r.OS != "" && r.Arch != "" {
		fmt.Fprintf(w, "Platform: %s/%s\n", r.OS, r.Arch)
	}
	fmt.Fprintln(w)

	// Group results by package
	byPackage := make(map[string][]BenchmarkResult)
	for _, result := range r.Results {
		pkg := result.Package
		if pkg == "" {
			pkg = "unknown"
		}
		byPackage[pkg] = append(byPackage[pkg], result)
	}

	// Sort packages
	packages := make([]string, 0, len(byPackage))
	for pkg := range byPackage {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	// Print results by package
	for _, pkg := range packages {
		results := byPackage[pkg]
		fmt.Fprintf(w, "--- Package: %s ---\n\n", pkg)

		// Sort results by name
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})

		// Print header
		fmt.Fprintf(w, "%-45s %12s %12s %12s %12s\n",
			"Benchmark", "Iterations", "ns/op", "B/op", "allocs/op")
		fmt.Fprintf(w, "%s\n", strings.Repeat("-", 95))

		for _, result := range results {
			fmt.Fprintf(w, "%-45s %12d %12.2f %12d %12d\n",
				result.Name,
				result.Iterations,
				result.NsPerOp,
				result.BytesPerOp,
				result.AllocsPerOp)
		}
		fmt.Fprintln(w)
	}

	// Print PRD target checks
	checks := r.CheckPRDTargets()
	if len(checks) > 0 {
		fmt.Fprintln(w, "=== PRD Target Compliance ===")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%-30s %-20s %12s %12s %8s\n",
			"Target", "Benchmark", "Actual", "Target", "Status")
		fmt.Fprintf(w, "%s\n", strings.Repeat("-", 85))

		allPassed := true
		for _, check := range checks {
			status := "PASS"
			if !check.Passed {
				status = "FAIL"
				allPassed = false
			}

			var actual, target string
			if check.TargetNsPerOp > 0 {
				actual = formatDuration(check.ActualNsPerOp)
				target = fmt.Sprintf("< %s", formatDuration(check.TargetNsPerOp))
			} else {
				actual = formatOpsPerSec(check.ActualOpsPerSec)
				target = fmt.Sprintf(">= %s", formatOpsPerSec(check.TargetOpsPerSec))
			}

			fmt.Fprintf(w, "%-30s %-20s %12s %12s %8s\n",
				check.TargetName,
				check.BenchmarkName,
				actual,
				target,
				status)
		}

		fmt.Fprintln(w)
		if allPassed {
			fmt.Fprintln(w, "All PRD targets met!")
		} else {
			fmt.Fprintln(w, "WARNING: Some PRD targets not met!")
		}
	}

	return nil
}

// GenerateMarkdownReport generates a Markdown report.
func (r *Report) GenerateMarkdownReport(w io.Writer) error {
	fmt.Fprintln(w, "# OBA Performance Benchmark Report")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Generated: %s\n\n", r.Timestamp.Format(time.RFC3339))

	if r.GoVersion != "" || r.OS != "" {
		fmt.Fprintln(w, "## System Information")
		fmt.Fprintln(w)
		if r.GoVersion != "" {
			fmt.Fprintf(w, "- Go Version: %s\n", r.GoVersion)
		}
		if r.OS != "" && r.Arch != "" {
			fmt.Fprintf(w, "- Platform: %s/%s\n", r.OS, r.Arch)
		}
		fmt.Fprintln(w)
	}

	// Group results by package
	byPackage := make(map[string][]BenchmarkResult)
	for _, result := range r.Results {
		pkg := result.Package
		if pkg == "" {
			pkg = "unknown"
		}
		byPackage[pkg] = append(byPackage[pkg], result)
	}

	// Sort packages
	packages := make([]string, 0, len(byPackage))
	for pkg := range byPackage {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	fmt.Fprintln(w, "## Benchmark Results")
	fmt.Fprintln(w)

	for _, pkg := range packages {
		results := byPackage[pkg]
		fmt.Fprintf(w, "### %s\n\n", pkg)

		// Sort results by name
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})

		fmt.Fprintln(w, "| Benchmark | Iterations | ns/op | B/op | allocs/op |")
		fmt.Fprintln(w, "|-----------|------------|-------|------|-----------|")

		for _, result := range results {
			fmt.Fprintf(w, "| %s | %d | %.2f | %d | %d |\n",
				result.Name,
				result.Iterations,
				result.NsPerOp,
				result.BytesPerOp,
				result.AllocsPerOp)
		}
		fmt.Fprintln(w)
	}

	// Print PRD target checks
	checks := r.CheckPRDTargets()
	if len(checks) > 0 {
		fmt.Fprintln(w, "## PRD Target Compliance")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| Target | Benchmark | Actual | Target | Status |")
		fmt.Fprintln(w, "|--------|-----------|--------|--------|--------|")

		allPassed := true
		for _, check := range checks {
			status := "PASS"
			if !check.Passed {
				status = "**FAIL**"
				allPassed = false
			}

			var actual, target string
			if check.TargetNsPerOp > 0 {
				actual = formatDuration(check.ActualNsPerOp)
				target = fmt.Sprintf("< %s", formatDuration(check.TargetNsPerOp))
			} else {
				actual = formatOpsPerSec(check.ActualOpsPerSec)
				target = fmt.Sprintf(">= %s", formatOpsPerSec(check.TargetOpsPerSec))
			}

			fmt.Fprintf(w, "| %s | %s | %s | %s | %s |\n",
				check.TargetName,
				check.BenchmarkName,
				actual,
				target,
				status)
		}

		fmt.Fprintln(w)
		if allPassed {
			fmt.Fprintln(w, "All PRD targets met.")
		} else {
			fmt.Fprintln(w, "**WARNING: Some PRD targets not met!**")
		}
	}

	return nil
}

// GenerateJSONReport generates a JSON report.
func (r *Report) GenerateJSONReport(w io.Writer) error {
	fmt.Fprintln(w, "{")
	fmt.Fprintf(w, "  \"timestamp\": \"%s\",\n", r.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(w, "  \"goVersion\": \"%s\",\n", r.GoVersion)
	fmt.Fprintf(w, "  \"os\": \"%s\",\n", r.OS)
	fmt.Fprintf(w, "  \"arch\": \"%s\",\n", r.Arch)
	fmt.Fprintln(w, "  \"results\": [")

	for i, result := range r.Results {
		comma := ","
		if i == len(r.Results)-1 {
			comma = ""
		}
		fmt.Fprintf(w, "    {\"name\": \"%s\", \"package\": \"%s\", \"iterations\": %d, \"nsPerOp\": %.2f, \"bytesPerOp\": %d, \"allocsPerOp\": %d}%s\n",
			result.Name,
			result.Package,
			result.Iterations,
			result.NsPerOp,
			result.BytesPerOp,
			result.AllocsPerOp,
			comma)
	}

	fmt.Fprintln(w, "  ],")

	// PRD checks
	checks := r.CheckPRDTargets()
	fmt.Fprintln(w, "  \"prdChecks\": [")
	for i, check := range checks {
		comma := ","
		if i == len(checks)-1 {
			comma = ""
		}
		fmt.Fprintf(w, "    {\"target\": \"%s\", \"benchmark\": \"%s\", \"passed\": %t, \"actualNsPerOp\": %.2f}%s\n",
			check.TargetName,
			check.BenchmarkName,
			check.Passed,
			check.ActualNsPerOp,
			comma)
	}
	fmt.Fprintln(w, "  ]")
	fmt.Fprintln(w, "}")

	return nil
}

// SaveReport saves the report to a file.
func (r *Report) SaveReport(filename string, format string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create report file: %w", err)
	}
	defer f.Close()

	switch format {
	case "text", "txt":
		return r.GenerateTextReport(f)
	case "markdown", "md":
		return r.GenerateMarkdownReport(f)
	case "json":
		return r.GenerateJSONReport(f)
	default:
		return fmt.Errorf("unknown report format: %s", format)
	}
}

// Helper functions

func formatDuration(ns float64) string {
	if ns < 1000 {
		return fmt.Sprintf("%.2f ns", ns)
	} else if ns < 1000000 {
		return fmt.Sprintf("%.2f us", ns/1000)
	} else if ns < 1000000000 {
		return fmt.Sprintf("%.2f ms", ns/1000000)
	}
	return fmt.Sprintf("%.2f s", ns/1000000000)
}

func formatOpsPerSec(ops float64) string {
	if ops >= 1000000 {
		return fmt.Sprintf("%.2fM/s", ops/1000000)
	} else if ops >= 1000 {
		return fmt.Sprintf("%.2fK/s", ops/1000)
	}
	return fmt.Sprintf("%.2f/s", ops)
}

// RunBenchmarks runs all benchmarks and returns a report.
// This is a convenience function that can be called from tests or CLI.
func RunBenchmarks(packages []string, pattern string) (*Report, error) {
	report := NewReport()

	// Note: In a real implementation, this would execute `go test -bench`
	// and parse the output. For now, we provide the parsing infrastructure.
	fmt.Printf("To run benchmarks, execute:\n")
	fmt.Printf("  go test -bench='%s' -benchmem ./...\n", pattern)
	fmt.Printf("\nThen pipe the output to ParseBenchmarkOutput()\n")

	return report, nil
}

// Summary returns a summary of the benchmark results.
func (r *Report) Summary() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Total benchmarks: %d\n", len(r.Results)))

	// Calculate statistics
	var totalNs float64
	var totalAllocs int64
	for _, result := range r.Results {
		totalNs += result.NsPerOp
		totalAllocs += result.AllocsPerOp
	}

	if len(r.Results) > 0 {
		avgNs := totalNs / float64(len(r.Results))
		avgAllocs := float64(totalAllocs) / float64(len(r.Results))
		sb.WriteString(fmt.Sprintf("Average ns/op: %.2f\n", avgNs))
		sb.WriteString(fmt.Sprintf("Average allocs/op: %.2f\n", avgAllocs))
	}

	// PRD compliance
	checks := r.CheckPRDTargets()
	passed := 0
	for _, check := range checks {
		if check.Passed {
			passed++
		}
	}
	sb.WriteString(fmt.Sprintf("PRD targets: %d/%d passed\n", passed, len(checks)))

	return sb.String()
}
