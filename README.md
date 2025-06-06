# NetJugo

[![Go Reference](https://pkg.go.dev/badge/github.com/rretina/netjugo.svg)](https://pkg.go.dev/github.com/rretina/netjugo)
[![Go Report Card](https://goreportcard.com/badge/github.com/rretina/netjugo)](https://goreportcard.com/report/github.com/rretina/netjugo)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

NetJugo is a high-performance Go library for aggregating IPv4 and IPv6 prefixes with support for minimum prefix lengths, inclusion/exclusion constraints, and optimal aggregation quality. The library is designed to handle millions of prefixes efficiently using an array-based approach with uint256 integer representation.

## Features

- **High Performance**: Process millions of prefixes in seconds
- **Thread-Safe**: All operations are safe for concurrent use
- **Dual Stack**: Full support for both IPv4 and IPv6
- **Memory Efficient**: Uses object pooling and optimized data structures
- **Flexible Filtering**: Include/exclude specific prefixes
- **Minimum Prefix Length**: Enforce minimum aggregation boundaries
- **Statistics**: Detailed performance and memory metrics
- **Binary Search**: Fast exclusion processing with O(log n) lookups
- **Bare IP Support**: Automatically converts bare IPs to /32 (IPv4) or /128 (IPv6)

## Installation

```bash
go get github.com/rretina/netjugo
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    "github.com/rretina/netjugo"
)

func main() {
    // Create a new aggregator
    pa := netjugo.NewPrefixAggregator()
    
    // Add some prefixes
    prefixes := []string{
        "192.168.1.0/24",
        "192.168.2.0/24",
        "192.168.3.0/24",
        "192.168.4.0/24",
        "10.0.0.0/24",
        "10.0.1.0/24",
    }
    
    if err := pa.AddPrefixes(prefixes); err != nil {
        log.Fatal(err)
    }
    
    // Perform aggregation
    if err := pa.Aggregate(); err != nil {
        log.Fatal(err)
    }
    
    // Get results
    results := pa.GetPrefixes()
    stats := pa.GetStats()
    
    fmt.Printf("Aggregated %d prefixes to %d (%.2f%% reduction)\n",
        stats.OriginalCount,
        stats.TotalPrefixes,
        stats.ReductionRatio * 100)
    
    for _, prefix := range results {
        fmt.Println(prefix)
    }
}
```

## Advanced Usage

### Minimum Prefix Length

Control the granularity of aggregation by setting minimum prefix lengths:

```go
// Set minimum prefix lengths: IPv4 /24, IPv6 /48
err := pa.SetMinPrefixLength(24, 48)
if err != nil {
    log.Fatal(err)
}
```

### Include/Exclude Prefixes

Force inclusion or exclusion of specific prefixes:

```go
// Force these prefixes to be included
includes := []string{"10.1.0.0/16", "192.168.0.0/16"}
pa.SetIncludePrefixes(includes)

// Exclude these prefixes from aggregation
excludes := []string{"10.1.1.0/24", "192.168.100.0/24"}
pa.SetExcludePrefixes(excludes)
```

### Bare IP Address Support

NetJugo automatically handles bare IP addresses without prefix notation:

```go
// These bare IP addresses are automatically converted:
// - IPv4 addresses → /32
// - IPv6 addresses → /128
prefixes := []string{
    "192.168.1.1",      // Becomes 192.168.1.1/32
    "192.168.1.2",      // Becomes 192.168.1.2/32
    "2001:db8::1",      // Becomes 2001:db8::1/128
    "10.0.0.0/24",      // Regular CIDR notation works too
}

err := pa.AddPrefixes(prefixes)
```

### File Operations

Load prefixes from a file:

```go
// Load from file (one prefix per line)
err := pa.AddFromFile("prefixes.txt")
if err != nil {
    log.Fatal(err)
}

// Save results to file
err = pa.WriteToFile("aggregated.txt")
if err != nil {
    log.Fatal(err)
}
```

### Performance Monitoring

Get detailed statistics about the aggregation:

```go
stats := pa.GetStats()
fmt.Printf("IPv4 prefixes: %d\n", stats.IPv4PrefixCount)
fmt.Printf("IPv6 prefixes: %d\n", stats.IPv6PrefixCount)
fmt.Printf("Processing time: %dms\n", stats.ProcessingTimeMs)
fmt.Printf("Memory usage: %d MB\n", stats.MemoryUsageBytes/1024/1024)

// Get memory statistics
memStats := pa.GetMemoryStats()
fmt.Printf("Aggregator memory: %d MB\n", memStats.AggregatorBytes/1024/1024)
```

## Performance

NetJugo is optimized for high-performance prefix aggregation:

| Dataset Size | Processing Time | Memory Usage | Reduction Ratio |
|-------------|-----------------|--------------|-----------------|
| 1,000       | < 1ms          | ~1 MB        | 60-80%         |
| 10,000      | ~10ms          | ~8 MB        | 70-85%         |
| 100,000     | ~100ms         | ~80 MB       | 75-90%         |
| 1,000,000   | ~1.5s          | ~800 MB      | 80-95%         |

*Benchmarks run on Apple M1, 16GB RAM*

### Optimizations

- **Binary Search**: O(log n) lookups for exclusion processing
- **Memory Pooling**: Reduces GC pressure by 70%
- **In-Place Operations**: Minimizes memory allocations
- **Efficient Data Structures**: Uses uint256 for all IP calculations

## Best Practices

### Exclusion Prefix Recommendations

While NetJugo supports excluding individual IP addresses (/32 for IPv4, /128 for IPv6), this can significantly impact aggregation efficiency. For optimal performance:

- **IPv4**: Use exclusions of /30 or larger (recommended minimum: /30)
- **IPv6**: Use exclusions of /64 or larger (recommended minimum: /64)

Excluding more specific prefixes will generate warnings:

```go
pa := NewPrefixAggregator()

// Set up a warning handler to receive real-time warnings
pa.SetWarningHandler(func(msg string) {
    log.Println(msg)
})

// This will generate a warning
pa.SetExcludePrefixes([]string{"192.168.1.100/32"})
// WARNING: IPv4 exclusion 192.168.1.100/32 is more specific than recommended /30

// After aggregation, check all warnings
warnings := pa.GetWarnings()
for _, warning := range warnings {
    log.Println(warning)
}
```

### Why These Recommendations?

Excluding a single IP from a larger block requires creating multiple prefixes to represent the remaining addresses. For example, excluding one /32 from a /24 can create up to 8 new prefixes. For IPv6, excluding a single /128 can create dozens of prefixes, defeating the purpose of aggregation.

## CLI Tool

NetJugo includes a command-line tool for batch processing:

```bash
# Install the CLI tool
go install github.com/rretina/netjugo/cmd/ipaggregator@latest

# Basic usage
ipaggregator -input prefixes.txt -output aggregated.txt

# With options
ipaggregator -input prefixes.txt \
             -output aggregated.txt \
             -min-ipv4 24 \
             -min-ipv6 48 \
             -include includes.txt \
             -exclude excludes.txt \
             -stats
```

## Examples

See the [examples](examples/) directory for more detailed examples:

- [Basic Usage](examples/basic/main.go) - Simple aggregation example
- [Advanced Features](examples/advanced/main.go) - Include/exclude and file operations
- [Large Scale](examples/largescale/main.go) - Processing millions of prefixes
- [Bare IP Addresses](examples/bare_ip/main.go) - Working with bare IP addresses

## Documentation

- [API Documentation](docs/api.md) - Complete API reference
- [Architecture](docs/architecture.md) - Design and implementation details
- [Performance Guide](docs/performance.md) - Optimization tips and benchmarks

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## Testing

Run the test suite:

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run benchmarks
make benchmark
```

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [holiman/uint256](https://github.com/holiman/uint256) - High-performance 256-bit integers
- Go's `net/netip` package for efficient IP handling