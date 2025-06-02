package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/rretina/netjugo"
)

func main() {
	// Command line flags
	var (
		inputFile   = flag.String("input", "", "Input file containing IP prefixes (one per line)")
		outputFile  = flag.String("output", "", "Output file for aggregated prefixes (default: stdout)")
		minIPv4Len  = flag.Int("min-ipv4", 0, "Minimum IPv4 prefix length (0-32)")
		minIPv6Len  = flag.Int("min-ipv6", 0, "Minimum IPv6 prefix length (0-128)")
		includeFile = flag.String("include", "", "File containing prefixes to include")
		excludeFile = flag.String("exclude", "", "File containing prefixes to exclude")
		includePfx  = flag.String("include-prefix", "", "Comma-separated list of prefixes to include")
		excludePfx  = flag.String("exclude-prefix", "", "Comma-separated list of prefixes to exclude")
		showStats   = flag.Bool("stats", false, "Show aggregation statistics")
		showMemory  = flag.Bool("memory", false, "Show memory usage statistics")
		verbose     = flag.Bool("verbose", false, "Verbose output")
		version     = flag.Bool("version", false, "Show version information")
	)

	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "IP Prefix Aggregation Tool\n")
		_, _ = fmt.Fprintf(os.Stderr, "Aggregates IPv4 and IPv6 CIDR prefixes with support for minimum lengths,\n")
		_, _ = fmt.Fprintf(os.Stderr, "inclusion/exclusion constraints, and optimal aggregation quality.\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		_, _ = fmt.Fprintf(os.Stderr, "\nExamples:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  %s -input prefixes.txt -output aggregated.txt -stats\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "  %s -input large.txt -min-ipv4 24 -min-ipv6 48 -verbose\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "  %s -input base.txt -include include.txt -exclude exclude.txt\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "  %s -input prefixes.txt -exclude-prefix '192.168.1.0/24,10.0.0.0/24'\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "\nInput Format:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  One IP prefix per line in CIDR notation (e.g., 192.168.1.0/24, 2001:db8::/32)\n")
		_, _ = fmt.Fprintf(os.Stderr, "  Comments (lines starting with #) and empty lines are ignored\n")
		_, _ = fmt.Fprintf(os.Stderr, "  IPv4 addresses without /xx will be treated as /32\n")
		_, _ = fmt.Fprintf(os.Stderr, "  IPv6 addresses without /xx will be treated as /128\n")
	}

	flag.Parse()

	if *version {
		fmt.Println("IP Aggregator v1.0.0")
		fmt.Println("High-performance Go library for IP prefix aggregation")
		fmt.Println("Supports IPv4/IPv6, minimum prefix lengths, inclusion/exclusion")
		return
	}

	if *inputFile == "" {
		_, _ = fmt.Fprintf(os.Stderr, "Error: input file is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// Validate minimum prefix lengths
	if *minIPv4Len < 0 || *minIPv4Len > 32 {
		log.Fatalf("Invalid IPv4 minimum prefix length: %d (must be 0-32)", *minIPv4Len)
	}
	if *minIPv6Len < 0 || *minIPv6Len > 128 {
		log.Fatalf("Invalid IPv6 minimum prefix length: %d (must be 0-128)", *minIPv6Len)
	}

	// Create aggregator
	aggregator := netjugo.NewPrefixAggregator()

	// Set minimum prefix lengths
	if *minIPv4Len > 0 || *minIPv6Len > 0 {
		if *verbose {
			fmt.Printf("Setting minimum prefix lengths: IPv4=%d, IPv6=%d\n", *minIPv4Len, *minIPv6Len)
		}
		if err := aggregator.SetMinPrefixLength(*minIPv4Len, *minIPv6Len); err != nil {
			log.Fatalf("Failed to set minimum prefix lengths: %v", err)
		}
	}

	// Process include prefixes
	if *includeFile != "" {
		if *verbose {
			fmt.Printf("Loading include prefixes from %s\n", *includeFile)
		}
		includePrefixes, err := readPrefixesFromFile(*includeFile)
		if err != nil {
			log.Fatalf("Failed to read include file: %v", err)
		}
		if err := aggregator.SetIncludePrefixes(includePrefixes); err != nil {
			log.Fatalf("Failed to set include prefixes: %v", err)
		}
		if *verbose {
			fmt.Printf("Loaded %d include prefixes\n", len(includePrefixes))
		}
	}

	if *includePfx != "" {
		prefixes := strings.Split(*includePfx, ",")
		for i := range prefixes {
			prefixes[i] = strings.TrimSpace(prefixes[i])
		}
		if err := aggregator.SetIncludePrefixes(prefixes); err != nil {
			log.Fatalf("Failed to set include prefixes: %v", err)
		}
		if *verbose {
			fmt.Printf("Added %d include prefixes from command line\n", len(prefixes))
		}
	}

	// Process exclude prefixes
	if *excludeFile != "" {
		if *verbose {
			fmt.Printf("Loading exclude prefixes from %s\n", *excludeFile)
		}
		excludePrefixes, err := readPrefixesFromFile(*excludeFile)
		if err != nil {
			log.Fatalf("Failed to read exclude file: %v", err)
		}
		if err := aggregator.SetExcludePrefixes(excludePrefixes); err != nil {
			log.Fatalf("Failed to set exclude prefixes: %v", err)
		}
		if *verbose {
			fmt.Printf("Loaded %d exclude prefixes\n", len(excludePrefixes))
		}
	}

	if *excludePfx != "" {
		prefixes := strings.Split(*excludePfx, ",")
		for i := range prefixes {
			prefixes[i] = strings.TrimSpace(prefixes[i])
		}
		if err := aggregator.SetExcludePrefixes(prefixes); err != nil {
			log.Fatalf("Failed to set exclude prefixes: %v", err)
		}
		if *verbose {
			fmt.Printf("Added %d exclude prefixes from command line\n", len(prefixes))
		}
	}

	// Load input prefixes
	if *verbose {
		fmt.Printf("Loading prefixes from %s\n", *inputFile)
	}
	if err := aggregator.AddFromFile(*inputFile); err != nil {
		log.Fatalf("Failed to load input file: %v", err)
	}

	initialStats := aggregator.GetStats()
	if *verbose {
		fmt.Printf("Loaded %d prefixes (%d IPv4, %d IPv6)\n",
			initialStats.OriginalCount, initialStats.IPv4PrefixCount, initialStats.IPv6PrefixCount)
	}

	// Set up warning handler for verbose mode
	if *verbose {
		aggregator.SetWarningHandler(func(msg string) {
			if _, err := fmt.Fprintf(os.Stderr, "%s\n", msg); err != nil {
				log.Printf("Failed to write warning: %v", err)
			}
		})
	}

	// Perform aggregation
	if *verbose {
		fmt.Println("Performing aggregation...")
	}
	if err := aggregator.Aggregate(); err != nil {
		log.Fatalf("Aggregation failed: %v", err)
	}

	// Get final statistics
	finalStats := aggregator.GetStats()

	// Show warnings if not in verbose mode (verbose mode shows them real-time)
	if !*verbose {
		warnings := aggregator.GetWarnings()
		for _, warning := range warnings {
			if _, err := fmt.Fprintf(os.Stderr, "%s\n", warning); err != nil {
				log.Printf("Failed to write warning: %v", err)
			}
		}
	}

	// Write output
	if *outputFile != "" {
		if err := aggregator.WriteToFile(*outputFile); err != nil {
			log.Fatalf("Failed to write output file: %v", err)
		}
		if *verbose {
			fmt.Printf("Wrote %d aggregated prefixes to %s\n", finalStats.TotalPrefixes, *outputFile)
		}
	} else {
		// Write to stdout
		if err := aggregator.WriteToWriter(os.Stdout); err != nil {
			log.Fatalf("Failed to write to stdout: %v", err)
		}
	}

	// Show statistics
	if *showStats || *verbose {
		_, _ = fmt.Fprintf(os.Stderr, "\nAggregation Statistics:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  Original prefixes: %d\n", finalStats.OriginalCount)
		_, _ = fmt.Fprintf(os.Stderr, "  Aggregated prefixes: %d\n", finalStats.TotalPrefixes)
		_, _ = fmt.Fprintf(os.Stderr, "  IPv4 prefixes: %d\n", finalStats.IPv4PrefixCount)
		_, _ = fmt.Fprintf(os.Stderr, "  IPv6 prefixes: %d\n", finalStats.IPv6PrefixCount)
		_, _ = fmt.Fprintf(os.Stderr, "  Reduction ratio: %.2f%%\n", finalStats.ReductionRatio*100)
		_, _ = fmt.Fprintf(os.Stderr, "  Processing time: %d ms\n", finalStats.ProcessingTimeMs)
		_, _ = fmt.Fprintf(os.Stderr, "  Memory usage: %s\n", formatBytes(finalStats.MemoryUsageBytes))
	}

	// Show memory statistics
	if *showMemory {
		memStats := aggregator.GetMemoryStats()
		_, _ = fmt.Fprintf(os.Stderr, "\nMemory Statistics:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  Aggregator memory: %s\n", formatBytes(memStats.AggregatorBytes))
		_, _ = fmt.Fprintf(os.Stderr, "  System allocation: %s\n", formatBytes(memStats.AllocBytes))
		_, _ = fmt.Fprintf(os.Stderr, "  Total allocated: %s\n", formatBytes(memStats.TotalAllocBytes))
		_, _ = fmt.Fprintf(os.Stderr, "  System memory: %s\n", formatBytes(memStats.SysBytes))
		_, _ = fmt.Fprintf(os.Stderr, "  GC runs: %d\n", memStats.NumGC)
	}
}

func readPrefixesFromFile(filename string) ([]string, error) {
	// Create a temporary aggregator to leverage the existing file reading logic
	tempAggregator := netjugo.NewPrefixAggregator()
	if err := tempAggregator.AddFromFile(filename); err != nil {
		return nil, err
	}
	return tempAggregator.GetPrefixes(), nil
}

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return strconv.FormatInt(bytes, 10) + " B"
	}
}
