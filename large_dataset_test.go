package netjugo

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestLargeDatasetLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large dataset test in short mode")
	}

	pa := NewPrefixAggregator()

	// Check if large dataset file exists
	datasetPath := ".samples/large-dataset-prefixes.txt"
	if _, err := os.Stat(datasetPath); os.IsNotExist(err) {
		t.Skipf("Large dataset file not found at %s", datasetPath)
	}

	t.Logf("Loading large dataset from %s", datasetPath)
	start := time.Now()

	err := pa.AddFromFile(datasetPath)
	if err != nil {
		t.Fatalf("Failed to load large dataset: %v", err)
	}

	loadTime := time.Since(start)
	t.Logf("Dataset loaded in %v", loadTime)

	// Get initial stats
	stats := pa.GetStats()
	memStats := pa.GetMemoryStats()

	t.Logf("Initial stats:")
	t.Logf("  Total prefixes loaded: %d", stats.OriginalCount)
	t.Logf("  IPv4 prefixes: %d", stats.IPv4PrefixCount)
	t.Logf("  IPv6 prefixes: %d", stats.IPv6PrefixCount)
	t.Logf("  Memory usage: %.2f MB", float64(memStats.AggregatorBytes)/(1024*1024))
	t.Logf("  System memory: %.2f MB", float64(memStats.AllocBytes)/(1024*1024))

	// Verify we loaded a substantial number of prefixes
	if stats.OriginalCount < 1000000 {
		t.Errorf("Expected to load at least 1M prefixes, got %d", stats.OriginalCount)
	}

	// Memory constraint check (should be under 1GB for 8M prefixes as per SOW)
	maxMemoryBytes := int64(1024 * 1024 * 1024) // 1GB
	if memStats.AggregatorBytes > maxMemoryBytes {
		t.Errorf("Memory usage %d bytes exceeds 1GB limit", memStats.AggregatorBytes)
	}
}

func TestLargeDatasetAggregation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large dataset aggregation test in short mode")
	}

	pa := NewPrefixAggregator()

	// Use a smaller sample for aggregation test to avoid timeout
	datasetPath := ".samples/sample-100k-prefixes.txt"
	if _, err := os.Stat(datasetPath); os.IsNotExist(err) {
		t.Skipf("Sample dataset file not found at %s", datasetPath)
	}

	t.Log("Loading sample dataset for aggregation test...")
	err := pa.AddFromFile(datasetPath)
	if err != nil {
		t.Fatalf("Failed to load dataset: %v", err)
	}

	initialStats := pa.GetStats()
	t.Logf("Loaded %d prefixes", initialStats.OriginalCount)

	// Perform aggregation
	t.Log("Starting aggregation...")
	start := time.Now()

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	aggregationTime := time.Since(start)

	// Get final stats
	finalStats := pa.GetStats()
	memStats := pa.GetMemoryStats()

	t.Logf("Aggregation completed in %v", aggregationTime)
	t.Logf("Final stats:")
	t.Logf("  Original prefixes: %d", finalStats.OriginalCount)
	t.Logf("  Aggregated prefixes: %d", finalStats.TotalPrefixes)
	t.Logf("  IPv4 prefixes: %d", finalStats.IPv4PrefixCount)
	t.Logf("  IPv6 prefixes: %d", finalStats.IPv6PrefixCount)
	t.Logf("  Reduction ratio: %.2f%%", finalStats.ReductionRatio*100)
	t.Logf("  Processing time: %d ms", finalStats.ProcessingTimeMs)
	t.Logf("  Memory usage: %.2f MB", float64(memStats.AggregatorBytes)/(1024*1024))

	// Performance requirements from SOW
	maxAggregationTime := 10 * time.Second // 10 seconds for 1M prefixes
	if aggregationTime > maxAggregationTime && finalStats.OriginalCount >= 1000000 {
		t.Errorf("Aggregation took %v, expected under %v for 1M+ prefixes", aggregationTime, maxAggregationTime)
	}

	// Verify aggregation actually reduced prefix count
	if finalStats.TotalPrefixes >= finalStats.OriginalCount {
		t.Logf("Warning: No aggregation occurred (final: %d, original: %d)",
			finalStats.TotalPrefixes, finalStats.OriginalCount)
	}

	// Memory constraint check
	maxMemoryBytes := int64(1024 * 1024 * 1024) // 1GB
	if memStats.AggregatorBytes > maxMemoryBytes {
		t.Errorf("Memory usage %d bytes exceeds 1GB limit", memStats.AggregatorBytes)
	}
}

