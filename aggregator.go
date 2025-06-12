package netjugo

import (
	"bufio"
	"fmt"
	"io"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/holiman/uint256"
)

// Memory pool for IPPrefix allocations to reduce GC pressure
var ipPrefixPool = sync.Pool{
	New: func() interface{} {
		return &IPPrefix{
			Min: new(uint256.Int),
			Max: new(uint256.Int),
		}
	},
}

type IPPrefix struct {
	Prefix netip.Prefix
	Min    *uint256.Int
	Max    *uint256.Int
}

type PrefixAggregator struct {
	IPv4Prefixes     []*IPPrefix
	IPv6Prefixes     []*IPPrefix
	IncludeIPv4      []*IPPrefix
	IncludeIPv6      []*IPPrefix
	ExcludeIPv4      []*IPPrefix
	ExcludeIPv6      []*IPPrefix
	MinPrefixLenIPv4 int
	MinPrefixLenIPv6 int
	mu               sync.RWMutex
	originalCount    int
	lastProcessTime  time.Duration
	warnings         []string
	warningHandler   func(string)
}

type AggregationStats struct {
	IPv4PrefixCount  int
	IPv6PrefixCount  int
	TotalPrefixes    int
	OriginalCount    int
	ReductionRatio   float64
	ProcessingTimeMs int64
	MemoryUsageBytes int64
}

type MemoryStats struct {
	AllocBytes      int64
	TotalAllocBytes int64
	SysBytes        int64
	NumGC           int64
	AggregatorBytes int64
}

// acquireIPPrefix gets an IPPrefix from the pool
func acquireIPPrefix() *IPPrefix {
	return ipPrefixPool.Get().(*IPPrefix)
}

// releaseIPPrefix returns an IPPrefix to the pool
func releaseIPPrefix(p *IPPrefix) {
	// Clear the prefix before returning to pool
	p.Prefix = netip.Prefix{}
	p.Min.Clear()
	p.Max.Clear()
	ipPrefixPool.Put(p)
}

func NewPrefixAggregator() *PrefixAggregator {
	return &PrefixAggregator{
		IPv4Prefixes:     make([]*IPPrefix, 0),
		IPv6Prefixes:     make([]*IPPrefix, 0),
		IncludeIPv4:      make([]*IPPrefix, 0),
		IncludeIPv6:      make([]*IPPrefix, 0),
		ExcludeIPv4:      make([]*IPPrefix, 0),
		ExcludeIPv6:      make([]*IPPrefix, 0),
		MinPrefixLenIPv4: 0,
		MinPrefixLenIPv6: 0,
	}
}

func (pa *PrefixAggregator) SetMinPrefixLength(ipv4Len, ipv6Len int) error {
	if ipv4Len < 0 || ipv4Len > 32 {
		return fmt.Errorf("%w: IPv4 length must be 0-32, got %d", ErrInvalidMinPrefixLen, ipv4Len)
	}
	if ipv6Len < 0 || ipv6Len > 128 {
		return fmt.Errorf("%w: IPv6 length must be 0-128, got %d", ErrInvalidMinPrefixLen, ipv6Len)
	}

	pa.mu.Lock()
	defer pa.mu.Unlock()

	pa.MinPrefixLenIPv4 = ipv4Len
	pa.MinPrefixLenIPv6 = ipv6Len
	return nil
}

func (pa *PrefixAggregator) SetIncludePrefixes(prefixes []string) error {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	pa.IncludeIPv4 = pa.IncludeIPv4[:0]
	pa.IncludeIPv6 = pa.IncludeIPv6[:0]

	for _, prefixStr := range prefixes {
		ipPrefix, err := parseIPPrefix(prefixStr)
		if err != nil {
			return fmt.Errorf("failed to parse include prefix %q: %w", prefixStr, err)
		}

		if ipPrefix.Prefix.Addr().Is4() {
			pa.IncludeIPv4 = append(pa.IncludeIPv4, ipPrefix)
		} else {
			pa.IncludeIPv6 = append(pa.IncludeIPv6, ipPrefix)
		}
	}

	return nil
}

