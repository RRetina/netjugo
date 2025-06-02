# NetJugo API Documentation

## Overview

NetJugo provides a simple yet powerful API for IP prefix aggregation. This document covers all public types, methods, and their usage.

## Types

### PrefixAggregator

The main type for prefix aggregation operations.

```go
type PrefixAggregator struct {
    // Internal fields (not exposed)
}
```

### AggregationStats

Statistics about the aggregation process.

```go
type AggregationStats struct {
    IPv4PrefixCount     int     // Number of IPv4 prefixes after aggregation
    IPv6PrefixCount     int     // Number of IPv6 prefixes after aggregation
    TotalPrefixes       int     // Total number of prefixes
    OriginalCount       int     // Original number of prefixes before aggregation
    ReductionRatio      float64 // Ratio of reduction (0.0 to 1.0)
    ProcessingTimeMs    int64   // Processing time in milliseconds
    MemoryUsageBytes    int64   // Memory usage in bytes
}
```

### MemoryStats

Detailed memory usage statistics.

```go
type MemoryStats struct {
    AllocBytes      int64 // Currently allocated memory
    TotalAllocBytes int64 // Total allocated memory
    SysBytes        int64 // System memory
    NumGC           int64 // Number of GC cycles
    AggregatorBytes int64 // Memory used by aggregator
}
```

## Constructor

### NewPrefixAggregator

Creates a new prefix aggregator instance.

```go
func NewPrefixAggregator() *PrefixAggregator
```

**Example:**
```go
pa := netjugo.NewPrefixAggregator()
```

## Configuration Methods

### SetMinPrefixLength

Sets the minimum prefix length for aggregation.

```go
func (pa *PrefixAggregator) SetMinPrefixLength(ipv4Len, ipv6Len int) error
```

**Parameters:**
- `ipv4Len`: Minimum prefix length for IPv4 (0-32)
- `ipv6Len`: Minimum prefix length for IPv6 (0-128)

**Returns:**
- `error`: Error if lengths are invalid

**Example:**
```go
err := pa.SetMinPrefixLength(24, 48)  // IPv4 /24, IPv6 /48
if err != nil {
    log.Fatal(err)
}
```

### SetIncludePrefixes

Sets prefixes to be included in the aggregation.

```go
func (pa *PrefixAggregator) SetIncludePrefixes(prefixes []string) error
```

**Parameters:**
- `prefixes`: Slice of CIDR prefixes to include

**Returns:**
- `error`: Error if any prefix is invalid

**Example:**
```go
includes := []string{"10.0.0.0/8", "172.16.0.0/12"}
err := pa.SetIncludePrefixes(includes)
```

### SetExcludePrefixes

Sets prefixes to be excluded from the aggregation.

```go
func (pa *PrefixAggregator) SetExcludePrefixes(prefixes []string) error
```

**Parameters:**
- `prefixes`: Slice of CIDR prefixes to exclude

**Returns:**
- `error`: Error if any prefix is invalid

**Example:**
```go
excludes := []string{"10.1.0.0/16", "172.16.1.0/24"}
err := pa.SetExcludePrefixes(excludes)
```

## Prefix Management Methods

### AddPrefix

Adds a single prefix to the aggregator.

```go
func (pa *PrefixAggregator) AddPrefix(prefixStr string) error
```

**Parameters:**
- `prefixStr`: CIDR prefix string (e.g., "192.168.1.0/24")

**Returns:**
- `error`: Error if prefix is invalid

**Example:**
```go
err := pa.AddPrefix("192.168.1.0/24")
```

### AddPrefixes

Adds multiple prefixes to the aggregator.

```go
func (pa *PrefixAggregator) AddPrefixes(prefixes []string) error
```

**Parameters:**
- `prefixes`: Slice of CIDR prefix strings

**Returns:**
- `error`: Error if any prefix is invalid

**Example:**
```go
prefixes := []string{
    "192.168.1.0/24",
    "192.168.2.0/24",
    "10.0.0.0/16",
}
err := pa.AddPrefixes(prefixes)
```

### AddFromFile

Loads prefixes from a file.

```go
func (pa *PrefixAggregator) AddFromFile(path string) error
```

**Parameters:**
- `path`: Path to file containing prefixes (one per line)

**Returns:**
- `error`: Error if file cannot be read or contains invalid data

**File Format:**
- One prefix per line
- Lines starting with # are ignored (comments)
- Empty lines are ignored
- Supports both IPv4 and IPv6 prefixes

**Example:**
```go
err := pa.AddFromFile("/path/to/prefixes.txt")
```

### AddFromReader

Loads prefixes from an io.Reader.

```go
func (pa *PrefixAggregator) AddFromReader(reader io.Reader) error
```

**Parameters:**
- `reader`: Any io.Reader containing prefix data

**Returns:**
- `error`: Error if reader fails or contains invalid data

**Example:**
```go
data := strings.NewReader("192.168.1.0/24\n10.0.0.0/8")
err := pa.AddFromReader(data)
```

## Processing Methods

### Aggregate

Performs the aggregation process.

```go
func (pa *PrefixAggregator) Aggregate() error
```

**Returns:**
- `error`: Error if aggregation fails

**Process:**
1. Process inclusions
2. Enforce minimum prefix lengths
3. Sort and deduplicate
4. Aggregate overlapping/adjacent prefixes
5. Process exclusions

**Example:**
```go
err := pa.Aggregate()
if err != nil {
    log.Fatal(err)
}
```

### Reset

Clears all data and resets the aggregator.

```go
func (pa *PrefixAggregator) Reset() error
```

**Returns:**
- `error`: Always returns nil (for interface consistency)

**Example:**
```go
err := pa.Reset()
```

