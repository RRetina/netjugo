package netjugo

import (
	"fmt"
	"net/netip"
	"sort"

	"github.com/holiman/uint256"
)

// Minimum exclusion prefix lengths
// Note: While we allow /32 and /128 for compatibility, it's recommended
// to use larger exclusion prefixes for better performance
const (
	MinExclusionLenIPv4 = 32  // Allow /32 for IPv4 (single IPs)
	MinExclusionLenIPv6 = 128 // Allow /128 for IPv6 (single IPs)

	// RecommendedMinExclusionIPv4 and RecommendedMinExclusionIPv6 prefix lengths for optimal aggregation
	RecommendedMinExclusionIPv4 = 30 // /30 for IPv4
	RecommendedMinExclusionIPv6 = 64 // /64 for IPv6
)

func (pa *PrefixAggregator) processInclusions() error {
	// Add include prefixes to main prefix lists
	pa.IPv4Prefixes = append(pa.IPv4Prefixes, pa.IncludeIPv4...)
	pa.IPv6Prefixes = append(pa.IPv6Prefixes, pa.IncludeIPv6...)

	return nil
}

func (pa *PrefixAggregator) processExclusionsNew() error {
	if err := pa.processExclusionsIPv4New(); err != nil {
		return fmt.Errorf("failed to process IPv4 exclusions: %w", err)
	}

	if err := pa.processExclusionsIPv6New(); err != nil {
		return fmt.Errorf("failed to process IPv6 exclusions: %w", err)
	}

	return nil
}

func (pa *PrefixAggregator) processExclusionsIPv4New() error {
	if len(pa.ExcludeIPv4) == 0 {
		return nil
	}

	for _, excludePrefix := range pa.ExcludeIPv4 {
		// Check minimum exclusion prefix length
		if excludePrefix.Prefix.Bits() > MinExclusionLenIPv4 {
			// Skip exclusions that are too specific
			continue
		}

		// Warn if exclusion is more specific than recommended
		if excludePrefix.Prefix.Bits() > RecommendedMinExclusionIPv4 {
			pa.addWarning(fmt.Sprintf("WARNING: IPv4 exclusion %s is more specific than recommended /%d. This may significantly impact aggregation efficiency.",
				excludePrefix.Prefix.String(), RecommendedMinExclusionIPv4))
		}

		overlapping := pa.findOverlappingPrefixes(excludePrefix, pa.IPv4Prefixes)

		if len(overlapping) == 0 {
			continue
		}

		// Process based on whether exclusion is larger or smaller than overlapping prefixes
		newPrefixes, err := pa.processExclusionNew(excludePrefix, overlapping, true)
		if err != nil {
			return fmt.Errorf("failed to process exclusion %s: %w", excludePrefix.Prefix.String(), err)
		}

		pa.IPv4Prefixes = pa.replacePrefixesInList(pa.IPv4Prefixes, overlapping, newPrefixes)
	}

	return nil
}

func (pa *PrefixAggregator) processExclusionsIPv6New() error {
	if len(pa.ExcludeIPv6) == 0 {
		return nil
	}

	for _, excludePrefix := range pa.ExcludeIPv6 {
		// Check minimum exclusion prefix length
		if excludePrefix.Prefix.Bits() > MinExclusionLenIPv6 {
			// Skip exclusions that are too specific
			continue
		}

		// Warn if exclusion is more specific than recommended
		if excludePrefix.Prefix.Bits() > RecommendedMinExclusionIPv6 {
			pa.addWarning(fmt.Sprintf("WARNING: IPv6 exclusion %s is more specific than recommended /%d. This may significantly impact aggregation efficiency.",
				excludePrefix.Prefix.String(), RecommendedMinExclusionIPv6))
		}

		overlapping := pa.findOverlappingPrefixes(excludePrefix, pa.IPv6Prefixes)

		if len(overlapping) == 0 {
			continue
		}

		// Process based on whether exclusion is larger or smaller than overlapping prefixes
		newPrefixes, err := pa.processExclusionNew(excludePrefix, overlapping, false)
		if err != nil {
			return fmt.Errorf("failed to process exclusion %s: %w", excludePrefix.Prefix.String(), err)
		}

		pa.IPv6Prefixes = pa.replacePrefixesInList(pa.IPv6Prefixes, overlapping, newPrefixes)
	}

	return nil
}