func (pa *PrefixAggregator) SetExcludePrefixes(prefixes []string) error {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	pa.ExcludeIPv4 = pa.ExcludeIPv4[:0]
	pa.ExcludeIPv6 = pa.ExcludeIPv6[:0]

	for _, prefixStr := range prefixes {
		ipPrefix, err := parseIPPrefix(prefixStr)
		if err != nil {
			return fmt.Errorf("failed to parse exclude prefix %q: %w", prefixStr, err)
		}

		if ipPrefix.Prefix.Addr().Is4() {
			pa.ExcludeIPv4 = append(pa.ExcludeIPv4, ipPrefix)
		} else {
			pa.ExcludeIPv6 = append(pa.ExcludeIPv6, ipPrefix)
		}
	}

	return nil
}

func (pa *PrefixAggregator) AddPrefix(prefixStr string) error {
	ipPrefix, err := parseIPPrefix(prefixStr)
	if err != nil {
		return fmt.Errorf("failed to parse prefix %q: %w", prefixStr, err)
	}

	pa.mu.Lock()
	defer pa.mu.Unlock()

	if ipPrefix.Prefix.Addr().Is4() {
		pa.IPv4Prefixes = append(pa.IPv4Prefixes, ipPrefix)
	} else {
		pa.IPv6Prefixes = append(pa.IPv6Prefixes, ipPrefix)
	}

	pa.originalCount++
	return nil
}

func (pa *PrefixAggregator) AddPrefixes(prefixes []string) error {
	for _, prefixStr := range prefixes {
		if err := pa.AddPrefix(prefixStr); err != nil {
			return err
		}
	}
	return nil
}

func (pa *PrefixAggregator) AddFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrFileNotFound, path)
		}
		return fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	return pa.AddFromReader(file)
}

