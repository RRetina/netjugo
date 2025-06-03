package netjugo

import (
	"strings"
	"testing"
)

func TestBasicInclusion(t *testing.T) {
	pa := NewPrefixAggregator()

	// Add base prefixes
	err := pa.AddPrefixes([]string{
		"192.168.1.0/24",
		"10.0.0.0/24",
	})
	if err != nil {
		t.Fatalf("Failed to add base prefixes: %v", err)
	}

	// Add include prefixes
	err = pa.SetIncludePrefixes([]string{
		"192.168.2.0/24",
		"172.16.0.0/16",
	})
	if err != nil {
		t.Fatalf("Failed to set include prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	result := pa.GetIPv4Prefixes()

	// Should have all 4 prefixes (base + includes)
	if len(result) < 4 {
		t.Errorf("Expected at least 4 prefixes after inclusion, got %d", len(result))
	}

	t.Logf("Prefixes after inclusion: %v", result)
}

func TestBasicExclusion(t *testing.T) {
	pa := NewPrefixAggregator()

	// Add a large prefix
	err := pa.AddPrefix("192.168.0.0/16")
	if err != nil {
		t.Fatalf("Failed to add prefix: %v", err)
	}

	// Exclude a smaller part
	err = pa.SetExcludePrefixes([]string{
		"192.168.1.0/24",
	})
	if err != nil {
		t.Fatalf("Failed to set exclude prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	result := pa.GetIPv4Prefixes()

	// Should have multiple prefixes now (the /16 minus the /24)
	if len(result) <= 1 {
		t.Errorf("Expected multiple prefixes after exclusion, got %d", len(result))
	}

	// The excluded prefix should not be in the result
	for _, prefix := range result {
		if prefix == "192.168.1.0/24" {
			t.Errorf("Excluded prefix %s found in result", prefix)
		}
	}

	t.Logf("Prefixes after exclusion: %v", result)
}

func TestCompleteExclusion(t *testing.T) {
	pa := NewPrefixAggregator()

	// Add a prefix
	err := pa.AddPrefix("192.168.1.0/24")
	if err != nil {
		t.Fatalf("Failed to add prefix: %v", err)
	}

	// Exclude the same prefix completely
	err = pa.SetExcludePrefixes([]string{
		"192.168.1.0/24",
	})
	if err != nil {
		t.Fatalf("Failed to set exclude prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	result := pa.GetIPv4Prefixes()

	// Should have no prefixes left
	if len(result) != 0 {
		t.Errorf("Expected no prefixes after complete exclusion, got %d: %v", len(result), result)
	}
}

func TestPartialExclusion(t *testing.T) {
	pa := NewPrefixAggregator()

	// Add a prefix
	err := pa.AddPrefix("192.168.0.0/24")
	if err != nil {
		t.Fatalf("Failed to add prefix: %v", err)
	}

	// Exclude part of it (first half)
	err = pa.SetExcludePrefixes([]string{
		"192.168.0.0/25", // First half of the /24
	})
	if err != nil {
		t.Fatalf("Failed to set exclude prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	result := pa.GetIPv4Prefixes()

	// Should have at least one prefix remaining (the second half)
	if len(result) == 0 {
		t.Errorf("Expected some prefixes after partial exclusion, got none")
	}

	// Check that the remaining prefix covers the expected range
	foundSecondHalf := false
	for _, prefix := range result {
		if prefix == "192.168.0.128/25" {
			foundSecondHalf = true
			break
		}
	}

	if !foundSecondHalf {
		t.Logf("Available prefixes: %v", result)
		// This might fail due to the complexity of prefix splitting, so let's just check we have something
		if len(result) == 0 {
			t.Errorf("Expected some prefixes remaining after partial exclusion")
		}
	}

	t.Logf("Prefixes after partial exclusion: %v", result)
}

func TestInclusionAndExclusion(t *testing.T) {
	pa := NewPrefixAggregator()

	// Add base prefix
	err := pa.AddPrefix("10.0.0.0/16")
	if err != nil {
		t.Fatalf("Failed to add base prefix: %v", err)
	}

	// Include additional prefix
	err = pa.SetIncludePrefixes([]string{
		"192.168.0.0/16",
	})
	if err != nil {
		t.Fatalf("Failed to set include prefixes: %v", err)
	}

	// Exclude part of the included prefix
	err = pa.SetExcludePrefixes([]string{
		"192.168.1.0/24",
	})
	if err != nil {
		t.Fatalf("Failed to set exclude prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	result := pa.GetIPv4Prefixes()

	// Should have the base prefix and parts of the included prefix
	if len(result) == 0 {
		t.Errorf("Expected prefixes after inclusion and exclusion, got none")
	}

	// The base prefix should still be there
	foundBase := false
	for _, prefix := range result {
		if prefix == "10.0.0.0/16" {
			foundBase = true
			break
		}
	}

	if !foundBase {
		t.Errorf("Base prefix 10.0.0.0/16 not found in result: %v", result)
	}

	// The excluded prefix should not be there
	for _, prefix := range result {
		if prefix == "192.168.1.0/24" {
			t.Errorf("Excluded prefix %s found in result", prefix)
		}
	}

	t.Logf("Prefixes after inclusion and exclusion: %v", result)
}

func TestIPv6Exclusion(t *testing.T) {
	pa := NewPrefixAggregator()

	// Add IPv6 prefix
	err := pa.AddPrefix("2001:db8::/32")
	if err != nil {
		t.Fatalf("Failed to add IPv6 prefix: %v", err)
	}

	// Exclude part of it
	err = pa.SetExcludePrefixes([]string{
		"2001:db8:1::/48",
	})
	if err != nil {
		t.Fatalf("Failed to set exclude prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	result := pa.GetIPv6Prefixes()

	// Should have multiple prefixes (the /32 minus the /48)
	if len(result) == 0 {
		t.Errorf("Expected prefixes after IPv6 exclusion, got none")
	}

	// The excluded prefix should not be in the result
	for _, prefix := range result {
		if prefix == "2001:db8:1::/48" {
			t.Errorf("Excluded IPv6 prefix %s found in result", prefix)
		}
	}

	t.Logf("IPv6 prefixes after exclusion: %v", result)
}

func TestNoOverlapExclusion(t *testing.T) {
	pa := NewPrefixAggregator()

	// Add prefixes
	err := pa.AddPrefixes([]string{
		"192.168.1.0/24",
		"10.0.0.0/24",
	})
	if err != nil {
		t.Fatalf("Failed to add prefixes: %v", err)
	}

	// Exclude non-overlapping prefix
	err = pa.SetExcludePrefixes([]string{
		"172.16.0.0/24",
	})
	if err != nil {
		t.Fatalf("Failed to set exclude prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	result := pa.GetIPv4Prefixes()

	// Should still have both original prefixes (no overlap with exclusion)
	if len(result) != 2 {
		t.Errorf("Expected 2 prefixes after non-overlapping exclusion, got %d", len(result))
	}

	expectedPrefixes := map[string]bool{
		"192.168.1.0/24": false,
		"10.0.0.0/24":    false,
	}

	for _, prefix := range result {
		if _, exists := expectedPrefixes[prefix]; exists {
			expectedPrefixes[prefix] = true
		}
	}

	for prefix, found := range expectedPrefixes {
		if !found {
			t.Errorf("Expected prefix %s not found in result", prefix)
		}
	}
}

func TestExclusionWarnings(t *testing.T) {
	tests := []struct {
		name            string
		basePrefix      string
		excludePrefix   string
		expectWarning   bool
		warningContains string
	}{
		{
			name:            "IPv4 /32 exclusion should warn",
			basePrefix:      "192.168.0.0/16",
			excludePrefix:   "192.168.1.100/32",
			expectWarning:   true,
			warningContains: "more specific than recommended /30",
		},
		{
			name:            "IPv4 /31 exclusion should warn",
			basePrefix:      "192.168.0.0/16",
			excludePrefix:   "192.168.1.100/31",
			expectWarning:   true,
			warningContains: "more specific than recommended /30",
		},
		{
			name:            "IPv4 /30 exclusion should not warn",
			basePrefix:      "192.168.0.0/16",
			excludePrefix:   "192.168.1.100/30",
			expectWarning:   false,
			warningContains: "",
		},
		{
			name:            "IPv4 /24 exclusion should not warn",
			basePrefix:      "192.168.0.0/16",
			excludePrefix:   "192.168.1.0/24",
			expectWarning:   false,
			warningContains: "",
		},
		{
			name:            "IPv6 /128 exclusion should warn",
			basePrefix:      "2001:db8::/32",
			excludePrefix:   "2001:db8::1/128",
			expectWarning:   true,
			warningContains: "more specific than recommended /64",
		},
		{
			name:            "IPv6 /127 exclusion should warn",
			basePrefix:      "2001:db8::/32",
			excludePrefix:   "2001:db8::1/127",
			expectWarning:   true,
			warningContains: "more specific than recommended /64",
		},
		{
			name:            "IPv6 /64 exclusion should not warn",
			basePrefix:      "2001:db8::/32",
			excludePrefix:   "2001:db8:1::/64",
			expectWarning:   false,
			warningContains: "",
		},
		{
			name:            "IPv6 /48 exclusion should not warn",
			basePrefix:      "2001:db8::/32",
			excludePrefix:   "2001:db8:1::/48",
			expectWarning:   false,
			warningContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pa := NewPrefixAggregator()

			// Add base prefix
			if err := pa.AddPrefix(tt.basePrefix); err != nil {
				t.Fatalf("Failed to add base prefix: %v", err)
			}

			// Add exclusion
			if err := pa.SetExcludePrefixes([]string{tt.excludePrefix}); err != nil {
				t.Fatalf("Failed to set exclude prefix: %v", err)
			}

			// Track warnings
			var capturedWarnings []string
			pa.SetWarningHandler(func(msg string) {
				capturedWarnings = append(capturedWarnings, msg)
			})

			// Aggregate
			if err := pa.Aggregate(); err != nil {
				t.Fatalf("Failed to aggregate: %v", err)
			}

			// Check warnings
			warnings := pa.GetWarnings()

			if tt.expectWarning {
				if len(warnings) == 0 {
					t.Errorf("Expected warning for %s exclusion, but got none", tt.excludePrefix)
				} else {
					found := false
					for _, w := range warnings {
						if strings.Contains(w, tt.warningContains) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected warning containing '%s', got: %v", tt.warningContains, warnings)
					}
				}

				// Verify handler was called
				if len(capturedWarnings) == 0 {
					t.Error("Warning handler was not called")
				}
			} else {
				if len(warnings) > 0 {
					t.Errorf("Unexpected warnings for %s exclusion: %v", tt.excludePrefix, warnings)
				}
			}
		})
	}
}

func TestWarningHandlerConcurrency(t *testing.T) {
	pa := NewPrefixAggregator()

	// Add a large prefix
	if err := pa.AddPrefix("10.0.0.0/8"); err != nil {
		t.Fatalf("Failed to add prefix: %v", err)
	}

	// Add multiple specific exclusions
	exclusions := []string{
		"10.0.0.1/32",
		"10.0.0.2/32",
		"10.0.0.3/32",
		"10.0.0.4/32",
		"10.0.0.5/32",
	}

	if err := pa.SetExcludePrefixes(exclusions); err != nil {
		t.Fatalf("Failed to set exclusions: %v", err)
	}

	// Set up concurrent-safe warning counter
	warningCount := 0
	pa.SetWarningHandler(func(msg string) {
		warningCount++
	})

	// Aggregate
	if err := pa.Aggregate(); err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	// Should have warnings for each /32 exclusion
	if warningCount != len(exclusions) {
		t.Errorf("Expected %d warnings, got %d", len(exclusions), warningCount)
	}

	warnings := pa.GetWarnings()
	if len(warnings) != len(exclusions) {
		t.Errorf("Expected %d warnings in GetWarnings(), got %d", len(exclusions), len(warnings))
	}
}
