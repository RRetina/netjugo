package netjugo

import (
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
