package main

import (
	"fmt"
	"log"

	"github.com/rretina/netjugo"
)

func main() {
	fmt.Println("Basic IP Prefix Aggregation Example")
	fmt.Println("===================================")

	// Create a new prefix aggregator
	aggregator := netjugo.NewPrefixAggregator()

	// Add some sample prefixes
	prefixes := []string{
		"192.168.1.0/24",
		"192.168.2.0/24",
		"192.168.3.0/24",
		"192.168.4.0/24",
		"10.0.0.0/24",
		"10.0.1.0/24",
		"2001:db8::/64",
		"2001:db8:1::/64",
	}

	fmt.Printf("Adding %d prefixes:\n", len(prefixes))
	for _, prefix := range prefixes {
		fmt.Printf("  %s\n", prefix)
		if err := aggregator.AddPrefix(prefix); err != nil {
			log.Fatalf("Failed to add prefix %s: %v", prefix, err)
		}
	}

	// Get initial stats
	initialStats := aggregator.GetStats()
	fmt.Printf("\nInitial Statistics:\n")
	fmt.Printf("  Total prefixes: %d\n", initialStats.TotalPrefixes)
	fmt.Printf("  IPv4 prefixes: %d\n", initialStats.IPv4PrefixCount)
	fmt.Printf("  IPv6 prefixes: %d\n", initialStats.IPv6PrefixCount)

	// Perform aggregation
	fmt.Println("\nPerforming aggregation...")
	if err := aggregator.Aggregate(); err != nil {
		log.Fatalf("Aggregation failed: %v", err)
	}

	// Get final stats
	finalStats := aggregator.GetStats()
	fmt.Printf("\nFinal Statistics:\n")
	fmt.Printf("  Original prefixes: %d\n", finalStats.OriginalCount)
	fmt.Printf("  Aggregated prefixes: %d\n", finalStats.TotalPrefixes)
	fmt.Printf("  IPv4 prefixes: %d\n", finalStats.IPv4PrefixCount)
	fmt.Printf("  IPv6 prefixes: %d\n", finalStats.IPv6PrefixCount)
	fmt.Printf("  Reduction ratio: %.2f%%\n", finalStats.ReductionRatio*100)
	fmt.Printf("  Processing time: %d ms\n", finalStats.ProcessingTimeMs)

	// Display results
	fmt.Println("\nAggregated prefixes:")
	results := aggregator.GetPrefixes()
	for _, prefix := range results {
		fmt.Printf("  %s\n", prefix)
	}

	// Memory usage
	memStats := aggregator.GetMemoryStats()
	fmt.Printf("\nMemory Usage:\n")
	fmt.Printf("  Aggregator: %d bytes\n", memStats.AggregatorBytes)
	fmt.Printf("  System allocation: %d bytes\n", memStats.AllocBytes)
}
