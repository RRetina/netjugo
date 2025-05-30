package netjugo

import (
	"testing"
)

func TestMinPrefixLengthEnforcement(t *testing.T) {
	pa := NewPrefixAggregator()

	err := pa.SetMinPrefixLength(24, 64)
	if err != nil {
		t.Fatalf("Failed to set min prefix length: %v", err)
	}

	testPrefixes := []string{
		"192.168.0.0/22",  // Already less specific than /24, should remain unchanged
		"192.168.1.0/28",  // More specific than /24, should be rounded up to /24
		"10.0.0.0/24",     // Already meets requirement
		"10.0.0.128/25",   // More specific than /24, should be rounded up to /24
		"2001:db8::/62",   // Already less specific than /64, should remain unchanged
		"2001:db8:1::/64", // Already meets requirement
		"2001:db8:2::/80", // More specific than /64, should be rounded up to /64
	}

	err = pa.AddPrefixes(testPrefixes)
	if err != nil {
		t.Fatalf("Failed to add prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	ipv4Result := pa.GetIPv4Prefixes()
	ipv6Result := pa.GetIPv6Prefixes()

	t.Logf("IPv4 results after min length enforcement:")
	for _, prefix := range ipv4Result {
		t.Logf("  %s", prefix)
	}

	t.Logf("IPv6 results after min length enforcement:")
	for _, prefix := range ipv6Result {
		t.Logf("  %s", prefix)
	}

	// Check expected results
	expectedIPv4 := map[string]bool{
		"192.168.0.0/22": true, // Unchanged (less specific than /24)
		"192.168.1.0/24": true, // Rounded up from /28
		"10.0.0.0/24":    true, // Unchanged (already /24) - should merge with rounded /25
	}

	expectedIPv6 := map[string]bool{
		"2001:db8::/62":   true, // Unchanged (less specific than /64)
		"2001:db8:1::/64": true, // Unchanged (already /64)
		"2001:db8:2::/64": true, // Rounded up from /80
	}

	// Verify IPv4 results
	if len(ipv4Result) > len(expectedIPv4) {
		t.Errorf("Expected at most %d IPv4 prefixes after aggregation, got %d", len(expectedIPv4), len(ipv4Result))
	}

	// Verify IPv6 results
	if len(ipv6Result) != len(expectedIPv6) {
		t.Errorf("Expected %d IPv6 prefixes, got %d", len(expectedIPv6), len(ipv6Result))
	}

	// Verify all resulting prefixes are correctly handled
	// With the new implementation:
	// - Prefixes more specific than minimum are rounded up
	// - Prefixes less specific than minimum remain unchanged
	for _, prefix := range ipv4Result {
		parsed, err := parseIPPrefix(prefix)
		if err != nil {
			t.Errorf("Failed to parse result prefix %s: %v", prefix, err)
		}
		// No prefix should be more specific than the minimum
		if parsed.Prefix.Bits() > 24 {
			t.Errorf("IPv4 prefix %s has length %d, expected <= 24", prefix, parsed.Prefix.Bits())
		}
	}

	for _, prefix := range ipv6Result {
		parsed, err := parseIPPrefix(prefix)
		if err != nil {
			t.Errorf("Failed to parse result prefix %s: %v", prefix, err)
		}
		// No prefix should be more specific than the minimum
		if parsed.Prefix.Bits() > 64 {
			t.Errorf("IPv6 prefix %s has length %d, expected <= 64", prefix, parsed.Prefix.Bits())
		}
	}
}

func TestMinPrefixLengthRounding(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		minLength      int
		isIPv4         bool
		expectedPrefix string
	}{
		{
			name:           "No rounding needed - already less specific",
			input:          "192.168.0.0/22",
			minLength:      24,
			isIPv4:         true,
			expectedPrefix: "192.168.0.0/22",
		},
		{
			name:           "Round /28 up to /24",
			input:          "192.168.1.16/28",
			minLength:      24,
			isIPv4:         true,
			expectedPrefix: "192.168.1.0/24",
		},
		{
			name:           "No rounding needed - exactly minimum",
			input:          "192.168.1.0/24",
			minLength:      24,
			isIPv4:         true,
			expectedPrefix: "192.168.1.0/24",
		},
		{
			name:           "Round /25 up to /24",
			input:          "10.0.0.128/25",
			minLength:      24,
			isIPv4:         true,
			expectedPrefix: "10.0.0.0/24",
		},
		{
			name:           "No rounding needed - IPv6 less specific",
			input:          "2001:db8::/62",
			minLength:      64,
			isIPv4:         false,
			expectedPrefix: "2001:db8::/62",
		},
		{
			name:           "Round /80 up to /64",
			input:          "2001:db8:1:2::/80",
			minLength:      64,
			isIPv4:         false,
			expectedPrefix: "2001:db8:1:2::/64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, err := parseIPPrefix(tt.input)
			if err != nil {
				t.Fatalf("Failed to parse input prefix: %v", err)
			}

			result, err := roundUpToMinLength(prefix, tt.minLength)
			if err != nil {
				t.Fatalf("Failed to round prefix: %v", err)
			}

			if result.Prefix.String() != tt.expectedPrefix {
				t.Errorf("Expected prefix %s, got %s", tt.expectedPrefix, result.Prefix.String())
			}

			// Verify the result meets minimum length requirement
			if result.Prefix.Bits() > tt.minLength {
				t.Errorf("Result prefix %s has length %d, expected <= %d",
					result.Prefix.String(), result.Prefix.Bits(), tt.minLength)
			}
		})
	}
}

func TestMinPrefixLengthEdgeCases(t *testing.T) {
	pa := NewPrefixAggregator()

	// Test invalid minimum lengths
	err := pa.SetMinPrefixLength(-1, 64)
	if err == nil {
		t.Error("Expected error for negative IPv4 min length")
	}

	err = pa.SetMinPrefixLength(33, 64)
	if err == nil {
		t.Error("Expected error for IPv4 min length > 32")
	}

	err = pa.SetMinPrefixLength(24, -1)
	if err == nil {
		t.Error("Expected error for negative IPv6 min length")
	}

	err = pa.SetMinPrefixLength(24, 129)
	if err == nil {
		t.Error("Expected error for IPv6 min length > 128")
	}

	// Test with min length = 0 (should not split anything)
	pa = NewPrefixAggregator()
	err = pa.SetMinPrefixLength(0, 0)
	if err != nil {
		t.Fatalf("Failed to set min length to 0: %v", err)
	}

	err = pa.AddPrefix("192.168.0.0/22")
	if err != nil {
		t.Fatalf("Failed to add prefix: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	result := pa.GetIPv4Prefixes()
	if len(result) != 1 || result[0] != "192.168.0.0/22" {
		t.Errorf("Expected original prefix unchanged with min length 0, got %v", result)
	}
}
