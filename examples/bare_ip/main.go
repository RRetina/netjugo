package main

import (
	"fmt"
	"log"

	"github.com/rretina/netjugo"
)

func main() {
	fmt.Println("NetJugo - Bare IP Address Example")
	fmt.Println("==================================")

	pa := netjugo.NewPrefixAggregator()

	// Add a mix of bare IP addresses and CIDR prefixes
	prefixes := []string{
		// Bare IPv4 addresses (will be treated as /32)
		"192.168.1.1",
		"192.168.1.2",
		"192.168.1.3",
		"192.168.1.4",

		// IPv4 with explicit prefix
		"192.168.2.0/24",

		// Bare IPv6 addresses (will be treated as /128)
		"2001:db8::1",
		"2001:db8::2",
		"2001:db8::3",
		"2001:db8::4",

		// IPv6 with explicit prefix
		"2001:db8:1::/64",

		// Another bare IPv4
		"10.0.0.1",
		"10.0.0.2/32", // This is the same as above but explicit
	}

	fmt.Println("\nInput prefixes:")
	for _, p := range prefixes {
		fmt.Printf("  %s\n", p)
	}

	// Add prefixes
	if err := pa.AddPrefixes(prefixes); err != nil {
		log.Fatalf("Failed to add prefixes: %v", err)
	}

	// Perform aggregation
	if err := pa.Aggregate(); err != nil {
		log.Fatalf("Failed to aggregate: %v", err)
	}

	// Get results
	results := pa.GetPrefixes()
	stats := pa.GetStats()

	fmt.Printf("\nAggregation Results:")
	fmt.Printf("\n  Original: %d prefixes", stats.OriginalCount)
	fmt.Printf("\n  After:    %d prefixes", len(results))
	fmt.Printf("\n  Reduction: %.1f%%\n", stats.ReductionRatio*100)

	fmt.Println("\nAggregated prefixes:")
	for _, prefix := range results {
		fmt.Printf("  %s\n", prefix)
	}

	// Demonstrate exclusion with bare IP
	fmt.Println("\n\nExclusion Example:")
	fmt.Println("==================")

	pa2 := netjugo.NewPrefixAggregator()

	// Add a network
	if err := pa2.AddPrefix("192.168.10.0/24"); err != nil {
		log.Fatalf("Failed to add prefix: %v", err)
	}

	// Exclude specific IPs (bare addresses)
	excludes := []string{
		"192.168.10.100", // Router
		"192.168.10.101", // DNS server
		"192.168.10.102", // Mail server
	}

	fmt.Printf("\nNetwork: 192.168.10.0/24")
	fmt.Printf("\nExcluding IPs:")
	for _, ip := range excludes {
		fmt.Printf("\n  %s", ip)
	}

	if err := pa2.SetExcludePrefixes(excludes); err != nil {
		log.Fatalf("Failed to set exclude prefixes: %v", err)
	}

	if err := pa2.Aggregate(); err != nil {
		log.Fatalf("Failed to aggregate: %v", err)
	}

	results2 := pa2.GetPrefixes()
	fmt.Printf("\n\nResult: %d prefixes (excluding the 3 IPs)\n", len(results2))

	// Show first few results
	fmt.Println("First 10 prefixes:")
	for i, prefix := range results2 {
		if i >= 10 {
			fmt.Printf("  ... and %d more\n", len(results2)-10)
			break
		}
		fmt.Printf("  %s\n", prefix)
	}
}
