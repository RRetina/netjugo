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
		"192.168.0.0/22",  // Should be split to /24s (more reasonable size)
		"10.0.0.0/24",     // Already meets requirement
		"2001:db8::/62",   // Should be split to /64s (more reasonable size)
		"2001:db8:1::/64", // Already meets requirement
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

	// The /16 should have been split into multiple /24s
	if len(ipv4Result) < 2 {
		t.Errorf("Expected multiple IPv4 prefixes after splitting /16 to /24s, got %d", len(ipv4Result))
	}

	// The /32 should have been split into multiple /64s
	if len(ipv6Result) < 2 {
		t.Errorf("Expected multiple IPv6 prefixes after splitting /32 to /64s, got %d", len(ipv6Result))
	}

	// Verify all resulting prefixes meet minimum length requirements
	for _, prefix := range ipv4Result {
		parsed, err := parseIPPrefix(prefix)
		if err != nil {
			t.Errorf("Failed to parse result prefix %s: %v", prefix, err)
		}
		if parsed.Prefix.Bits() < 24 {
			t.Errorf("IPv4 prefix %s has length %d, expected >= 24", prefix, parsed.Prefix.Bits())
		}
	}

	for _, prefix := range ipv6Result {
		parsed, err := parseIPPrefix(prefix)
		if err != nil {
			t.Errorf("Failed to parse result prefix %s: %v", prefix, err)
		}
		if parsed.Prefix.Bits() < 64 {
			t.Errorf("IPv6 prefix %s has length %d, expected >= 64", prefix, parsed.Prefix.Bits())
		}
	}
}

func TestMinPrefixLengthSplitting(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		minLength     int
		isIPv4        bool
		expectedCount int
	}{
		{
			name:          "Split /22 to /24",
			input:         "192.168.0.0/22",
			minLength:     24,
			isIPv4:        true,
			expectedCount: 4, // 2^(24-22) = 4
		},
		{
			name:          "Split /22 to /24",
			input:         "10.0.0.0/22",
			minLength:     24,
			isIPv4:        true,
			expectedCount: 4, // 2^(24-22) = 4
		},
		{
			name:          "No split needed",
			input:         "192.168.1.0/24",
			minLength:     24,
			isIPv4:        true,
			expectedCount: 1,
		},
		{
			name:          "Split /62 to /64",
			input:         "2001:db8::/62",
			minLength:     64,
			isIPv4:        false,
			expectedCount: 4, // 2^(64-62) = 4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, err := parseIPPrefix(tt.input)
			if err != nil {
				t.Fatalf("Failed to parse input prefix: %v", err)
			}

			result, err := splitPrefixToMinLength(prefix, tt.minLength, tt.isIPv4)
			if err != nil {
				t.Fatalf("Failed to split prefix: %v", err)
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d prefixes, got %d", tt.expectedCount, len(result))
			}

			// Verify all results meet minimum length
			for i, resultPrefix := range result {
				if resultPrefix.Prefix.Bits() < tt.minLength {
					t.Errorf("Result prefix %d (%s) has length %d, expected >= %d",
						i, resultPrefix.Prefix.String(), resultPrefix.Prefix.Bits(), tt.minLength)
				}
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
