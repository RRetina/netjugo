package main

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/rretina/netjugo"
)

func main() {
	fmt.Println("NetJugo 8M+ Prefix Performance Test")
	fmt.Println("===================================")

	// Get initial memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	initialMem := m.Alloc

	// Create aggregator
	pa := netjugo.NewPrefixAggregator()

	// Load 8M+ prefixes
	fmt.Println("\nLoading prefixes from .samples/large-dataset-prefixes.txt...")
	start := time.Now()

	err := pa.AddFromFile("../../.samples/large-dataset-prefixes.txt")
	if err != nil {
		log.Fatalf("Failed to load file: %v", err)
	}

	loadTime := time.Since(start)
	stats := pa.GetStats()
	fmt.Printf("Loaded %d prefixes in %v\n", stats.OriginalCount, loadTime)

	// Check memory after loading
	runtime.ReadMemStats(&m)
	loadMem := m.Alloc - initialMem
	fmt.Printf("Memory used after loading: %.2f MB\n", float64(loadMem)/1024/1024)

	// Set minimum prefix lengths (common for routing tables)
	fmt.Println("\nSetting minimum prefix lengths: IPv4=/24, IPv6=/48...")
	err = pa.SetMinPrefixLength(24, 48)
	if err != nil {
		log.Fatalf("Failed to set min prefix length: %v", err)
	}

	// Perform aggregation
	fmt.Println("\nPerforming aggregation...")
	start = time.Now()

	err = pa.Aggregate()
	if err != nil {
		log.Fatalf("Failed to aggregate: %v", err)
	}

	aggregateTime := time.Since(start)

	// Get final results
	stats = pa.GetStats()
	memStats := pa.GetMemoryStats()

	// Check memory after aggregation
	runtime.ReadMemStats(&m)
	peakMem := m.Alloc - initialMem

	// Display results
	fmt.Println("\n=== RESULTS ===")
	fmt.Printf("Original prefixes:   %d\n", stats.OriginalCount)
	fmt.Printf("IPv4 prefixes:       %d\n", stats.IPv4PrefixCount)
	fmt.Printf("IPv6 prefixes:       %d\n", stats.IPv6PrefixCount)
	fmt.Printf("Total after:         %d\n", stats.TotalPrefixes)
	fmt.Printf("Reduction ratio:     %.2f%%\n", stats.ReductionRatio*100)
	fmt.Println("\n=== PERFORMANCE ===")
	fmt.Printf("Load time:           %v\n", loadTime)
	fmt.Printf("Aggregation time:    %v\n", aggregateTime)
	fmt.Printf("Total time:          %v\n", loadTime+aggregateTime)
	fmt.Printf("Processing speed:    %.0f prefixes/sec\n", float64(stats.OriginalCount)/aggregateTime.Seconds())
	fmt.Println("\n=== MEMORY USAGE ===")
	fmt.Printf("After loading:       %.2f MB\n", float64(loadMem)/1024/1024)
	fmt.Printf("Peak usage:          %.2f MB\n", float64(peakMem)/1024/1024)
	fmt.Printf("Aggregator reported: %.2f MB\n", float64(memStats.AggregatorBytes)/1024/1024)
	fmt.Printf("Bytes per prefix:    %.0f\n", float64(peakMem)/float64(stats.OriginalCount))

	// Test SoW requirements
	fmt.Println("\n=== SOW REQUIREMENT VALIDATION ===")
	fmt.Printf("Requirement: Process 1M prefixes in < 10 seconds\n")
	fmt.Printf("Result: Processed %.1fM prefixes in %.2f seconds ", float64(stats.OriginalCount)/1000000, aggregateTime.Seconds())
	if stats.OriginalCount >= 1000000 && aggregateTime.Seconds() < 10 {
		fmt.Println("✓ PASS")
	} else if aggregateTime.Seconds() < 10*float64(stats.OriginalCount)/1000000 {
		fmt.Println("✓ PASS (extrapolated)")
	} else {
		fmt.Println("✗ FAIL")
	}

	fmt.Printf("\nRequirement: Use < 1GB memory for 8M prefixes\n")
	fmt.Printf("Result: Used %.2f MB for %.1fM prefixes ", float64(peakMem)/1024/1024, float64(stats.OriginalCount)/1000000)
	if peakMem < 1024*1024*1024 {
		fmt.Println("✓ PASS")
	} else {
		fmt.Println("✗ FAIL")
	}

	// Force GC and check memory again
	runtime.GC()
	runtime.ReadMemStats(&m)
	finalMem := m.Alloc - initialMem
	fmt.Printf("\nMemory after GC:     %.2f MB\n", float64(finalMem)/1024/1024)
}