func (pa *PrefixAggregator) AddFromReader(reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines, comments, and common header words
		if line == "" || strings.HasPrefix(line, "#") ||
			line == "network" || line == "prefix" || line == "cidr" {
			continue
		}

		// Handle lines that might be missing CIDR notation
		if !strings.Contains(line, "/") {
			// Try to add /32 for IPv4 addresses or /128 for IPv6 addresses
			if strings.Contains(line, ":") {
				line = line + "/128"
			} else if strings.Count(line, ".") == 3 {
				line = line + "/32"
			} else {
				// Skip invalid lines
				continue
			}
		}

		if err := pa.AddPrefix(line); err != nil {
			// Log the error but continue processing (graceful degradation)
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	return nil
}

func (pa *PrefixAggregator) Reset() error {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	// Release all prefixes back to the pool
	for _, p := range pa.IPv4Prefixes {
		releaseIPPrefix(p)
	}
	for _, p := range pa.IPv6Prefixes {
		releaseIPPrefix(p)
	}
	for _, p := range pa.IncludeIPv4 {
		releaseIPPrefix(p)
	}
	for _, p := range pa.IncludeIPv6 {
		releaseIPPrefix(p)
	}
	for _, p := range pa.ExcludeIPv4 {
		releaseIPPrefix(p)
	}
	for _, p := range pa.ExcludeIPv6 {
		releaseIPPrefix(p)
	}

	pa.IPv4Prefixes = pa.IPv4Prefixes[:0]
	pa.IPv6Prefixes = pa.IPv6Prefixes[:0]
	pa.IncludeIPv4 = pa.IncludeIPv4[:0]
	pa.IncludeIPv6 = pa.IncludeIPv6[:0]
	pa.ExcludeIPv4 = pa.ExcludeIPv4[:0]
	pa.ExcludeIPv6 = pa.ExcludeIPv6[:0]
	pa.originalCount = 0
	pa.lastProcessTime = 0
	pa.clearWarnings()

	return nil
}

func (pa *PrefixAggregator) GetPrefixes() []string {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	result := make([]string, 0, len(pa.IPv4Prefixes)+len(pa.IPv6Prefixes))

	for _, prefix := range pa.IPv4Prefixes {
		result = append(result, prefix.Prefix.String())
	}
	for _, prefix := range pa.IPv6Prefixes {
		result = append(result, prefix.Prefix.String())
	}

	return result
}

func (pa *PrefixAggregator) GetIPv4Prefixes() []string {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	result := make([]string, 0, len(pa.IPv4Prefixes))
	for _, prefix := range pa.IPv4Prefixes {
		result = append(result, prefix.Prefix.String())
	}

	return result
}

func (pa *PrefixAggregator) GetIPv6Prefixes() []string {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	result := make([]string, 0, len(pa.IPv6Prefixes))
	for _, prefix := range pa.IPv6Prefixes {
		result = append(result, prefix.Prefix.String())
	}

	return result
}

func (pa *PrefixAggregator) GetStats() AggregationStats {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	ipv4Count := len(pa.IPv4Prefixes)
	ipv6Count := len(pa.IPv6Prefixes)
	totalPrefixes := ipv4Count + ipv6Count

	var reductionRatio float64
	if pa.originalCount > 0 {
		reductionRatio = 1.0 - (float64(totalPrefixes) / float64(pa.originalCount))
	}

	memoryUsage := pa.calculateMemoryUsage()

	return AggregationStats{
		IPv4PrefixCount:  ipv4Count,
		IPv6PrefixCount:  ipv6Count,
		TotalPrefixes:    totalPrefixes,
		OriginalCount:    pa.originalCount,
		ReductionRatio:   reductionRatio,
		ProcessingTimeMs: pa.lastProcessTime.Milliseconds(),
		MemoryUsageBytes: memoryUsage,
	}
}

func (pa *PrefixAggregator) calculateMemoryUsage() int64 {
	var totalMemory int64

	// Size of PrefixAggregator struct itself
	totalMemory += int64(unsafe.Sizeof(*pa))

	// Calculate memory for IPv4 prefixes
	totalMemory += pa.calculatePrefixSliceMemory(pa.IPv4Prefixes)
	totalMemory += pa.calculatePrefixSliceMemory(pa.IPv6Prefixes)
	totalMemory += pa.calculatePrefixSliceMemory(pa.IncludeIPv4)
	totalMemory += pa.calculatePrefixSliceMemory(pa.IncludeIPv6)
	totalMemory += pa.calculatePrefixSliceMemory(pa.ExcludeIPv4)
	totalMemory += pa.calculatePrefixSliceMemory(pa.ExcludeIPv6)

	return totalMemory
}

func (pa *PrefixAggregator) calculatePrefixSliceMemory(prefixes []*IPPrefix) int64 {
	if len(prefixes) == 0 {
		return 0
	}

	// Slice header
	sliceMemory := int64(unsafe.Sizeof(prefixes))

	// Slice backing array (pointers to IPPrefix)
	sliceMemory += int64(cap(prefixes)) * int64(unsafe.Sizeof((*IPPrefix)(nil)))

	// Each IPPrefix struct and its uint256.Int allocations
	for _, prefix := range prefixes {
		if prefix != nil {
			// IPPrefix struct itself
			sliceMemory += int64(unsafe.Sizeof(*prefix))

			// Two uint256.Int structs (Min and Max)
			if prefix.Min != nil {
				sliceMemory += int64(unsafe.Sizeof(*prefix.Min))
			}
			if prefix.Max != nil {
				sliceMemory += int64(unsafe.Sizeof(*prefix.Max))
			}
		}
	}

	return sliceMemory
}

func (pa *PrefixAggregator) GetMemoryStats() MemoryStats {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryStats{
		AllocBytes:      int64(m.Alloc),
		TotalAllocBytes: int64(m.TotalAlloc),
		SysBytes:        int64(m.Sys),
		NumGC:           int64(m.NumGC),
		AggregatorBytes: pa.calculateMemoryUsage(),
	}
}

func (pa *PrefixAggregator) WriteToFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	return pa.WriteToWriter(file)
}

func (pa *PrefixAggregator) WriteToWriter(writer io.Writer) error {
	prefixes := pa.GetPrefixes()

	for _, prefix := range prefixes {
		if _, err := fmt.Fprintf(writer, "%s\n", prefix); err != nil {
			return fmt.Errorf("failed to write prefix %s: %w", prefix, err)
		}
	}

	return nil
}

// SetWarningHandler sets a custom handler for warnings
func (pa *PrefixAggregator) SetWarningHandler(handler func(string)) {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	pa.warningHandler = handler
}

// GetWarnings returns all warnings generated during processing
func (pa *PrefixAggregator) GetWarnings() []string {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	if len(pa.warnings) == 0 {
		return nil
	}

	// Return a copy to prevent external modification
	result := make([]string, len(pa.warnings))
	copy(result, pa.warnings)
	return result
}

// addWarning adds a warning message
func (pa *PrefixAggregator) addWarning(msg string) {
	pa.warnings = append(pa.warnings, msg)

	// Call handler if set
	if pa.warningHandler != nil {
		pa.warningHandler(msg)
	}
}

// clearWarnings clears all warnings
func (pa *PrefixAggregator) clearWarnings() {
	pa.warnings = nil
}
