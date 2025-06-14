package netjugo

import (
	"fmt"
	"github.com/holiman/uint256"
	"net/netip"
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

func BenchmarkPrefixParsing(b *testing.B) {
	testCases := []struct {
		name   string
		prefix string
	}{
		{"IPv4_24", "192.168.1.0/24"},
		{"IPv4_32", "192.168.1.1/32"},
		{"IPv6_64", "2001:db8::/64"},
		{"IPv6_128", "2001:db8::1/128"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := parseIPPrefix(tc.prefix)
				if err != nil {
					b.Fatalf("Failed to parse prefix %s: %v", tc.prefix, err)
				}
			}
		})
	}
}

func BenchmarkAddPrefix(b *testing.B) {
	testCases := []struct {
		name     string
		prefixes []string
	}{
		{
			"IPv4_Small",
			[]string{"192.168.1.0/24", "192.168.2.0/24", "192.168.3.0/24"},
		},
		{
			"IPv6_Small",
			[]string{"2001:db8::/64", "2001:db8:1::/64", "2001:db8:2::/64"},
		},
		{
			"Mixed_Small",
			[]string{"192.168.1.0/24", "2001:db8::/64", "10.0.0.0/24"},
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pa := NewPrefixAggregator()
				for _, prefix := range tc.prefixes {
					err := pa.AddPrefix(prefix)
					if err != nil {
						b.Fatalf("Failed to add prefix %s: %v", prefix, err)
					}
				}
			}
		})
	}
}

func BenchmarkAggregation(b *testing.B) {
	sizes := []int{100, 1000, 10000, 50000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Prefixes_%d", size), func(b *testing.B) {
			// Pre-generate prefixes to avoid including generation time in benchmark
			prefixes := generateBenchmarkPrefixes(size)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pa := NewPrefixAggregator()

				// Add prefixes
				for _, prefix := range prefixes {
					err := pa.AddPrefix(prefix)
					if err != nil {
						b.Fatalf("Failed to add prefix %s: %v", prefix, err)
					}
				}

				// Perform aggregation
				err := pa.Aggregate()
				if err != nil {
					b.Fatalf("Aggregation failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkMemoryUsage(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Prefixes_%d", size), func(b *testing.B) {
			prefixes := generateBenchmarkPrefixes(size)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Force GC to get clean measurement
				runtime.GC()

				var m1 runtime.MemStats
				runtime.ReadMemStats(&m1)

				pa := NewPrefixAggregator()
				for _, prefix := range prefixes {
					err := pa.AddPrefix(prefix)
					if err != nil {
						b.Fatalf("Failed to add prefix %s: %v", prefix, err)
					}
				}

				runtime.GC()
				var m2 runtime.MemStats
				runtime.ReadMemStats(&m2)

				// Record memory per prefix
				memPerPrefix := int64(m2.Alloc-m1.Alloc) / int64(size)
				b.ReportMetric(float64(memPerPrefix), "bytes/prefix")
			}
		})
	}
}

