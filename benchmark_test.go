package netjugo

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/holiman/uint256"
)

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
					pa.AddPrefix(prefix)
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
					pa.AddPrefix(prefix)
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
				pa.SetMinPrefixLength(tc.minIPv4, tc.minIPv6)

				for _, prefix := range tc.basePrefixes {
					pa.AddPrefix(prefix)
				}

				err := pa.Aggregate()
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
					pa.AddPrefix(prefix)
				}

				pa.SetExcludePrefixes(tc.excludePrefixes)

				err := pa.Aggregate()
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
			pa.AddPrefix(prefix)
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
		pa.AddPrefix(prefix)
	}
	pa.Aggregate()

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
		pa.AddPrefix(prefix)
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

func BenchmarkStatisticsCalculation(b *testing.B) {
	pa := NewPrefixAggregator()
	prefixes := generateBenchmarkPrefixes(10000)

	for _, prefix := range prefixes {
		pa.AddPrefix(prefix)
	}
	pa.Aggregate()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats := pa.GetStats()
		_ = stats.ReductionRatio

		memStats := pa.GetMemoryStats()
		_ = memStats.AggregatorBytes
	}
}
