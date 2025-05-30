# NetJugo Architecture Overview

## Introduction

NetJugo is a high-performance Go library for aggregating IPv4 and IPv6 prefixes with support for minimum prefix lengths, inclusion/exclusion constraints, and optimal aggregation quality. The library is designed to handle millions of prefixes efficiently using an array-based approach with uint256 integer representation.

## Core Design Principles

1. **Performance**: Optimized for handling millions of prefixes
2. **Memory Efficiency**: Uses memory pooling and efficient data structures
3. **Thread Safety**: All public methods are thread-safe
4. **Zero Allocation**: Minimizes allocations through object pooling
5. **Correctness**: Comprehensive error handling and validation

## Architecture Components

### Data Structures

#### IPPrefix
```go
type IPPrefix struct {
    Prefix netip.Prefix     // Original CIDR prefix
    Min    *uint256.Int     // Min IP value in prefix range (inclusive)
    Max    *uint256.Int     // Max IP value in prefix range (inclusive)
}
```

The `IPPrefix` structure represents an IP prefix with its range boundaries stored as 256-bit integers for efficient range calculations and comparisons.

#### PrefixAggregator
The main aggregator structure maintains separate slices for IPv4 and IPv6 prefixes to optimize processing:

- **Separate IP version handling**: IPv4 and IPv6 prefixes are stored and processed separately
- **Include/Exclude lists**: Separate lists for inclusion and exclusion operations
- **Thread-safe operations**: Uses `sync.RWMutex` for concurrent access
- **Statistics tracking**: Tracks original count and processing time

### Memory Management

#### Object Pooling
The library implements a `sync.Pool` for `IPPrefix` allocations to reduce GC pressure:

```go
var ipPrefixPool = sync.Pool{
    New: func() interface{} {
        return &IPPrefix{
            Min: new(uint256.Int),
            Max: new(uint256.Int),
        }
    },
}
```

This significantly reduces allocations when processing large datasets.

### Algorithm Flow

#### Aggregation Process
1. **Process Inclusions**: Add include prefixes to main lists
2. **Enforce Minimum Prefix Lengths**: Round up more specific prefixes
3. **Sort and Deduplicate**: Sort by minimum IP value and remove duplicates
4. **Initial Aggregation**: Merge adjacent and overlapping prefixes
5. **Process Exclusions**: Subtract excluded ranges using binary search
6. **Final Sort**: Ensure results are properly ordered

#### Binary Search for Exclusions
The library uses binary search to efficiently find overlapping prefixes during exclusion processing:

```go
func findOverlappingPrefixes(target *IPPrefix, prefixList []*IPPrefix) []*IPPrefix
```

This reduces the complexity from O(n) to O(log n) for finding overlaps.

### Prefix Operations

#### Minimum Prefix Length Enforcement
- Prefixes more specific than the minimum are rounded up
- Example: A /28 prefix with minimum /24 becomes /24
- Applied before aggregation to reduce unnecessary work

#### Aggregation Rules
1. **Containment**: If one prefix contains another, keep only the larger
2. **Adjacency**: Adjacent prefixes that form a valid CIDR are merged
3. **Overlap**: Overlapping prefixes are merged if they form a valid CIDR

#### Exclusion Processing
1. Find all prefixes that overlap with the exclusion
2. Split overlapping prefixes to exclude the specified range
3. Use recursive binary splitting for optimal results

## Performance Characteristics

### Time Complexity
- **Sorting**: O(n log n)
- **Deduplication**: O(n)
- **Aggregation**: O(n) per iteration, typically O(n log n) total
- **Exclusion**: O(m log n) where m is exclusions, n is prefixes

### Space Complexity
- **Memory**: O(n) where n is the number of prefixes
- **Additional**: O(1) for aggregation operations

### Optimization Techniques
1. **In-place operations**: Minimize memory allocations
2. **Binary search**: Fast overlap detection
3. **Memory pooling**: Reuse objects
4. **Batch processing**: Process multiple operations together

## Error Handling

The library uses typed errors for better error handling:

```go
var (
    ErrInvalidPrefix        = errors.New("invalid IP prefix")
    ErrInvalidMinPrefixLen  = errors.New("invalid minimum prefix length")
    ErrUnsupportedIPVersion = errors.New("unsupported IP version")
    // ...
)
```

All errors are wrapped with context for debugging.

## Thread Safety

### Locking Strategy
- **Read operations**: Use `RLock()` for concurrent reads
- **Write operations**: Use `Lock()` for exclusive access
- **Defer unlocking**: Ensures locks are always released

### Safe Operations
All public methods are thread-safe and can be called concurrently.

## Integration Patterns

### Basic Usage
```go
pa := NewPrefixAggregator()
pa.AddPrefixes(prefixes)
pa.Aggregate()
results := pa.GetPrefixes()
```

### Advanced Usage
```go
pa := NewPrefixAggregator()
pa.SetMinPrefixLength(24, 48)
pa.SetIncludePrefixes(includes)
pa.SetExcludePrefixes(excludes)
pa.AddFromFile("prefixes.txt")
pa.Aggregate()
```

## Future Enhancements

1. **Parallel Processing**: Process IPv4 and IPv6 concurrently
2. **Streaming Mode**: Support for extremely large datasets
3. **Incremental Updates**: Add/remove prefixes without full re-aggregation
4. **Custom Aggregation Rules**: Pluggable aggregation strategies