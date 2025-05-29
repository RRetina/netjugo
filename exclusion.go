package netjugo

import (
	"fmt"
	"sort"

	"github.com/holiman/uint256"
)

func (pa *PrefixAggregator) processInclusions() error {
	// Add include prefixes to main prefix lists
	for _, includePrefix := range pa.IncludeIPv4 {
		pa.IPv4Prefixes = append(pa.IPv4Prefixes, includePrefix)
	}

	for _, includePrefix := range pa.IncludeIPv6 {
		pa.IPv6Prefixes = append(pa.IPv6Prefixes, includePrefix)
	}

	return nil
}

func (pa *PrefixAggregator) processExclusions() error {
	if err := pa.processExclusionsIPv4(); err != nil {
		return fmt.Errorf("failed to process IPv4 exclusions: %w", err)
	}

	if err := pa.processExclusionsIPv6(); err != nil {
		return fmt.Errorf("failed to process IPv6 exclusions: %w", err)
	}

	return nil
}

func (pa *PrefixAggregator) processExclusionsIPv4() error {
	if len(pa.ExcludeIPv4) == 0 {
		return nil
	}

	for _, excludePrefix := range pa.ExcludeIPv4 {
		overlapping := pa.findOverlappingPrefixes(excludePrefix, pa.IPv4Prefixes)

		if len(overlapping) == 0 {
			continue
		}

		newPrefixes, err := pa.subtractPrefixFromList(excludePrefix, overlapping, true)
		if err != nil {
			return fmt.Errorf("failed to subtract prefix %s: %w", excludePrefix.Prefix.String(), err)
		}

		pa.IPv4Prefixes = pa.replacePrefixesInList(pa.IPv4Prefixes, overlapping, newPrefixes)
	}

	return nil
}

func (pa *PrefixAggregator) processExclusionsIPv6() error {
	if len(pa.ExcludeIPv6) == 0 {
		return nil
	}

	for _, excludePrefix := range pa.ExcludeIPv6 {
		overlapping := pa.findOverlappingPrefixes(excludePrefix, pa.IPv6Prefixes)

		if len(overlapping) == 0 {
			continue
		}

		newPrefixes, err := pa.subtractPrefixFromList(excludePrefix, overlapping, false)
		if err != nil {
			return fmt.Errorf("failed to subtract prefix %s: %w", excludePrefix.Prefix.String(), err)
		}

		pa.IPv6Prefixes = pa.replacePrefixesInList(pa.IPv6Prefixes, overlapping, newPrefixes)
	}

	return nil
}

func (pa *PrefixAggregator) findOverlappingPrefixes(target *IPPrefix, prefixList []*IPPrefix) []*IPPrefix {
	var overlapping []*IPPrefix

	for _, prefix := range prefixList {
		if overlaps(target, prefix) {
			overlapping = append(overlapping, prefix)
		}
	}

	return overlapping
}

func (pa *PrefixAggregator) subtractPrefixFromList(excludePrefix *IPPrefix, overlappingPrefixes []*IPPrefix, isIPv4 bool) ([]*IPPrefix, error) {
	var result []*IPPrefix

	for _, overlapping := range overlappingPrefixes {
		subtracted, err := pa.subtractPrefix(excludePrefix, overlapping, isIPv4)
		if err != nil {
			return nil, err
		}
		result = append(result, subtracted...)
	}

	return result, nil
}

func (pa *PrefixAggregator) subtractPrefix(exclude, original *IPPrefix, isIPv4 bool) ([]*IPPrefix, error) {
	// If exclude completely contains original, return empty (original is completely excluded)
	if contains(exclude, original) {
		return []*IPPrefix{}, nil
	}

	// If exclude doesn't overlap with original, return original unchanged
	if !overlaps(exclude, original) {
		return []*IPPrefix{original}, nil
	}

	// If original completely contains exclude, we need to split original around exclude
	if contains(original, exclude) {
		return pa.splitAroundExclusion(original, exclude, isIPv4)
	}

	// Partial overlap - trim the overlapping part
	return pa.trimOverlap(original, exclude, isIPv4)
}