func (pa *PrefixAggregator) processExclusionNew(excludePrefix *IPPrefix, overlappingPrefixes []*IPPrefix, isIPv4 bool) ([]*IPPrefix, error) {
	var result []*IPPrefix

	for _, overlapping := range overlappingPrefixes {
		// Case 1: Exclusion prefix is larger than or equal to overlapping prefix
		// (e.g., exclude 10.0.0.0/8, overlapping is 10.1.0.0/24)
		// Action: Remove the overlapping prefix entirely
		if contains(excludePrefix, overlapping) {
			// Skip this prefix - it's completely excluded
			continue
		}

		// Case 2: Exclusion prefix is smaller than overlapping prefix
		// (e.g., exclude 10.0.0.0/24, overlapping is 10.0.0.0/8)
		// Action: Split the overlapping prefix to create complement
		if contains(overlapping, excludePrefix) {
			// Create the complement of the exclusion within the overlapping prefix
			complement, err := pa.createComplement(overlapping, excludePrefix, isIPv4)
			if err != nil {
				return nil, fmt.Errorf("failed to create complement: %w", err)
			}
			result = append(result, complement...)
		} else if overlaps(excludePrefix, overlapping) {
			// Partial overlap - need to trim
			trimmed, err := pa.trimOverlapNew(overlapping, excludePrefix, isIPv4)
			if err != nil {
				return nil, fmt.Errorf("failed to trim overlap: %w", err)
			}
			result = append(result, trimmed...)
		} else {
			// No overlap - keep the original
			result = append(result, overlapping)
		}
	}

	return result, nil
}

// createComplement creates the optimal set of prefixes representing
// the complement of 'exclude' within 'container'
func (pa *PrefixAggregator) createComplement(container, exclude *IPPrefix, isIPv4 bool) ([]*IPPrefix, error) {
	var result []*IPPrefix

	// We need to create prefixes for:
	// 1. The range before the exclusion (if any)
	// 2. The range after the exclusion (if any)

	// Before exclusion: from container.Min to exclude.Min - 1
	if container.Min.Cmp(exclude.Min) < 0 {
		beforeMax := new(uint256.Int).Sub(exclude.Min, uint256.NewInt(1))
		beforePrefixes, err := pa.createOptimalPrefixes(container.Min, beforeMax, isIPv4)
		if err != nil {
			return nil, fmt.Errorf("failed to create prefixes before exclusion: %w", err)
		}
		result = append(result, beforePrefixes...)
	}

	// After exclusion: from exclude.Max + 1 to container.Max
	if exclude.Max.Cmp(container.Max) < 0 {
		afterMin := new(uint256.Int).Add(exclude.Max, uint256.NewInt(1))
		afterPrefixes, err := pa.createOptimalPrefixes(afterMin, container.Max, isIPv4)
		if err != nil {
			return nil, fmt.Errorf("failed to create prefixes after exclusion: %w", err)
		}
		result = append(result, afterPrefixes...)
	}

	return result, nil
}

// createOptimalPrefixes creates the minimal set of CIDR prefixes that
// cover the range from min to max (inclusive)
func (pa *PrefixAggregator) createOptimalPrefixes(minVal, maxVal *uint256.Int, isIPv4 bool) ([]*IPPrefix, error) {
	var result []*IPPrefix

	// Current position in the range
	current := new(uint256.Int).Set(minVal)

	for current.Cmp(maxVal) <= 0 {
		// Find the largest prefix that:
		// 1. Starts at 'current'
		// 2. Doesn't exceed 'max'
		prefix, prefixMax, err := pa.findLargestValidPrefix(current, maxVal, isIPv4)
		if err != nil {
			return nil, fmt.Errorf("failed to find valid prefix: %w", err)
		}

		result = append(result, prefix)

		// Move to the next position
		current.Add(prefixMax, uint256.NewInt(1))

		// Prevent infinite loop
		if prefixMax.Cmp(maxVal) >= 0 {
			break
		}
	}

	return result, nil
}

// findLargestValidPrefix finds the largest CIDR prefix that starts at 'start'
// and doesn't exceed 'maxAllowed'
func (pa *PrefixAggregator) findLargestValidPrefix(start, maxAllowed *uint256.Int, isIPv4 bool) (*IPPrefix, *uint256.Int, error) {
	// Convert start to IP address
	var addr netip.Addr
	var err error

	if isIPv4 {
		// Extract the lower 32 bits for IPv4
		ipBytes := make([]byte, 4)
		startBytes := start.Bytes32()
		copy(ipBytes, startBytes[28:32])
		addr, _ = netip.AddrFromSlice(ipBytes)
		if !addr.IsValid() {
			return nil, nil, fmt.Errorf("failed to create valid IPv4 address")
		}
	} else {
		// Use all 128 bits for IPv6
		ipBytes := make([]byte, 16)
		startBytes := start.Bytes32()
		copy(ipBytes, startBytes[16:32])
		addr, _ = netip.AddrFromSlice(ipBytes)
		if !addr.IsValid() {
			return nil, nil, fmt.Errorf("failed to create valid IPv6 address")
		}
	}

	// Try different prefix lengths, starting from the most general
	maxBits := 128
	if isIPv4 {
		maxBits = 32
	}

	// Find the number of trailing zeros in start - this determines
	// the maximum prefix length we can use
	trailingZeros := pa.countTrailingZeros(start, isIPv4)

	// Start with the largest possible prefix
	for prefixLen := 0; prefixLen <= maxBits; prefixLen++ {
		// Can't create a prefix more specific than our alignment allows
		if maxBits-prefixLen > trailingZeros {
			continue
		}

		prefix, err := addr.Prefix(prefixLen)
		if err != nil {
			continue
		}

		// Calculate the range this prefix covers
		prefixMin, prefixMax, err := prefixToUint256Range(prefix)
		if err != nil {
			continue
		}

		// Check if this prefix is valid:
		// 1. It must start at our start position
		// 2. It must not exceed maxAllowed
		if prefixMin.Cmp(start) == 0 && prefixMax.Cmp(maxAllowed) <= 0 {
			// This is a valid prefix
			result := acquireIPPrefix()
			result.Prefix = prefix
			result.Min.Set(prefixMin)
			result.Max.Set(prefixMax)
			return result, prefixMax, nil
		}
	}

	// If we can't find a valid prefix, create a host route
	hostBits := maxBits
	prefix, err := addr.Prefix(hostBits)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create host prefix: %w", err)
	}

	result := acquireIPPrefix()
	result.Prefix = prefix
	result.Min.Set(start)
	result.Max.Set(start)
	return result, start, nil
}

