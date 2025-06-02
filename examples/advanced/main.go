package main

import (
	"fmt"
	"log"
	"os"

	"github.com/rretina/netjugo"
)

func main() {
	fmt.Println("Advanced IP Prefix Aggregation Example")
	fmt.Println("======================================")

	// Create a new prefix aggregator
	aggregator := netjugo.NewPrefixAggregator()

	// Set minimum prefix lengths
	fmt.Println("Setting minimum prefix lengths: IPv4=/24, IPv6=/48")
	if err := aggregator.SetMinPrefixLength(24, 48); err != nil {
		log.Fatalf("Failed to set minimum prefix lengths: %v", err)
	}

	// Add base prefixes
	basePrefixes := []string{
		"192.168.0.0/16", // Will be split to /24s
		"10.0.0.0/22",    // Will be split to /24s
		"172.16.0.0/20",  // Will be split to /24s
		"2001:db8::/32",  // Will be split to /48s
	}

	fmt.Printf("\nAdding base prefixes (will be split to meet minimum lengths):\n")
	for _, prefix := range basePrefixes {
		fmt.Printf("  %s\n", prefix)
		if err := aggregator.AddPrefix(prefix); err != nil {
			log.Fatalf("Failed to add base prefix %s: %v", prefix, err)
		}
	}

	// Set include prefixes
	includePrefixes := []string{
		"203.0.113.0/24",
		"198.51.100.0/24",
		"2001:db8:1000::/48",
	}

	fmt.Printf("\nSetting include prefixes:\n")
	for _, prefix := range includePrefixes {
		fmt.Printf("  %s\n", prefix)
	}
	if err := aggregator.SetIncludePrefixes(includePrefixes); err != nil {
		log.Fatalf("Failed to set include prefixes: %v", err)
	}

	// Set exclude prefixes
	excludePrefixes := []string{
		"192.168.1.0/24",    // Exclude this specific /24
		"10.0.0.0/24",       // Exclude this /24
		"2001:db8:0:1::/64", // Exclude this /64 from the split /48s
	}

	fmt.Printf("\nSetting exclude prefixes:\n")
	for _, prefix := range excludePrefixes {
		fmt.Printf("  %s\n", prefix)
	}
	if err := aggregator.SetExcludePrefixes(excludePrefixes); err != nil {
		log.Fatalf("Failed to set exclude prefixes: %v", err)
	}

	// Get initial stats
	initialStats := aggregator.GetStats()
	fmt.Printf("\nInitial Statistics:\n")
	fmt.Printf("  Total prefixes: %d\n", initialStats.TotalPrefixes)

	// Perform aggregation
	fmt.Println("\nPerforming advanced aggregation...")
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

	// Display some results (limited to avoid flooding output)
	fmt.Println("\nSample IPv4 prefixes (first 10):")
	ipv4Results := aggregator.GetIPv4Prefixes()
	for i, prefix := range ipv4Results {
		if i >= 10 {
			fmt.Printf("  ... and %d more\n", len(ipv4Results)-10)
			break
		}
		fmt.Printf("  %s\n", prefix)
	}

	fmt.Println("\nSample IPv6 prefixes (first 10):")
	ipv6Results := aggregator.GetIPv6Prefixes()
	for i, prefix := range ipv6Results {
		if i >= 10 {
			fmt.Printf("  ... and %d more\n", len(ipv6Results)-10)
			break
		}
		fmt.Printf("  %s\n", prefix)
	}

	// Memory usage
	memStats := aggregator.GetMemoryStats()
	fmt.Printf("\nMemory Usage:\n")
	fmt.Printf("  Aggregator: %.2f KB\n", float64(memStats.AggregatorBytes)/1024)
	fmt.Printf("  System allocation: %.2f KB\n", float64(memStats.AllocBytes)/1024)

	// Write results to file
	outputFile := "/tmp/aggregated_prefixes.txt"
	fmt.Printf("\nWriting results to %s...\n", outputFile)
	if err := aggregator.WriteToFile(outputFile); err != nil {
		log.Fatalf("Failed to write results: %v", err)
	}

	// Check file size
	if info, err := os.Stat(outputFile); err == nil {
		fmt.Printf("Output file size: %d bytes\n", info.Size())
	}

	fmt.Println("\nAdvanced aggregation completed successfully!")
}
