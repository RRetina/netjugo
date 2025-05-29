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
	if err := pa.processExclusions(); err != nil {
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

	isIPv4 := len(*prefixes) > 0 && (*prefixes)[0].Prefix.Addr().Is4()
	minLength := pa.MinPrefixLenIPv6
	if isIPv4 {
		minLength = pa.MinPrefixLenIPv4
	}

	changed := true
	for changed {
		changed = false

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
				if err == nil && (minLength == 0 || merged.Prefix.Bits() >= minLength) {
					newPrefixes = append(newPrefixes, merged)
					i += 2
					changed = true
				} else {
					newPrefixes = append(newPrefixes, current)
					i++
				}
			} else if overlaps(current, next) {
				merged, err := mergeOverlapping(current, next)
				if err == nil && (minLength == 0 || merged.Prefix.Bits() >= minLength) {
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
	if bMaxPlusOne.Cmp(a.Min) == 0 {
		return true
	}

	return false
}

func overlaps(a, b *IPPrefix) bool {
	return !(a.Max.Cmp(b.Min) < 0 || b.Max.Cmp(a.Min) < 0)
}

func mergeAdjacent(a, b *IPPrefix) (*IPPrefix, error) {
	var min, max *uint256.Int

	if a.Min.Cmp(b.Min) <= 0 {
		min = new(uint256.Int).Set(a.Min)
	} else {
		min = new(uint256.Int).Set(b.Min)
	}

	if a.Max.Cmp(b.Max) >= 0 {
		max = new(uint256.Int).Set(a.Max)
	} else {
		max = new(uint256.Int).Set(b.Max)
	}

	if canMergeToValidPrefix(min, max, a.Prefix.Addr().Is4()) {
		prefix, err := uint256RangeToPrefix(min, max, a.Prefix.Addr().Is4())
		if err != nil {
			return nil, err
		}

		return &IPPrefix{
			Prefix: prefix,
			Min:    min,
			Max:    max,
		}, nil
	}

	return nil, fmt.Errorf("cannot merge ranges into valid CIDR prefix")
}

func mergeOverlapping(a, b *IPPrefix) (*IPPrefix, error) {
	var min, max *uint256.Int

	if a.Min.Cmp(b.Min) <= 0 {
		min = new(uint256.Int).Set(a.Min)
	} else {
		min = new(uint256.Int).Set(b.Min)
	}

	if a.Max.Cmp(b.Max) >= 0 {
		max = new(uint256.Int).Set(a.Max)
	} else {
		max = new(uint256.Int).Set(b.Max)
	}

	if canMergeToValidPrefix(min, max, a.Prefix.Addr().Is4()) {
		prefix, err := uint256RangeToPrefix(min, max, a.Prefix.Addr().Is4())
		if err != nil {
			return nil, err
		}

		return &IPPrefix{
			Prefix: prefix,
			Min:    min,
			Max:    max,
		}, nil
	}

	return nil, fmt.Errorf("cannot merge ranges into valid CIDR prefix")
}

func canMergeToValidPrefix(min, max *uint256.Int, isIPv4 bool) bool {
	_, err := uint256RangeToPrefix(min, max, isIPv4)
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
		if prefix.Prefix.Bits() < pa.MinPrefixLenIPv4 {
			split, err := splitPrefixToMinLength(prefix, pa.MinPrefixLenIPv4, true)
			if err != nil {
				return fmt.Errorf("failed to split IPv4 prefix %s: %w", prefix.Prefix.String(), err)
			}
			newPrefixes = append(newPrefixes, split...)
		} else {
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
		if prefix.Prefix.Bits() < pa.MinPrefixLenIPv6 {
			split, err := splitPrefixToMinLength(prefix, pa.MinPrefixLenIPv6, false)
			if err != nil {
				return fmt.Errorf("failed to split IPv6 prefix %s: %w", prefix.Prefix.String(), err)
			}
			newPrefixes = append(newPrefixes, split...)
		} else {
			newPrefixes = append(newPrefixes, prefix)
		}
	}

	pa.IPv6Prefixes = newPrefixes
	return nil
}

func splitPrefixToMinLength(prefix *IPPrefix, minLength int, isIPv4 bool) ([]*IPPrefix, error) {
	currentLength := prefix.Prefix.Bits()

	if currentLength >= minLength {
		return []*IPPrefix{prefix}, nil
	}

	maxBits := 32
	if !isIPv4 {
		maxBits = 128
	}

	if minLength > maxBits {
		return nil, fmt.Errorf("%w: minimum length %d exceeds maximum for IP version", ErrInvalidMinPrefixLen, minLength)
	}

	return binarySplitPrefix(prefix, minLength, isIPv4)
}

func binarySplitPrefix(prefix *IPPrefix, targetLength int, isIPv4 bool) ([]*IPPrefix, error) {
	if prefix.Prefix.Bits() >= targetLength {
		return []*IPPrefix{prefix}, nil
	}

	result := []*IPPrefix{prefix}

	for len(result) > 0 && result[0].Prefix.Bits() < targetLength {
		var newResult []*IPPrefix

		for _, p := range result {
			if p.Prefix.Bits() >= targetLength {
				newResult = append(newResult, p)
			} else {
				split, err := splitPrefixInHalf(p, isIPv4)
				if err != nil {
					return nil, err
				}
				newResult = append(newResult, split...)
			}
		}

		result = newResult
	}

	return result, nil
}

func splitPrefixInHalf(prefix *IPPrefix, isIPv4 bool) ([]*IPPrefix, error) {
	currentBits := prefix.Prefix.Bits()
	maxBits := 32
	if !isIPv4 {
		maxBits = 128
	}

	if currentBits >= maxBits {
		return []*IPPrefix{prefix}, nil
	}

	mid := new(uint256.Int).Add(prefix.Min, prefix.Max)
	mid.Add(mid, uint256.NewInt(1))
	mid.Rsh(mid, 1)

	maxFirst := new(uint256.Int).Sub(mid, uint256.NewInt(1))

	firstPrefix, err := uint256RangeToPrefix(prefix.Min, maxFirst, isIPv4)
	if err != nil {
		return nil, fmt.Errorf("failed to create first half prefix: %w", err)
	}

	secondPrefix, err := uint256RangeToPrefix(mid, prefix.Max, isIPv4)
	if err != nil {
		return nil, fmt.Errorf("failed to create second half prefix: %w", err)
	}

	firstMin, firstMax, err := prefixToUint256Range(firstPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to convert first prefix to range: %w", err)
	}

	secondMin, secondMax, err := prefixToUint256Range(secondPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to convert second prefix to range: %w", err)
	}

	return []*IPPrefix{
		{
			Prefix: firstPrefix,
			Min:    firstMin,
			Max:    firstMax,
		},
		{
			Prefix: secondPrefix,
			Min:    secondMin,
			Max:    secondMax,
		},
	}, nil
}
