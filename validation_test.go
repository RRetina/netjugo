package netjugo

import (
	"net/netip"
	"testing"
)

func TestParseIPPrefix(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "valid IPv4 prefix",
			input:     "192.168.1.0/24",
			wantError: false,
		},
		{
			name:      "valid IPv6 prefix",
			input:     "2001:db8::/32",
			wantError: false,
		},
		{
			name:      "valid IPv4 host /32",
			input:     "10.0.0.1/32",
			wantError: false,
		},
		{
			name:      "valid IPv6 host /128",
			input:     "2001:db8::1/128",
			wantError: false,
		},
		{
			name:      "bare IPv4 address - auto converts to /32",
			input:     "192.168.1.0",
			wantError: false,
		},
		{
			name:      "invalid prefix - empty string",
			input:     "",
			wantError: true,
		},
		{
			name:      "invalid prefix - whitespace only",
			input:     "   ",
			wantError: true,
		},
		{
			name:      "invalid IPv4 address",
			input:     "256.256.256.256/24",
			wantError: true,
		},
		{
			name:      "invalid IPv4 prefix length",
			input:     "192.168.1.0/33",
			wantError: true,
		},
		{
			name:      "invalid IPv6 prefix length",
			input:     "2001:db8::/129",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseIPPrefix(tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("parseIPPrefix(%q) expected error, got nil", tt.input)
				}
				if result != nil {
					t.Errorf("parseIPPrefix(%q) expected nil result when error, got %+v", tt.input, result)
				}
			} else {
				if err != nil {
					t.Errorf("parseIPPrefix(%q) unexpected error: %v", tt.input, err)
				}
				if result == nil {
					t.Errorf("parseIPPrefix(%q) expected result, got nil", tt.input)
				}
				if result != nil {
					if result.Min == nil || result.Max == nil {
						t.Errorf("parseIPPrefix(%q) result has nil Min or Max", tt.input)
					}
					if result.Min.Cmp(result.Max) > 0 {
						t.Errorf("parseIPPrefix(%q) Min > Max: %v > %v", tt.input, result.Min, result.Max)
					}
				}
			}
		})
	}
}

func TestValidatePrefixLength(t *testing.T) {
	tests := []struct {
		name      string
		isIPv4    bool
		bits      int
		wantError bool
	}{
		{"IPv4 valid 0", true, 0, false},
		{"IPv4 valid 24", true, 24, false},
		{"IPv4 valid 32", true, 32, false},
		{"IPv4 invalid negative", true, -1, true},
		{"IPv4 invalid too large", true, 33, true},
		{"IPv6 valid 0", false, 0, false},
		{"IPv6 valid 64", false, 64, false},
		{"IPv6 valid 128", false, 128, false},
		{"IPv6 invalid negative", false, -1, true},
		{"IPv6 invalid too large", false, 129, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePrefixLength(tt.isIPv4, tt.bits)

			if tt.wantError && err == nil {
				t.Errorf("validatePrefixLength(%v, %d) expected error, got nil", tt.isIPv4, tt.bits)
			}
			if !tt.wantError && err != nil {
				t.Errorf("validatePrefixLength(%v, %d) unexpected error: %v", tt.isIPv4, tt.bits, err)
			}
		})
	}
}

func TestIsValidIPPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid IPv4", "192.168.1.0/24", true},
		{"valid IPv6", "2001:db8::/32", true},
		{"invalid empty", "", false},
		{"invalid format", "not-an-ip", false},
		{"invalid prefix length", "192.168.1.0/33", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidIPPrefix(tt.input)
			if result != tt.expected {
				t.Errorf("isValidIPPrefix(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBareIPAddressParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		// IPv4 bare addresses
		{
			name:     "IPv4 bare address",
			input:    "192.168.1.1",
			expected: "192.168.1.1/32",
			wantErr:  false,
		},
		{
			name:     "IPv4 with /32",
			input:    "192.168.1.1/32",
			expected: "192.168.1.1/32",
			wantErr:  false,
		},
		{
			name:     "IPv4 network prefix",
			input:    "192.168.1.0/24",
			expected: "192.168.1.0/24",
			wantErr:  false,
		},
		// IPv6 bare addresses
		{
			name:     "IPv6 bare address",
			input:    "2001:db8::1",
			expected: "2001:db8::1/128",
			wantErr:  false,
		},
		{
			name:     "IPv6 with /128",
			input:    "2001:db8::1/128",
			expected: "2001:db8::1/128",
			wantErr:  false,
		},
		{
			name:     "IPv6 network prefix",
			input:    "2001:db8::/32",
			expected: "2001:db8::/32",
			wantErr:  false,
		},
		{
			name:     "IPv6 full form bare address",
			input:    "2001:0db8:0000:0000:0000:0000:0000:0001",
			expected: "2001:db8::1/128",
			wantErr:  false,
		},
		// Error cases
		{
			name:     "Invalid input",
			input:    "not-an-ip",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Empty input",
			input:    "",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "IPv4 with invalid prefix",
			input:    "192.168.1.1/33",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "IPv6 with invalid prefix",
			input:    "2001:db8::/129",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, err := parseIPPrefix(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseIPPrefix(%q) expected error, but got none", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("parseIPPrefix(%q) unexpected error: %v", tt.input, err)
				return
			}

			if prefix.Prefix.String() != tt.expected {
				t.Errorf("parseIPPrefix(%q) = %q, want %q", tt.input, prefix.Prefix.String(), tt.expected)
			}

			// Release the prefix back to the pool
			releaseIPPrefix(prefix)
		})
	}
}

