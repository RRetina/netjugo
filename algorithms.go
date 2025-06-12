package netjugo

import (
	"fmt"
	"sort"
	"time"

	"github.com/holiman/uint256"
)

func (pa *PrefixAggregator) Aggregate() error {
	start := time.Now()

	pa.mu.Lock()
	defer pa.mu.Unlock()

	// Clear any previous warnings
	pa.clearWarnings()

	// Add include prefixes to main lists
	if err := pa.processInclusions(); err != nil {
		return fmt.Errorf("failed to process inclusions: %w", err)
	}

	// Enforce minimum prefix lengths on all prefixes (including newly added includes)
	if err := pa.enforceMinPrefixLengths(); err != nil {
		return err
	}

	// Sort and deduplicate
	if err := pa.sortAndDeduplicateIPv4(); err != nil {
		return err
	}

	if err := pa.sortAndDeduplicateIPv6(); err != nil {
		return err
	}

	// Initial aggregation
	if err := pa.aggregatePrefixes(&pa.IPv4Prefixes); err != nil {
		return err
	}

	if err := pa.aggregatePrefixes(&pa.IPv6Prefixes); err != nil {
		return err
	}

	// Process exclusions after initial aggregation
	if err := pa.processExclusionsNew(); err != nil {
		return fmt.Errorf("failed to process exclusions: %w", err)
	}

	// Final sort after exclusion processing
	if err := pa.sortAndDeduplicateIPv4(); err != nil {
		return err
	}

	if err := pa.sortAndDeduplicateIPv6(); err != nil {
		return err
	}

	pa.lastProcessTime = time.Since(start)
	return nil
}

func (pa *PrefixAggregator) sortAndDeduplicateIPv4() error {
	if len(pa.IPv4Prefixes) == 0 {
		return nil
	}

	sort.Slice(pa.IPv4Prefixes, func(i, j int) bool {
		return pa.IPv4Prefixes[i].Min.Cmp(pa.IPv4Prefixes[j].Min) < 0
	})

	return pa.deduplicate(&pa.IPv4Prefixes)
}

func (pa *PrefixAggregator) sortAndDeduplicateIPv6() error {
	if len(pa.IPv6Prefixes) == 0 {
		return nil
	}

	sort.Slice(pa.IPv6Prefixes, func(i, j int) bool {
		return pa.IPv6Prefixes[i].Min.Cmp(pa.IPv6Prefixes[j].Min) < 0
	})

	return pa.deduplicate(&pa.IPv6Prefixes)
}

func (pa *PrefixAggregator) deduplicate(prefixes *[]*IPPrefix) error {
	if len(*prefixes) <= 1 {
		return nil
	}

	writeIndex := 0
	for readIndex := 1; readIndex < len(*prefixes); readIndex++ {
		current := (*prefixes)[readIndex]
		last := (*prefixes)[writeIndex]

		if !isDuplicatePrefix(current, last) {
			writeIndex++
			if writeIndex != readIndex {
				(*prefixes)[writeIndex] = current
			}
		}
	}

	*prefixes = (*prefixes)[:writeIndex+1]
	return nil
}

func isDuplicatePrefix(a, b *IPPrefix) bool {
	return a.Min.Cmp(b.Min) == 0 && a.Max.Cmp(b.Max) == 0
}

func (pa *PrefixAggregator) aggregatePrefixes(prefixes *[]*IPPrefix) error {
	if len(*prefixes) <= 1 {
		return nil
	}

	changed := true
	iterations := 0
	maxIterations := 5000 // Safety limit to prevent infinite loops

	for changed && iterations < maxIterations {
		changed = false
		iterations++

		newPrefixes := make([]*IPPrefix, 0, len(*prefixes))
		i := 0

		for i < len(*prefixes) {
			current := (*prefixes)[i]

			if i == len(*prefixes)-1 {
				newPrefixes = append(newPrefixes, current)
				break
			}

			next := (*prefixes)[i+1]

			if contains(current, next) {
				newPrefixes = append(newPrefixes, current)
				i += 2
				changed = true
			} else if contains(next, current) {
				newPrefixes = append(newPrefixes, next)
				i += 2
				changed = true
			} else if areAdjacent(current, next) {
				merged, err := mergeAdjacent(current, next)
				if err == nil {
					newPrefixes = append(newPrefixes, merged)
					i += 2
					changed = true
				} else {
					newPrefixes = append(newPrefixes, current)
					i++
				}
			} else if overlaps(current, next) {
				merged, err := mergeOverlapping(current, next)
				if err == nil {
					newPrefixes = append(newPrefixes, merged)
					i += 2
					changed = true
				} else {
					newPrefixes = append(newPrefixes, current)
					i++
				}
			} else {
				newPrefixes = append(newPrefixes, current)
				i++
			}
		}

		*prefixes = newPrefixes
	}

	if iterations >= maxIterations {
		return fmt.Errorf("aggregation did not converge after %d iterations - possible infinite loop detected", maxIterations)
	}

	return nil
}

func contains(outer, inner *IPPrefix) bool {
	return outer.Min.Cmp(inner.Min) <= 0 && outer.Max.Cmp(inner.Max) >= 0
}

func areAdjacent(a, b *IPPrefix) bool {
	one := uint256.NewInt(1)

	aMaxPlusOne := new(uint256.Int).Add(a.Max, one)
	if aMaxPlusOne.Cmp(b.Min) == 0 {
		return true
	}

	bMaxPlusOne := new(uint256.Int).Add(b.Max, one)
	return bMaxPlusOne.Cmp(a.Min) == 0
}

func overlaps(a, b *IPPrefix) bool {
	return !(a.Max.Cmp(b.Min) < 0 || b.Max.Cmp(a.Min) < 0)
}