## Output Methods

### GetPrefixes

Returns all aggregated prefixes (IPv4 and IPv6).

```go
func (pa *PrefixAggregator) GetPrefixes() []string
```

**Returns:**
- `[]string`: Slice of CIDR prefix strings

**Example:**
```go
results := pa.GetPrefixes()
for _, prefix := range results {
    fmt.Println(prefix)
}
```

### GetIPv4Prefixes

Returns only IPv4 aggregated prefixes.

```go
func (pa *PrefixAggregator) GetIPv4Prefixes() []string
```

**Returns:**
- `[]string`: Slice of IPv4 CIDR prefix strings

**Example:**
```go
ipv4Results := pa.GetIPv4Prefixes()
```

### GetIPv6Prefixes

Returns only IPv6 aggregated prefixes.

```go
func (pa *PrefixAggregator) GetIPv6Prefixes() []string
```

**Returns:**
- `[]string`: Slice of IPv6 CIDR prefix strings

**Example:**
```go
ipv6Results := pa.GetIPv6Prefixes()
```

### GetStats

Returns aggregation statistics.

```go
func (pa *PrefixAggregator) GetStats() AggregationStats
```

**Returns:**
- `AggregationStats`: Statistics about the aggregation

**Example:**
```go
stats := pa.GetStats()
fmt.Printf("Reduced %d prefixes to %d (%.2f%% reduction)\n",
    stats.OriginalCount,
    stats.TotalPrefixes,
    stats.ReductionRatio * 100)
```

### GetMemoryStats

Returns memory usage statistics.

```go
func (pa *PrefixAggregator) GetMemoryStats() MemoryStats
```

**Returns:**
- `MemoryStats`: Detailed memory usage information

**Example:**
```go
memStats := pa.GetMemoryStats()
fmt.Printf("Memory usage: %d MB\n", memStats.AggregatorBytes/1024/1024)
```

### WriteToFile

Writes aggregated prefixes to a file.

```go
func (pa *PrefixAggregator) WriteToFile(path string) error
```

**Parameters:**
- `path`: Output file path

**Returns:**
- `error`: Error if file cannot be written

**Example:**
```go
err := pa.WriteToFile("/path/to/output.txt")
```

### WriteToWriter

Writes aggregated prefixes to an io.Writer.

```go
func (pa *PrefixAggregator) WriteToWriter(writer io.Writer) error
```

**Parameters:**
- `writer`: Any io.Writer

**Returns:**
- `error`: Error if write fails

**Example:**
```go
var buf bytes.Buffer
err := pa.WriteToWriter(&buf)
```

## Error Types

The library defines several error types for better error handling:

```go
var (
    ErrInvalidPrefix        = errors.New("invalid IP prefix")
    ErrInvalidMinPrefixLen  = errors.New("invalid minimum prefix length")
    ErrUnsupportedIPVersion = errors.New("unsupported IP version")
    ErrNilPointer           = errors.New("nil pointer reference")
    ErrFileNotFound         = errors.New("file not found")
    ErrInvalidFormat        = errors.New("invalid file format")
)
```

## Thread Safety

All public methods are thread-safe and can be called concurrently. The library uses read-write mutexes to allow multiple concurrent read operations while ensuring exclusive write access.

## Complete Example

```go
package main

import (
    "fmt"
    "log"
    "github.com/rretina/netjugo"
)

func main() {
    // Create aggregator
    pa := netjugo.NewPrefixAggregator()
    
    // Configure
    err := pa.SetMinPrefixLength(24, 48)
    if err != nil {
        log.Fatal(err)
    }
    
    // Add prefixes
    prefixes := []string{
        "192.168.1.0/24",
        "192.168.2.0/24",
        "192.168.3.0/24",
        "192.168.4.0/24",
    }
    err = pa.AddPrefixes(prefixes)
    if err != nil {
        log.Fatal(err)
    }
    
    // Aggregate
    err = pa.Aggregate()
    if err != nil {
        log.Fatal(err)
    }
    
    // Get results
    results := pa.GetPrefixes()
    stats := pa.GetStats()
    
    // Display
    fmt.Printf("Aggregated %d prefixes to %d\n", 
        stats.OriginalCount, stats.TotalPrefixes)
    for _, prefix := range results {
        fmt.Println(prefix)
    }
}
```
## Warning Management

### SetWarningHandler

Sets a custom handler function to receive warnings in real-time during processing.

\\n
**Parameters:**
- \: Function to call for each warning message

**Example:**
\\n
### GetWarnings

Returns all warnings generated during the last aggregation.

\\n
**Returns:**
- \: Slice of warning messages (nil if no warnings)

**Example:**
\\n
**Common Warnings:**
- Exclusion prefixes more specific than recommended minimums (/30 for IPv4, /64 for IPv6)


## Warning Management

### SetWarningHandler

Sets a custom handler function to receive warnings in real-time during processing.

```go
func (pa *PrefixAggregator) SetWarningHandler(handler func(string))
```

**Parameters:**
- `handler`: Function to call for each warning message

**Example:**
```go
pa.SetWarningHandler(func(msg string) {
    log.Printf("Warning: %s", msg)
})
```

### GetWarnings

Returns all warnings generated during the last aggregation.

```go
func (pa *PrefixAggregator) GetWarnings() []string
```

**Returns:**
- `[]string`: Slice of warning messages (nil if no warnings)

**Example:**
```go
warnings := pa.GetWarnings()
for _, warning := range warnings {
    fmt.Fprintf(os.Stderr, "%s\n", warning)
}
```

**Common Warnings:**
- Exclusion prefixes more specific than recommended minimums (/30 for IPv4, /64 for IPv6)