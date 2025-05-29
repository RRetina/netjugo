package netjugo

import (
	"runtime"
	"time"
)

// AggregationStats contains statistics about the aggregation process
type AggregationStats struct {
	IPv4PrefixCount  int     // Number of IPv4 prefixes after aggregation
	IPv6PrefixCount  int     // Number of IPv6 prefixes after aggregation
	TotalPrefixes    int     // Total number of prefixes after aggregation
	OriginalCount    int     // Original number of prefixes before aggregation
	ReductionRatio   float64 // Reduction ratio (1 - final/original)
	ProcessingTimeMs int64   // Processing time in milliseconds
	MemoryUsageBytes int64   // Estimated memory usage in bytes
}

// statsCollector collects statistics during aggregation
type statsCollector struct {
	startTime      time.Time
	originalCount  int
	ipv4Count      int
	ipv6Count      int
	memoryBaseline uint64
}

// newStatsCollector creates a new statistics collector
func newStatsCollector() *statsCollector {
	return &statsCollector{
		startTime:      time.Now(),
		memoryBaseline: getCurrentMemoryUsage(),
	}
}

// getCurrentMemoryUsage returns the current memory usage
func getCurrentMemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

// recordOriginalCount records the original prefix count
func (sc *statsCollector) recordOriginalCount(count int) {
	sc.originalCount = count
}

// recordFinalCounts records the final prefix counts
func (sc *statsCollector) recordFinalCounts(ipv4Count, ipv6Count int) {
	sc.ipv4Count = ipv4Count
	sc.ipv6Count = ipv6Count
}

// getStats returns the collected statistics
func (sc *statsCollector) getStats() AggregationStats {
	processingTime := time.Since(sc.startTime).Milliseconds()
	totalPrefixes := sc.ipv4Count + sc.ipv6Count

	reductionRatio := 0.0
	if sc.originalCount > 0 {
		reductionRatio = 1.0 - float64(totalPrefixes)/float64(sc.originalCount)
		if reductionRatio < 0 {
			reductionRatio = 0
		}
	}

	currentMemory := getCurrentMemoryUsage()
	memoryUsage := int64(currentMemory - sc.memoryBaseline)
	if memoryUsage < 0 {
		// Account for garbage collection
		memoryUsage = estimateMemoryUsage(totalPrefixes)
	}

	return AggregationStats{
		IPv4PrefixCount:  sc.ipv4Count,
		IPv6PrefixCount:  sc.ipv6Count,
		TotalPrefixes:    totalPrefixes,
		OriginalCount:    sc.originalCount,
		ReductionRatio:   reductionRatio,
		ProcessingTimeMs: processingTime,
		MemoryUsageBytes: memoryUsage,
	}
}

// calculatePrefixStats calculates statistics for a list of prefixes
func calculatePrefixStats(prefixes []*IPPrefix) (ipv4Count, ipv6Count int) {
	for _, p := range prefixes {
		if p.IsIPv4() {
			ipv4Count++
		} else {
			ipv6Count++
		}
	}
	return ipv4Count, ipv6Count
}

// estimateAggregationPotential estimates how much a prefix list can be aggregated
func estimateAggregationPotential(prefixes []*IPPrefix) float64 {
	if len(prefixes) <= 1 {
		return 0.0
	}

	// Count adjacent and overlapping prefixes
	adjacentCount := 0
	overlappingCount := 0

	sortPrefixes(prefixes)

	for i := 0; i < len(prefixes)-1; i++ {
		if prefixes[i].Adjacent(prefixes[i+1]) {
			adjacentCount++
		} else if prefixes[i].Overlaps(prefixes[i+1]) {
			overlappingCount++
		}
	}

	// Estimate potential reduction
	potentialReduction := float64(adjacentCount+overlappingCount) / float64(len(prefixes))
	return potentialReduction
}

// MemoryStats provides detailed memory usage information
type MemoryStats struct {
	PrefixCount    int   // Number of prefixes
	BytesPerPrefix int64 // Average bytes per prefix
	TotalBytes     int64 // Total memory usage
	IPv4Bytes      int64 // Memory used by IPv4 prefixes
	IPv6Bytes      int64 // Memory used by IPv6 prefixes
}