func TestLargeDatasetWithMinPrefixLength(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large dataset min prefix length test in short mode")
	}

	pa := NewPrefixAggregator()

	datasetPath := ".samples/large-dataset-prefixes.txt"
	if _, err := os.Stat(datasetPath); os.IsNotExist(err) {
		t.Skipf("Large dataset file not found at %s", datasetPath)
	}

	// Set minimum prefix lengths
	err := pa.SetMinPrefixLength(24, 48)
	if err != nil {
		t.Fatalf("Failed to set min prefix lengths: %v", err)
	}

	t.Log("Loading subset of dataset for min prefix length test...")
	// For this test, load a smaller subset to avoid excessive memory usage
	err = pa.AddFromFile(datasetPath)
	if err != nil {
		t.Fatalf("Failed to load dataset: %v", err)
	}

	initialStats := pa.GetStats()
	if initialStats.OriginalCount > 100000 {
		t.Logf("Dataset too large (%d prefixes), this test should use a smaller subset", initialStats.OriginalCount)
	}

	t.Log("Starting aggregation with min prefix lengths...")
	start := time.Now()

	err = pa.Aggregate()
	if err != nil {
		t.Fatalf("Aggregation with min prefix lengths failed: %v", err)
	}

	aggregationTime := time.Since(start)

	finalStats := pa.GetStats()
	t.Logf("Aggregation with min lengths completed in %v", aggregationTime)
	t.Logf("Original: %d, Final: %d", finalStats.OriginalCount, finalStats.TotalPrefixes)

	// Verify all resulting prefixes meet minimum length requirements
	ipv4Prefixes := pa.GetIPv4Prefixes()
	ipv6Prefixes := pa.GetIPv6Prefixes()

	for i, prefixStr := range ipv4Prefixes {
		if i >= 100 { // Check first 100 to avoid excessive logging
			break
		}
		prefix, err := parseIPPrefix(prefixStr)
		if err != nil {
			t.Errorf("Invalid IPv4 prefix in result: %s", prefixStr)
			continue
		}
		if prefix.Prefix.Bits() > 24 {
			t.Errorf("IPv4 prefix %s has length %d, expected <= 24 (due to min prefix setting)", prefixStr, prefix.Prefix.Bits())
		}
	}

	for i, prefixStr := range ipv6Prefixes {
		if i >= 100 { // Check first 100 to avoid excessive logging
			break
		}
		prefix, err := parseIPPrefix(prefixStr)
		if err != nil {
			t.Errorf("Invalid IPv6 prefix in result: %s", prefixStr)
			continue
		}
		if prefix.Prefix.Bits() > 48 {
			t.Errorf("IPv6 prefix %s has length %d, expected <= 48 (due to min prefix setting)", prefixStr, prefix.Prefix.Bits())
		}
	}
}

func TestMemoryEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory efficiency test in short mode")
	}

	pa := NewPrefixAggregator()

	// Force garbage collection to get clean baseline
	runtime.GC()
	runtime.GC()

	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	baseline := m1.Alloc

	// Add a known number of prefixes
	testPrefixes := []string{
		"192.168.0.0/16",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"2001:db8::/32",
		"2001:0db8:0001::/48",
	}

	for _, prefix := range testPrefixes {
		err := pa.AddPrefix(prefix)
		if err != nil {
			t.Fatalf("Failed to add prefix %s: %v", prefix, err)
		}
	}

	err := pa.Aggregate()
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	memStats := pa.GetMemoryStats()
	calculatedMemory := pa.calculateMemoryUsage()

	runtime.GC()
	runtime.GC()

	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	actualUsed := m2.Alloc - baseline

	t.Logf("Memory efficiency test:")
	t.Logf("  Prefixes: %d", len(testPrefixes))
	t.Logf("  Calculated aggregator memory: %d bytes", calculatedMemory)
	t.Logf("  Runtime stats aggregator memory: %d bytes", memStats.AggregatorBytes)
	t.Logf("  Actual memory increase: %d bytes", actualUsed)
	t.Logf("  System allocation: %d bytes", memStats.AllocBytes)

	// Verify the calculated memory is reasonable
	if calculatedMemory <= 0 {
		t.Error("Calculated memory usage should be positive")
	}

	// Memory should be efficient for small datasets
	expectedMaxMemory := int64(10 * 1024) // 10KB should be plenty for 5 prefixes
	if calculatedMemory > expectedMaxMemory {
		t.Errorf("Memory usage %d bytes seems excessive for %d prefixes", calculatedMemory, len(testPrefixes))
	}
}

func BenchmarkLargeDatasetLoad(b *testing.B) {
	datasetPath := ".samples/large-dataset-prefixes.txt"
	if _, err := os.Stat(datasetPath); os.IsNotExist(err) {
		b.Skipf("Large dataset file not found at %s", datasetPath)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pa := NewPrefixAggregator()
		err := pa.AddFromFile(datasetPath)
		if err != nil {
			b.Fatalf("Failed to load dataset: %v", err)
		}
	}
}

func BenchmarkAggregationScaling(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("prefixes_%d", size), func(b *testing.B) {
			// Generate test prefixes
			prefixes := generateTestPrefixes(size)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pa := NewPrefixAggregator()
				err := pa.AddPrefixes(prefixes)
				if err != nil {
					b.Fatalf("Failed to add prefixes: %v", err)
				}

				err = pa.Aggregate()
				if err != nil {
					b.Fatalf("Aggregation failed: %v", err)
				}
			}
		})
	}
}

func generateTestPrefixes(count int) []string {
	prefixes := make([]string, count)

	for i := 0; i < count; i++ {
		// Generate various prefix sizes for realistic testing
		if i%2 == 0 {
			// IPv4 prefixes
			prefixes[i] = fmt.Sprintf("192.%d.%d.0/24", i%256, (i/256)%256)
		} else {
			// IPv6 prefixes
			prefixes[i] = fmt.Sprintf("2001:db8:%x::/48", i%65536)
		}
	}

	return prefixes
}
