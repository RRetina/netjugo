package netjugo

import (
	"bytes"
	"strings"
	"testing"

	"github.com/holiman/uint256"
)

// Test memory stats functionality
func TestMemoryStats(t *testing.T) {
	pa := NewPrefixAggregator()

	// Add some prefixes
	prefixes := []string{
		"192.168.1.0/24",
		"10.0.0.0/16",
		"2001:db8::/32",
	}

	err := pa.AddPrefixes(prefixes)
	if err != nil {
		t.Fatalf("Failed to add prefixes: %v", err)
	}

	// Get memory stats
	stats := pa.GetMemoryStats()

	// Verify stats are populated
	if stats.AggregatorBytes == 0 {
		t.Error("Expected non-zero aggregator bytes")
	}

	if stats.AllocBytes == 0 {
		t.Error("Expected non-zero allocated bytes")
	}
}

// Test validation functions
func TestValidationFunctions(t *testing.T) {
	// Test validatePrefixLength
	err := validatePrefixLength(true, 24)
	if err != nil {
		t.Errorf("Valid IPv4 prefix length failed: %v", err)
	}

	err = validatePrefixLength(true, 33)
	if err == nil {
		t.Error("Expected error for invalid IPv4 prefix length")
	}

	err = validatePrefixLength(false, 64)
	if err != nil {
		t.Errorf("Valid IPv6 prefix length failed: %v", err)
	}

	err = validatePrefixLength(false, 129)
	if err == nil {
		t.Error("Expected error for invalid IPv6 prefix length")
	}

	// Test isValidIPPrefix
	if !isValidIPPrefix("192.168.1.0/24") {
		t.Error("Valid prefix reported as invalid")
	}

	if isValidIPPrefix("invalid-prefix") {
		t.Error("Invalid prefix reported as valid")
	}
}

// Test IPv6 range to prefix conversion
func TestIPv6RangeToPrefix(t *testing.T) {
	// Test exact single address
	addr := "2001:db8::1"
	prefix, err := parseIPPrefix(addr + "/128")
	if err != nil {
		t.Fatalf("Failed to parse prefix: %v", err)
	}

	resultPrefix, err := uint256RangeToPrefix(prefix.Min, prefix.Max, false)
	if err != nil {
		t.Errorf("Failed to convert range to prefix: %v", err)
	}

	if resultPrefix.String() != addr+"/128" {
		t.Errorf("Expected %s/128, got %s", addr, resultPrefix.String())
	}
}

// Test power of two function
func TestIsPowerOfTwo(t *testing.T) {
	tests := []struct {
		value    uint64
		expected bool
	}{
		{0, false},
		{1, true},
		{2, true},
		{3, false},
		{4, true},
		{16, true},
		{17, false},
		{256, true},
	}

	for _, tt := range tests {
		n := uint256.NewInt(tt.value)
		result := isPowerOfTwo(n)
		if result != tt.expected {
			t.Errorf("isPowerOfTwo(%d) = %v, expected %v", tt.value, result, tt.expected)
		}
	}
}

// Test WriteToWriter functionality
func TestWriteToWriterFunc(t *testing.T) {
	pa := NewPrefixAggregator()

	prefixes := []string{
		"192.168.1.0/24",
		"10.0.0.0/16",
		"2001:db8::/32",
	}

	err := pa.AddPrefixes(prefixes)
	if err != nil {
		t.Fatalf("Failed to add prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	// Write to buffer
	var buf bytes.Buffer
	err = pa.WriteToWriter(&buf)
	if err != nil {
		t.Fatalf("Failed to write to buffer: %v", err)
	}

	// Verify output
	output := buf.String()
	if !strings.Contains(output, "192.168.1.0/24") {
		t.Error("Expected output to contain IPv4 prefix")
	}

	if !strings.Contains(output, "2001:db8::/32") {
		t.Error("Expected output to contain IPv6 prefix")
	}
}

// Test trimOverlap function
func TestTrimOverlap(t *testing.T) {
	pa := NewPrefixAggregator()

	// Create two overlapping prefixes
	prefix1, _ := parseIPPrefix("192.168.0.0/24")
	prefix2, _ := parseIPPrefix("192.168.0.128/25")

	// Test trim overlap
	result, err := pa.trimOverlapNew(prefix1, prefix2, true)
	if err != nil {
		t.Fatalf("Failed to trim overlap: %v", err)
	}

	// Should return a list of prefixes that don't overlap with prefix2
	for _, res := range result {
		if overlaps(res, prefix2) {
			t.Error("Result still overlaps with excluded prefix")
		}
	}
}

// Test edge cases in exclusion processing
func TestExclusionEdgeCases(t *testing.T) {
	pa := NewPrefixAggregator()

	// Add IPv6 prefixes
	err := pa.AddPrefixes([]string{
		"2001:db8::/32",
		"2001:db8:1::/48",
		"2001:db8:2::/48",
	})
	if err != nil {
		t.Fatalf("Failed to add prefixes: %v", err)
	}

	// Set exclusion
	err = pa.SetExcludePrefixes([]string{"2001:db8:1::/48"})
	if err != nil {
		t.Fatalf("Failed to set exclude prefixes: %v", err)
	}

	// Aggregate
	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	// Check results
	results := pa.GetIPv6Prefixes()
	for _, prefix := range results {
		if prefix == "2001:db8:1::/48" {
			t.Error("Excluded prefix found in results")
		}
	}
}

// Test concurrent operations
func TestConcurrentOperations(t *testing.T) {
	pa := NewPrefixAggregator()

	// Add some initial data
	_ = pa.AddPrefixes([]string{"192.168.1.0/24", "10.0.0.0/16"})

	// Run concurrent operations
	done := make(chan bool, 3)

	// Reader 1
	go func() {
		for i := 0; i < 100; i++ {
			_ = pa.GetPrefixes()
		}
		done <- true
	}()

	// Reader 2
	go func() {
		for i := 0; i < 100; i++ {
			_ = pa.GetStats()
		}
		done <- true
	}()

	// Writer
	go func() {
		for i := 0; i < 10; i++ {
			_ = pa.AddPrefix("172.16.0.0/16")
			_ = pa.Reset()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}
}
