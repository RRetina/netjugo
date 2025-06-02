package netjugo

import (
	"testing"
)

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