func TestBareIPAddressInAggregation(t *testing.T) {
	pa := NewPrefixAggregator()

	// Mix of bare IPs and proper CIDR notation
	prefixes := []string{
		"192.168.1.1",    // bare IPv4
		"192.168.1.2",    // bare IPv4
		"192.168.1.3",    // bare IPv4
		"192.168.1.4/32", // IPv4 with /32
		"192.168.2.0/24", // IPv4 network
		"2001:db8::1",    // bare IPv6
		"2001:db8::2",    // bare IPv6
		"2001:db8::/64",  // IPv6 network
	}

	err := pa.AddPrefixes(prefixes)
	if err != nil {
		t.Fatalf("Failed to add prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	results := pa.GetPrefixes()
	stats := pa.GetStats()

	t.Logf("Original: %d prefixes, Aggregated: %d prefixes", stats.OriginalCount, len(results))
	t.Logf("Results: %v", results)

	// Check that bare IPs were properly parsed and included
	// The exact aggregation result depends on the algorithm
	if len(results) == 0 {
		t.Errorf("Expected aggregated results, got none")
	}

	// Verify IPv4 and IPv6 prefixes are present
	hasIPv4 := false
	hasIPv6 := false
	for _, result := range results {
		if prefix, _ := netip.ParsePrefix(result); prefix.IsValid() {
			if prefix.Addr().Is4() {
				hasIPv4 = true
			} else if prefix.Addr().Is6() {
				hasIPv6 = true
			}
		}
	}

	if !hasIPv4 {
		t.Errorf("Expected IPv4 prefixes in results")
	}
	if !hasIPv6 {
		t.Errorf("Expected IPv6 prefixes in results")
	}
}

func TestBareIPAddressExclusion(t *testing.T) {
	pa := NewPrefixAggregator()

	// Add a network
	err := pa.AddPrefix("192.168.1.0/24")
	if err != nil {
		t.Fatalf("Failed to add prefix: %v", err)
	}

	// Exclude a single IP (bare)
	err = pa.SetExcludePrefixes([]string{"192.168.1.100"})
	if err != nil {
		t.Fatalf("Failed to set exclude prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	results := pa.GetPrefixes()

	// Should have multiple prefixes, excluding the single IP
	if len(results) < 2 {
		t.Errorf("Expected multiple prefixes after excluding single IP, got %d", len(results))
	}

	// The excluded IP should not be in any of the results
	excludedAddr, _ := netip.ParseAddr("192.168.1.100")
	for _, result := range results {
		prefix, _ := netip.ParsePrefix(result)
		if prefix.Contains(excludedAddr) {
			t.Errorf("Excluded IP 192.168.1.100 found within prefix %s", result)
		}
	}

	t.Logf("Prefixes after excluding 192.168.1.100: %v", results)
}

func TestMinPrefixLengthEnforcement(t *testing.T) {
	// Test the exact example from CLAUDE.md
	agg := NewPrefixAggregator()
	if err := agg.SetMinPrefixLength(21, 0); err != nil {
		t.Fatalf("Failed to set minimum prefix length: %v", err)
	}

	// Add the test prefixes
	testPrefixes := []string{
		"1.0.0.0/24",
		"1.0.1.0/24",
		"1.0.2.0/23",
		"1.0.4.0/24",
		"1.0.5.0/25",
		"1.0.5.128/26",
		"1.0.5.192/28",
		"1.0.5.208/30",
		"1.0.5.212",
		"1.0.5.213",
		"1.0.5.214/31",
		"1.0.5.216/29",
		"1.0.5.224/27",
		"1.0.6.0/23",
	}

	for _, prefix := range testPrefixes {
		if err := agg.AddPrefix(prefix); err != nil {
			t.Fatalf("Failed to add prefix %s: %v", prefix, err)
		}
	}

	// Aggregate
	if err := agg.Aggregate(); err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	// Get results
	results := agg.GetPrefixes()

	// Should have exactly one prefix: 1.0.0.0/21
	if len(results) != 1 {
		t.Errorf("Expected 1 prefix, got %d: %v", len(results), results)
	}

	if len(results) > 0 && results[0] != "1.0.0.0/21" {
		t.Errorf("Expected 1.0.0.0/21, got %s", results[0])
	}
}

func TestMinPrefixLengthWithReaggregation(t *testing.T) {
	// First aggregation without min prefix
	agg1 := NewPrefixAggregator()

	testPrefixes := []string{
		"1.0.0.0/24",
		"1.0.1.0/24",
		"1.0.2.0/23",
		"1.0.4.0/24",
		"1.0.5.0/24",
	}

	for _, prefix := range testPrefixes {
		if err := agg1.AddPrefix(prefix); err != nil {
			t.Fatalf("Failed to add prefix %s: %v", prefix, err)
		}
	}

	if err := agg1.Aggregate(); err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	firstResults := agg1.GetPrefixes()

	// Second aggregation with min prefix 21
	agg2 := NewPrefixAggregator()
	if err := agg2.SetMinPrefixLength(21, 0); err != nil {
		t.Fatalf("Failed to set minimum prefix length: %v", err)
	}

	// Add the results from first aggregation
	for _, prefix := range firstResults {
		if err := agg2.AddPrefix(prefix); err != nil {
			t.Fatalf("Failed to add prefix %s: %v", prefix, err)
		}
	}

	if err := agg2.Aggregate(); err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	finalResults := agg2.GetPrefixes()

	// Should result in 1.0.0.0/21
	if len(finalResults) != 1 {
		t.Errorf("Expected 1 prefix after re-aggregation, got %d: %v", len(finalResults), finalResults)
	}

	if len(finalResults) > 0 && finalResults[0] != "1.0.0.0/21" {
		t.Errorf("Expected 1.0.0.0/21, got %s", finalResults[0])
	}
}