func mergeAdjacent(a, b *IPPrefix) (*IPPrefix, error) {
	var minVal, maxVal *uint256.Int

	if a.Min.Cmp(b.Min) <= 0 {
		minVal = new(uint256.Int).Set(a.Min)
	} else {
		minVal = new(uint256.Int).Set(b.Min)
	}

	if a.Max.Cmp(b.Max) >= 0 {
		maxVal = new(uint256.Int).Set(a.Max)
	} else {
		maxVal = new(uint256.Int).Set(b.Max)
	}

	if canMergeToValidPrefix(minVal, maxVal, a.Prefix.Addr().Is4()) {
		prefix, err := uint256RangeToPrefix(minVal, maxVal, a.Prefix.Addr().Is4())
		if err != nil {
			return nil, err
		}

		result := acquireIPPrefix()
		result.Prefix = prefix
		result.Min.Set(minVal)
		result.Max.Set(maxVal)

		return result, nil
	}

	return nil, fmt.Errorf("cannot merge ranges into valid CIDR prefix")
}

func mergeOverlapping(a, b *IPPrefix) (*IPPrefix, error) {
	var minVal, maxVal *uint256.Int

	if a.Min.Cmp(b.Min) <= 0 {
		minVal = new(uint256.Int).Set(a.Min)
	} else {
		minVal = new(uint256.Int).Set(b.Min)
	}

	if a.Max.Cmp(b.Max) >= 0 {
		maxVal = new(uint256.Int).Set(a.Max)
	} else {
		maxVal = new(uint256.Int).Set(b.Max)
	}

	if canMergeToValidPrefix(minVal, maxVal, a.Prefix.Addr().Is4()) {
		prefix, err := uint256RangeToPrefix(minVal, maxVal, a.Prefix.Addr().Is4())
		if err != nil {
			return nil, err
		}

		result := acquireIPPrefix()
		result.Prefix = prefix
		result.Min.Set(minVal)
		result.Max.Set(maxVal)

		return result, nil
	}

	return nil, fmt.Errorf("cannot merge ranges into valid CIDR prefix")
}

func canMergeToValidPrefix(minVal, maxVal *uint256.Int, isIPv4 bool) bool {
	_, err := uint256RangeToPrefix(minVal, maxVal, isIPv4)
	return err == nil
}

func (pa *PrefixAggregator) enforceMinPrefixLengths() error {
	if err := pa.enforceMinPrefixLengthIPv4(); err != nil {
		return err
	}

	if err := pa.enforceMinPrefixLengthIPv6(); err != nil {
		return err
	}

	return nil
}

func (pa *PrefixAggregator) enforceMinPrefixLengthIPv4() error {
	if pa.MinPrefixLenIPv4 == 0 || len(pa.IPv4Prefixes) == 0 {
		return nil
	}

	newPrefixes := make([]*IPPrefix, 0, len(pa.IPv4Prefixes))

	for _, prefix := range pa.IPv4Prefixes {
		if prefix.Prefix.Bits() >= pa.MinPrefixLenIPv4 {
			// Round up to minimum prefix length (make it less specific)
			// This includes prefixes that are EQUAL to or MORE specific than minimum
			rounded, err := roundUpToMinLength(prefix, pa.MinPrefixLenIPv4)
			if err != nil {
				return fmt.Errorf("failed to round up IPv4 prefix %s: %w", prefix.Prefix.String(), err)
			}
			newPrefixes = append(newPrefixes, rounded)
		} else {
			// Prefix is already less specific than minimum
			newPrefixes = append(newPrefixes, prefix)
		}
	}

	pa.IPv4Prefixes = newPrefixes
	return nil
}

func (pa *PrefixAggregator) enforceMinPrefixLengthIPv6() error {
	if pa.MinPrefixLenIPv6 == 0 || len(pa.IPv6Prefixes) == 0 {
		return nil
	}

	newPrefixes := make([]*IPPrefix, 0, len(pa.IPv6Prefixes))

	for _, prefix := range pa.IPv6Prefixes {
		if prefix.Prefix.Bits() >= pa.MinPrefixLenIPv6 {
			// Round up to minimum prefix length (make it less specific)
			// This includes prefixes that are EQUAL to or MORE specific than minimum
			rounded, err := roundUpToMinLength(prefix, pa.MinPrefixLenIPv6)
			if err != nil {
				return fmt.Errorf("failed to round up IPv6 prefix %s: %w", prefix.Prefix.String(), err)
			}
			newPrefixes = append(newPrefixes, rounded)
		} else {
			// Prefix is already less specific than minimum
			newPrefixes = append(newPrefixes, prefix)
		}
	}

	pa.IPv6Prefixes = newPrefixes
	return nil
}

func roundUpToMinLength(prefix *IPPrefix, minLength int) (*IPPrefix, error) {
	currentLength := prefix.Prefix.Bits()

	// If prefix is already at or less specific than minimum, return as is
	if currentLength <= minLength {
		return prefix, nil
	}

	// Need to make the prefix less specific (round up to minLength)
	addr := prefix.Prefix.Addr()

	// Create a new prefix with the minimum length
	newPrefix, err := addr.Prefix(minLength)
	if err != nil {
		return nil, fmt.Errorf("failed to create prefix with length %d: %w", minLength, err)
	}

	// Calculate the new min and max for the rounded prefix
	newMin, newMax, err := prefixToUint256Range(newPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate range for rounded prefix: %w", err)
	}

	result := acquireIPPrefix()
	result.Prefix = newPrefix
	result.Min.Set(newMin)
	result.Max.Set(newMax)

	return result, nil
}
