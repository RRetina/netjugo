package netjugo

import (
	"strings"
	"testing"
)

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