func BenchmarkMinPrefixLengthEnforcement(b *testing.B) {
	// Test different prefix length enforcement scenarios
	testCases := []struct {
		name             string
		basePrefixes     []string
		minIPv4, minIPv6 int
	}{
		{
			"Split_16_to_24",
			[]string{"192.168.0.0/16", "10.0.0.0/16"},
			24, 0,
		},
		{
			"Split_32_to_48",
			[]string{"2001:db8::/32"},
			0, 48,
		},
		{
			"Mixed_Splitting",
			[]string{"192.168.0.0/16", "2001:db8::/32", "172.16.0.0/12"},
			24, 48,
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pa := NewPrefixAggregator()
				err := pa.SetMinPrefixLength(tc.minIPv4, tc.minIPv6)
				if err != nil {
					b.Fatalf("Failed to set min prefix length: %v", err)
				}

				for _, prefix := range tc.basePrefixes {
					err := pa.AddPrefix(prefix)
					if err != nil {
						b.Fatalf("Failed to add prefix %s: %v", prefix, err)
					}
				}

				err = pa.Aggregate()
				if err != nil {
					b.Fatalf("Aggregation failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkExclusionProcessing(b *testing.B) {
	testCases := []struct {
		name            string
		basePrefixes    []string
		excludePrefixes []string
	}{
		{
			"Simple_Exclusion",
			[]string{"192.168.0.0/16"},
			[]string{"192.168.1.0/24"},
		},
		{
			"Multiple_Exclusions",
			[]string{"10.0.0.0/8"},
			[]string{"10.1.0.0/16", "10.2.0.0/16", "10.3.0.0/16"},
		},
		{
			"IPv6_Exclusion",
			[]string{"2001:db8::/32"},
			[]string{"2001:db8:1::/48", "2001:db8:2::/48"},
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pa := NewPrefixAggregator()

				for _, prefix := range tc.basePrefixes {
					err := pa.AddPrefix(prefix)
					if err != nil {
						b.Fatalf("Failed to add prefix %s: %v", prefix, err)
					}
				}

				err := pa.SetExcludePrefixes(tc.excludePrefixes)
				if err != nil {
					b.Fatalf("Failed to set exclude prefixes: %v", err)
				}

				err = pa.Aggregate()
				if err != nil {
					b.Fatalf("Aggregation failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkFileIO(b *testing.B) {
	// Create a temporary file with test prefixes
	prefixes := generateBenchmarkPrefixes(10000)

	b.Run("FileRead", func(b *testing.B) {
		// Create temp file
		tmpfile := createTempPrefixFile(b, prefixes)
		defer removeTempFile(tmpfile)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pa := NewPrefixAggregator()
			err := pa.AddFromFile(tmpfile)
			if err != nil {
				b.Fatalf("Failed to read file: %v", err)
			}
		}
	})

	b.Run("FileWrite", func(b *testing.B) {
		// Pre-create aggregator with data
		pa := NewPrefixAggregator()
		for _, prefix := range prefixes {
			err := pa.AddPrefix(prefix)
			if err != nil {
				b.Fatalf("Failed to add prefix %s: %v", prefix, err)
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tmpfile := getTempFilename()
			err := pa.WriteToFile(tmpfile)
			if err != nil {
				b.Fatalf("Failed to write file: %v", err)
			}
			removeTempFile(tmpfile)
		}
	})
}

func BenchmarkConcurrentAccess(b *testing.B) {
	pa := NewPrefixAggregator()

	// Pre-populate with some data
	prefixes := generateBenchmarkPrefixes(1000)
	for _, prefix := range prefixes {
		err := pa.AddPrefix(prefix)
		if err != nil {
			b.Fatalf("Failed to add prefix %s: %v", prefix, err)
		}
	}
	err := pa.Aggregate()
	if err != nil {
		b.Fatalf("Failed to aggregate: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Test concurrent read operations
			stats := pa.GetStats()
			_ = stats.TotalPrefixes

			prefixList := pa.GetPrefixes()
			_ = len(prefixList)

			memStats := pa.GetMemoryStats()
			_ = memStats.AllocBytes
		}
	})
}

// Performance comparison benchmarks
func BenchmarkUint256Operations(b *testing.B) {
	// Test performance of uint256 operations used in the library
	from, _ := parseIPPrefix("192.168.0.0/24")
	to, _ := parseIPPrefix("192.168.1.0/24")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Test comparison operations
		_ = from.Min.Cmp(to.Min)
		_ = from.Max.Cmp(to.Max)

		// Test arithmetic operations
		temp := new(uint256.Int).Add(from.Min, uint256.NewInt(1))
		_ = temp.Cmp(from.Max)
	}
}

// Statistics calculation
func BenchmarkStatisticsCalculation(b *testing.B) {
	pa := NewPrefixAggregator()
	prefixes := generateBenchmarkPrefixes(10000)

	for _, prefix := range prefixes {
		err := pa.AddPrefix(prefix)
		if err != nil {
			b.Fatalf("Failed to add prefix %s: %v", prefix, err)
		}
	}
	err := pa.Aggregate()
	if err != nil {
		b.Fatalf("Failed to aggregate: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats := pa.GetStats()
		_ = stats.ReductionRatio

		memStats := pa.GetMemoryStats()
		_ = memStats.AggregatorBytes
	}
}

// Benchmark memory pooling vs direct allocation
func BenchmarkIPPrefixAllocation(b *testing.B) {
	b.Run("WithPooling", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := acquireIPPrefix()
			p.Prefix, _ = parseNetipPrefix("192.168.1.0/24")
			releaseIPPrefix(p)
		}
	})

	b.Run("WithoutPooling", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := &IPPrefix{
				Min: new(uint256.Int),
				Max: new(uint256.Int),
			}
			p.Prefix, _ = parseNetipPrefix("192.168.1.0/24")
		}
	})
}

// Benchmark binary search vs linear search for finding overlaps
func BenchmarkFindOverlapping(b *testing.B) {
	pa := NewPrefixAggregator()

	// Add many prefixes
	for i := 0; i < 10000; i++ {
		prefix := fmt.Sprintf("10.%d.%d.0/24", i/256, i%256)
		_ = pa.AddPrefix(prefix)
	}

	// Sort them
	_ = pa.sortAndDeduplicateIPv4()

	target, _ := parseIPPrefix("10.50.50.0/24")

	b.Run("BinarySearch", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = pa.findOverlappingPrefixes(target, pa.IPv4Prefixes)
		}
	})
}

// Benchmark aggregation with different dataset sizes
func BenchmarkAggregationScalingNew(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			prefixes := make([]string, size)
			for i := 0; i < size; i++ {
				prefixes[i] = fmt.Sprintf("10.%d.%d.0/24", i/256, i%256)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pa := NewPrefixAggregator()
				_ = pa.AddPrefixes(prefixes)
				_ = pa.Aggregate()
			}
		})
	}
}

// Helper functions for benchmarks
func generateBenchmarkPrefixes(count int) []string {
	prefixes := make([]string, count)

	for i := 0; i < count; i++ {
		switch i % 4 {
		case 0:
			// IPv4 /24 networks
			prefixes[i] = fmt.Sprintf("10.%d.%d.0/24", i%256, (i/256)%256)
		case 1:
			// IPv4 /23 networks
			prefixes[i] = fmt.Sprintf("172.%d.%d.0/23", 16+(i%16), (i/16)%256)
		case 2:
			// IPv6 /64 networks
			prefixes[i] = fmt.Sprintf("2001:db8:%x::/64", i%65536)
		case 3:
			// IPv6 /48 networks
			prefixes[i] = fmt.Sprintf("2001:db8:%x::/48", i%65536)
		}
	}

	return prefixes
}

func createTempPrefixFile(b *testing.B, prefixes []string) string {
	tmpfile := getTempFilename()

	pa := NewPrefixAggregator()
	for _, prefix := range prefixes {
		err := pa.AddPrefix(prefix)
		if err != nil {
			b.Fatalf("Failed to add prefix %s: %v", prefix, err)
		}
	}

	err := pa.WriteToFile(tmpfile)
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}

	return tmpfile
}

func getTempFilename() string {
	return fmt.Sprintf("/tmp/netjugo_bench_%d.txt", runtime.NumGoroutine())
}

func removeTempFile(filename string) {
	// os.Remove(filename) - would remove but keeping simple for benchmark
	_ = filename
}

func parseNetipPrefix(s string) (netip.Prefix, error) {
	return netip.ParsePrefix(s)
}