// calculateMemoryStats calculates detailed memory statistics
func calculateMemoryStats(ipv4Prefixes, ipv6Prefixes []*IPPrefix) MemoryStats {
	ipv4Count := len(ipv4Prefixes)
	ipv6Count := len(ipv6Prefixes)
	totalCount := ipv4Count + ipv6Count

	// Estimate memory per prefix type
	const (
		basePerPrefix = 24 // netip.Prefix size
		uint256Size   = 32 // uint256.Int size
		pointerSize   = 8  // pointer size
		overhead      = 16 // slice and alignment overhead
	)

	bytesPerPrefix := int64(basePerPrefix + 2*pointerSize + 2*uint256Size + overhead)

	ipv4Bytes := int64(ipv4Count) * bytesPerPrefix
	ipv6Bytes := int64(ipv6Count) * bytesPerPrefix
	totalBytes := ipv4Bytes + ipv6Bytes

	return MemoryStats{
		PrefixCount:    totalCount,
		BytesPerPrefix: bytesPerPrefix,
		TotalBytes:     totalBytes,
		IPv4Bytes:      ipv4Bytes,
		IPv6Bytes:      ipv6Bytes,
	}
}

// PerformanceMetrics provides detailed performance information
type PerformanceMetrics struct {
	ParseTimeMs       int64   // Time spent parsing prefixes
	SortTimeMs        int64   // Time spent sorting
	AggregateTimeMs   int64   // Time spent aggregating
	ExclusionTimeMs   int64   // Time spent processing exclusions
	TotalTimeMs       int64   // Total processing time
	PrefixesPerSecond float64 // Processing rate
}

// performanceTimer helps track performance metrics
type performanceTimer struct {
	parseStart     time.Time
	parseEnd       time.Time
	sortStart      time.Time
	sortEnd        time.Time
	aggregateStart time.Time
	aggregateEnd   time.Time
	exclusionStart time.Time
	exclusionEnd   time.Time
}

// newPerformanceTimer creates a new performance timer
func newPerformanceTimer() *performanceTimer {
	return &performanceTimer{}
}

// startParse marks the start of parsing phase
func (pt *performanceTimer) startParse() {
	pt.parseStart = time.Now()
}

// endParse marks the end of parsing phase
func (pt *performanceTimer) endParse() {
	pt.parseEnd = time.Now()
}

// startSort marks the start of sorting phase
func (pt *performanceTimer) startSort() {
	pt.sortStart = time.Now()
}

// endSort marks the end of sorting phase
func (pt *performanceTimer) endSort() {
	pt.sortEnd = time.Now()
}

// startAggregate marks the start of aggregation phase
func (pt *performanceTimer) startAggregate() {
	pt.aggregateStart = time.Now()
}

// endAggregate marks the end of aggregation phase
func (pt *performanceTimer) endAggregate() {
	pt.aggregateEnd = time.Now()
}

// startExclusion marks the start of exclusion processing
func (pt *performanceTimer) startExclusion() {
	pt.exclusionStart = time.Now()
}

// endExclusion marks the end of exclusion processing
func (pt *performanceTimer) endExclusion() {
	pt.exclusionEnd = time.Now()
}

// getMetrics returns the collected performance metrics
func (pt *performanceTimer) getMetrics(prefixCount int) PerformanceMetrics {
	parseTime := int64(0)
	if !pt.parseEnd.IsZero() {
		parseTime = pt.parseEnd.Sub(pt.parseStart).Milliseconds()
	}

	sortTime := int64(0)
	if !pt.sortEnd.IsZero() {
		sortTime = pt.sortEnd.Sub(pt.sortStart).Milliseconds()
	}

	aggregateTime := int64(0)
	if !pt.aggregateEnd.IsZero() {
		aggregateTime = pt.aggregateEnd.Sub(pt.aggregateStart).Milliseconds()
	}

	exclusionTime := int64(0)
	if !pt.exclusionEnd.IsZero() {
		exclusionTime = pt.exclusionEnd.Sub(pt.exclusionStart).Milliseconds()
	}

	totalTime := parseTime + sortTime + aggregateTime + exclusionTime

	prefixesPerSecond := 0.0
	if totalTime > 0 {
		prefixesPerSecond = float64(prefixCount) / (float64(totalTime) / 1000.0)
	}

	return PerformanceMetrics{
		ParseTimeMs:       parseTime,
		SortTimeMs:        sortTime,
		AggregateTimeMs:   aggregateTime,
		ExclusionTimeMs:   exclusionTime,
		TotalTimeMs:       totalTime,
		PrefixesPerSecond: prefixesPerSecond,
	}
}