func (pa *PrefixAggregator) splitAroundExclusion(original, exclude *IPPrefix, isIPv4 bool) ([]*IPPrefix, error) {
	var result []*IPPrefix

	// Left part: from original.Min to exclude.Min - 1
	if exclude.Min.Cmp(original.Min) > 0 {
		leftMax := new(uint256.Int).Sub(exclude.Min, uint256.NewInt(1))
		if original.Min.Cmp(leftMax) <= 0 {
			leftPrefixes, err := pa.rangeToPrefixes(original.Min, leftMax, isIPv4)
			if err != nil {
				return nil, fmt.Errorf("failed to create left part: %w", err)
			}
			result = append(result, leftPrefixes...)
		}
	}

	// Right part: from exclude.Max + 1 to original.Max
	if exclude.Max.Cmp(original.Max) < 0 {
		rightMin := new(uint256.Int).Add(exclude.Max, uint256.NewInt(1))
		if rightMin.Cmp(original.Max) <= 0 {
			rightPrefixes, err := pa.rangeToPrefixes(rightMin, original.Max, isIPv4)
			if err != nil {
				return nil, fmt.Errorf("failed to create right part: %w", err)
			}
			result = append(result, rightPrefixes...)
		}
	}

	return result, nil
}

func (pa *PrefixAggregator) trimOverlap(original, exclude *IPPrefix, isIPv4 bool) ([]*IPPrefix, error) {
	// Find the non-overlapping part of original
	var resultMin, resultMax *uint256.Int

	// If exclude starts before original, trim from the front
	if exclude.Min.Cmp(original.Min) <= 0 && exclude.Max.Cmp(original.Max) < 0 {
		// Keep the right part: from exclude.Max + 1 to original.Max
		resultMin = new(uint256.Int).Add(exclude.Max, uint256.NewInt(1))
		resultMax = new(uint256.Int).Set(original.Max)
	} else if exclude.Min.Cmp(original.Min) > 0 && exclude.Max.Cmp(original.Max) >= 0 {
		// Keep the left part: from original.Min to exclude.Min - 1
		resultMin = new(uint256.Int).Set(original.Min)
		resultMax = new(uint256.Int).Sub(exclude.Min, uint256.NewInt(1))
	} else {
		// This case should be handled by splitAroundExclusion
		return []*IPPrefix{}, nil
	}

	if resultMin.Cmp(resultMax) <= 0 {
		return pa.rangeToPrefixes(resultMin, resultMax, isIPv4)
	}

	return []*IPPrefix{}, nil
}

func (pa *PrefixAggregator) rangeToPrefixes(min, max *uint256.Int, isIPv4 bool) ([]*IPPrefix, error) {
	// Try to create a single prefix first
	prefix, err := uint256RangeToPrefix(min, max, isIPv4)
	if err == nil {
		prefixMin, prefixMax, err := prefixToUint256Range(prefix)
		if err != nil {
			return nil, err
		}
		return []*IPPrefix{{
			Prefix: prefix,
			Min:    prefixMin,
			Max:    prefixMax,
		}}, nil
	}

	// If single prefix doesn't work, split recursively
	if min.Cmp(max) == 0 {
		// Single address
		prefix, err := uint256RangeToPrefix(min, max, isIPv4)
		if err != nil {
			return nil, err
		}

		prefixMin, prefixMax, err := prefixToUint256Range(prefix)
		if err != nil {
			return nil, err
		}

		return []*IPPrefix{{
			Prefix: prefix,
			Min:    prefixMin,
			Max:    prefixMax,
		}}, nil
	}

	// Split range in half and recursively process each half
	mid := new(uint256.Int).Add(min, max)
	mid.Rsh(mid, 1)

	var result []*IPPrefix

	// Left half
	if min.Cmp(mid) <= 0 {
		leftPrefixes, err := pa.rangeToPrefixes(min, mid, isIPv4)
		if err != nil {
			return nil, err
		}
		result = append(result, leftPrefixes...)
	}

	// Right half
	midPlusOne := new(uint256.Int).Add(mid, uint256.NewInt(1))
	if midPlusOne.Cmp(max) <= 0 {
		rightPrefixes, err := pa.rangeToPrefixes(midPlusOne, max, isIPv4)
		if err != nil {
			return nil, err
		}
		result = append(result, rightPrefixes...)
	}

	return result, nil
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
