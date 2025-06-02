package netjugo

import (
	"net/netip"
	"testing"
)

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