// countTrailingZeros counts the number of trailing zero bits
func (pa *PrefixAggregator) countTrailingZeros(n *uint256.Int, isIPv4 bool) int {
	if n.IsZero() {
		if isIPv4 {
			return 32
		}
		return 128
	}

	count := 0
	temp := new(uint256.Int).Set(n)
	one := uint256.NewInt(1)

	maxBits := 128
	if isIPv4 {
		maxBits = 32
	}

	for count < maxBits {
		if new(uint256.Int).And(temp, one).IsZero() {
			count++
			temp.Rsh(temp, 1)
		} else {
			break
		}
	}

	return count
}

// trimOverlapNew handles partial overlaps between exclusion and original prefix
func (pa *PrefixAggregator) trimOverlapNew(original, exclude *IPPrefix, isIPv4 bool) ([]*IPPrefix, error) {
	// Find the non-overlapping part of original
	var resultMin, resultMax *uint256.Int

	// If exclude starts before or at original, trim from the front
	if exclude.Min.Cmp(original.Min) <= 0 && exclude.Max.Cmp(original.Max) < 0 {
		// Keep the right part: from exclude.Max + 1 to original.Max
		resultMin = new(uint256.Int).Add(exclude.Max, uint256.NewInt(1))
		resultMax = new(uint256.Int).Set(original.Max)
	} else if exclude.Min.Cmp(original.Min) > 0 && exclude.Max.Cmp(original.Max) >= 0 {
		// Keep the left part: from original.Min to exclude.Min - 1
		resultMin = new(uint256.Int).Set(original.Min)
		resultMax = new(uint256.Int).Sub(exclude.Min, uint256.NewInt(1))
	} else {
		// This shouldn't happen if we've correctly identified partial overlap
		return []*IPPrefix{}, nil
	}

	if resultMin.Cmp(resultMax) <= 0 {
		return pa.createOptimalPrefixes(resultMin, resultMax, isIPv4)
	}

	return []*IPPrefix{}, nil
}

func (pa *PrefixAggregator) findOverlappingPrefixes(target *IPPrefix, prefixList []*IPPrefix) []*IPPrefix {
	if len(prefixList) == 0 {
		return nil
	}

	// Binary search to find the first prefix that might overlap
	// We look for the rightmost prefix whose Min <= target.Max
	left := 0
	right := len(prefixList) - 1
	firstPossible := -1

	for left <= right {
		mid := left + (right-left)/2
		if prefixList[mid].Min.Cmp(target.Max) <= 0 {
			firstPossible = mid
			left = mid + 1
		} else {
			right = mid - 1
		}
	}

	if firstPossible == -1 {
		// No prefix has Min <= target.Max, so no overlaps possible
		return nil
	}

	// Now scan backwards from firstPossible to find all overlapping prefixes
	var overlapping []*IPPrefix
	for i := firstPossible; i >= 0; i-- {
		prefix := prefixList[i]
		// Stop when we find a prefix whose Max < target.Min (no more overlaps possible)
		if prefix.Max.Cmp(target.Min) < 0 {
			break
		}
		if overlaps(target, prefix) {
			overlapping = append(overlapping, prefix)
		}
	}

	// Reverse the slice to maintain original order
	for i := 0; i < len(overlapping)/2; i++ {
		j := len(overlapping) - 1 - i
		overlapping[i], overlapping[j] = overlapping[j], overlapping[i]
	}

	return overlapping
}

func (pa *PrefixAggregator) replacePrefixesInList(originalList []*IPPrefix, toReplace []*IPPrefix, newPrefixes []*IPPrefix) []*IPPrefix {
	// Create a set of prefixes to remove for efficient lookup
	toRemove := make(map[*IPPrefix]bool)
	for _, prefix := range toReplace {
		toRemove[prefix] = true
	}

	var result []*IPPrefix

	// Add prefixes that are not being replaced
	for _, prefix := range originalList {
		if !toRemove[prefix] {
			result = append(result, prefix)
		}
	}

	// Add new prefixes
	result = append(result, newPrefixes...)

	// Sort to maintain order
	sort.Slice(result, func(i, j int) bool {
		return result[i].Min.Cmp(result[j].Min) < 0
	})

	return result
}
