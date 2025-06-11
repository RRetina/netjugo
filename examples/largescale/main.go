package main

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/rretina/netjugo"
)

func main() {
	fmt.Println("Large Scale IP Prefix Aggregation Example")
	fmt.Println("=========================================")

	// Create a new prefix aggregator
	aggregator := netjugo.NewPrefixAggregator()

	// Check if large dataset exists
	datasetPath := ".samples/large-dataset-prefixes.txt"

	fmt.Printf("Attempting to load large dataset from: %s\n", datasetPath)

	// Monitor memory before loading
	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	fmt.Printf("Memory before loading: %.2f MB\n", float64(m1.Alloc)/(1024*1024))

	// Load the dataset
	start := time.Now()
	if err := aggregator.AddFromFile(datasetPath); err != nil {
		log.Fatalf("Failed to load dataset: %v", err)
	}
	loadTime := time.Since(start)

	// Get loading stats
	initialStats := aggregator.GetStats()
	initialMemStats := aggregator.GetMemoryStats()

	fmt.Printf("\nDataset Loading Results:\n")
	fmt.Printf("  Loading time: %v\n", loadTime)
	fmt.Printf("  Total prefixes loaded: %d\n", initialStats.OriginalCount)
	fmt.Printf("  IPv4 prefixes: %d\n", initialStats.IPv4PrefixCount)
	fmt.Printf("  IPv6 prefixes: %d\n", initialStats.IPv6PrefixCount)
	fmt.Printf("  Aggregator memory: %.2f MB\n", float64(initialMemStats.AggregatorBytes)/(1024*1024))
	fmt.Printf("  System memory: %.2f MB\n", float64(initialMemStats.AllocBytes)/(1024*1024))

	// Check if we meet the loading performance requirements
	expectedRate := float64(initialStats.OriginalCount) / loadTime.Seconds()
	fmt.Printf("  Loading rate: %.0f prefixes/second\n", expectedRate)

	// Memory constraint check
	maxMemoryMB := 1024.0 // 1GB limit from SOW
	actualMemoryMB := float64(initialMemStats.AggregatorBytes) / (1024 * 1024)
	if actualMemoryMB > maxMemoryMB {
		fmt.Printf("  WARNING: Memory usage %.2f MB exceeds 1GB limit\n", actualMemoryMB)
	} else {
		fmt.Printf("  Memory usage within 1GB limit (%.2f MB used)\n", actualMemoryMB)
	}

	// For demonstration, we'll aggregate a smaller subset to avoid excessive time
	fmt.Println("\nNote: For this demo, aggregating first 100k prefixes to avoid long runtime")

	// Create a new aggregator for the subset
	subsetAggregator := netjugo.NewPrefixAggregator()
	subsetPath := ".testdata/large_dataset.txt"

	if err := subsetAggregator.AddFromFile(subsetPath); err != nil {
		log.Printf("Sample file not found, proceeding with full dataset aggregation...")
		// Use the full aggregator
		performAggregation(aggregator, "Full Dataset")
	} else {
		// Use subset for demo
		performAggregation(subsetAggregator, "100k Sample")
	}

	fmt.Println("\nLarge scale processing completed successfully!")
}

func performAggregation(aggregator *netjugo.PrefixAggregator, datasetName string) {
	fmt.Printf("\nPerforming aggregation on %s...\n", datasetName)

	// Monitor memory before aggregation
	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	memBefore := m1.Alloc

	start := time.Now()
	if err := aggregator.Aggregate(); err != nil {
		log.Fatalf("Aggregation failed: %v", err)
	}
	aggregationTime := time.Since(start)

	// Monitor memory after aggregation
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	memAfter := m2.Alloc

	// Get final stats
	finalStats := aggregator.GetStats()
	finalMemStats := aggregator.GetMemoryStats()

	fmt.Printf("\nAggregation Results for %s:\n", datasetName)
	fmt.Printf("  Aggregation time: %v\n", aggregationTime)
	fmt.Printf("  Original prefixes: %d\n", finalStats.OriginalCount)
	fmt.Printf("  Aggregated prefixes: %d\n", finalStats.TotalPrefixes)
	fmt.Printf("  IPv4 prefixes: %d\n", finalStats.IPv4PrefixCount)
	fmt.Printf("  IPv6 prefixes: %d\n", finalStats.IPv6PrefixCount)
	fmt.Printf("  Reduction ratio: %.2f%%\n", finalStats.ReductionRatio*100)
	fmt.Printf("  Processing time: %d ms\n", finalStats.ProcessingTimeMs)

	// Performance analysis
	prefixesPerSecond := float64(finalStats.OriginalCount) / aggregationTime.Seconds()
	fmt.Printf("  Processing rate: %.0f prefixes/second\n", prefixesPerSecond)

	// Memory analysis
	fmt.Printf("\nMemory Analysis:\n")
	fmt.Printf("  Memory before aggregation: %.2f MB\n", float64(memBefore)/(1024*1024))
	fmt.Printf("  Memory after aggregation: %.2f MB\n", float64(memAfter)/(1024*1024))
	fmt.Printf("  Memory change: %.2f MB\n", float64(int64(memAfter)-int64(memBefore))/(1024*1024))
	fmt.Printf("  Aggregator memory: %.2f MB\n", float64(finalMemStats.AggregatorBytes)/(1024*1024))
	fmt.Printf("  GC runs: %d\n", finalMemStats.NumGC)

	// Efficiency metrics
	originalSize := finalStats.OriginalCount
	finalSize := finalStats.TotalPrefixes
	efficiency := float64(originalSize-finalSize) / float64(originalSize) * 100

	fmt.Printf("\nEfficiency Metrics:\n")
	fmt.Printf("  Space efficiency: %.2f%% reduction\n", efficiency)
	fmt.Printf("  Memory per prefix: %.2f bytes\n", float64(finalMemStats.AggregatorBytes)/float64(finalStats.TotalPrefixes))

	// Performance requirements check (from SOW)
	fmt.Printf("\nPerformance Requirements Check:\n")

	// Check 100K prefixes in under 5 seconds requirement
	if finalStats.OriginalCount >= 100000 {
		maxTime := 5 * time.Second
		if aggregationTime <= maxTime {
			fmt.Printf("  ✓ 100K+ prefixes aggregated in %v (requirement: <%v)\n", aggregationTime, maxTime)
		} else {
			fmt.Printf("  ✗ 100K+ prefixes took %v (requirement: <%v)\n", aggregationTime, maxTime)
		}
	}

	// Check 1M prefixes in under 10 seconds requirement
	if finalStats.OriginalCount >= 1000000 {
		maxTime := 10 * time.Second
		if aggregationTime <= maxTime {
			fmt.Printf("  ✓ 1M+ prefixes aggregated in %v (requirement: <%v)\n", aggregationTime, maxTime)
		} else {
			fmt.Printf("  ✗ 1M+ prefixes took %v (requirement: <%v)\n", aggregationTime, maxTime)
		}
	}

	// Memory requirement check
	maxMemoryBytes := int64(100 * 1024 * 1024) // 100MB for 1M prefixes
	if finalStats.OriginalCount >= 1000000 {
		if finalMemStats.AggregatorBytes <= maxMemoryBytes {
			fmt.Printf("  ✓ Memory usage %.2f MB for 1M+ prefixes (requirement: <100MB)\n",
				float64(finalMemStats.AggregatorBytes)/(1024*1024))
		} else {
			fmt.Printf("  ✗ Memory usage %.2f MB for 1M+ prefixes (requirement: <100MB)\n",
				float64(finalMemStats.AggregatorBytes)/(1024*1024))
		}
	}
}
