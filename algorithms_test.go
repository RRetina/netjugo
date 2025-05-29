package netjugo

import (
	"testing"
)

func TestBasicAggregation(t *testing.T) {
	pa := NewPrefixAggregator()

	testPrefixes := []string{
		"192.168.1.0/24",
		"192.168.2.0/24",
		"10.0.0.0/24",
		"10.0.1.0/24",
	}

	err := pa.AddPrefixes(testPrefixes)
	if err != nil {
		t.Fatalf("Failed to add prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	result := pa.GetIPv4Prefixes()
	if len(result) == 0 {
		t.Fatal("Expected aggregated prefixes, got none")
	}

	t.Logf("Original prefixes: %d, Aggregated: %d", len(testPrefixes), len(result))
	for _, prefix := range result {
		t.Logf("Aggregated prefix: %s", prefix)
	}
}

func TestDuplicateRemoval(t *testing.T) {
	pa := NewPrefixAggregator()

	testPrefixes := []string{
		"192.168.1.0/24",
		"192.168.1.0/24", // duplicate
		"10.0.0.0/24",
		"10.0.0.0/24", // duplicate
	}

	err := pa.AddPrefixes(testPrefixes)
	if err != nil {
		t.Fatalf("Failed to add prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	result := pa.GetIPv4Prefixes()
	if len(result) != 2 {
		t.Errorf("Expected 2 unique prefixes after deduplication, got %d", len(result))
	}
}

func TestContainmentAggregation(t *testing.T) {
	pa := NewPrefixAggregator()

	testPrefixes := []string{
		"192.168.0.0/16", // contains the others
		"192.168.1.0/24",
		"192.168.2.0/24",
	}

	err := pa.AddPrefixes(testPrefixes)
	if err != nil {
		t.Fatalf("Failed to add prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	result := pa.GetIPv4Prefixes()
	if len(result) != 1 {
		t.Errorf("Expected 1 prefix after containment aggregation, got %d", len(result))
	}

	if len(result) > 0 && result[0] != "192.168.0.0/16" {
		t.Errorf("Expected 192.168.0.0/16, got %s", result[0])
	}
}

func TestIPv6Aggregation(t *testing.T) {
	pa := NewPrefixAggregator()

	testPrefixes := []string{
		"2001:db8::/64",
		"2001:db8:1::/64",
		"2001:0db8:0002::/64",
	}

	err := pa.AddPrefixes(testPrefixes)
	if err != nil {
		t.Fatalf("Failed to add prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	result := pa.GetIPv6Prefixes()
	if len(result) == 0 {
		t.Fatal("Expected IPv6 prefixes, got none")
	}

	t.Logf("IPv6 aggregation result: %v", result)
}

func TestMixedIPVersions(t *testing.T) {
	pa := NewPrefixAggregator()

	testPrefixes := []string{
		"192.168.1.0/24",
		"192.168.2.0/24",
		"2001:db8::/64",
		"2001:db8:1::/64",
	}

	err := pa.AddPrefixes(testPrefixes)
	if err != nil {
		t.Fatalf("Failed to add prefixes: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate: %v", err)
	}

	ipv4Result := pa.GetIPv4Prefixes()
	ipv6Result := pa.GetIPv6Prefixes()

	if len(ipv4Result) == 0 {
		t.Error("Expected IPv4 prefixes, got none")
	}
	if len(ipv6Result) == 0 {
		t.Error("Expected IPv6 prefixes, got none")
	}

	stats := pa.GetStats()
	if stats.IPv4PrefixCount != len(ipv4Result) {
		t.Errorf("Stats IPv4 count mismatch: %d vs %d", stats.IPv4PrefixCount, len(ipv4Result))
	}
	if stats.IPv6PrefixCount != len(ipv6Result) {
		t.Errorf("Stats IPv6 count mismatch: %d vs %d", stats.IPv6PrefixCount, len(ipv6Result))
	}
}

func TestEmptyInput(t *testing.T) {
	pa := NewPrefixAggregator()

	err := pa.Aggregate()
	if err != nil {
		t.Fatalf("Aggregation of empty input failed: %v", err)
	}

	result := pa.GetPrefixes()
	if len(result) != 0 {
		t.Errorf("Expected no prefixes for empty input, got %d", len(result))
	}

	stats := pa.GetStats()
	if stats.TotalPrefixes != 0 {
		t.Errorf("Expected 0 total prefixes, got %d", stats.TotalPrefixes)
	}
}

func TestSinglePrefix(t *testing.T) {
	pa := NewPrefixAggregator()

	err := pa.AddPrefix("192.168.1.0/24")
	if err != nil {
		t.Fatalf("Failed to add single prefix: %v", err)
	}

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Failed to aggregate single prefix: %v", err)
	}

	result := pa.GetIPv4Prefixes()
	if len(result) != 1 {
		t.Errorf("Expected 1 prefix, got %d", len(result))
	}

	if len(result) > 0 && result[0] != "192.168.1.0/24" {
		t.Errorf("Expected 192.168.1.0/24, got %s", result[0])
	}
}
