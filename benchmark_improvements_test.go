package netjugo

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/holiman/uint256"
)

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

// Helper function
func parseNetipPrefix(s string) (netip.Prefix, error) {
	return netip.ParsePrefix(s)
}
