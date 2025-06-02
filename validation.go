package netjugo

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/holiman/uint256"
)

func parseIPPrefix(prefixStr string) (*IPPrefix, error) {
	prefixStr = strings.TrimSpace(prefixStr)
	if prefixStr == "" {
		return nil, fmt.Errorf("%w: empty prefix string", ErrInvalidPrefix)
	}

	// First try to parse as a prefix
	prefix, err := netip.ParsePrefix(prefixStr)
	if err != nil {
		// If that fails, try to parse as a bare IP address
		addr, addrErr := netip.ParseAddr(prefixStr)
		if addrErr != nil {
			// Neither worked, return the original prefix parsing error
			return nil, fmt.Errorf("%w: failed to parse %q: %v", ErrInvalidPrefix, prefixStr, err)
		}

		// Successfully parsed as IP address, convert to host prefix
		if addr.Is4() {
			prefix = netip.PrefixFrom(addr, 32)
		} else if addr.Is6() {
			prefix = netip.PrefixFrom(addr, 128)
		} else {
			return nil, fmt.Errorf("%w: unsupported address type for %q", ErrInvalidPrefix, prefixStr)
		}
	}

	if !prefix.IsValid() {
		return nil, fmt.Errorf("%w: invalid prefix %q", ErrInvalidPrefix, prefixStr)
	}

	min, max, err := prefixToUint256Range(prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to convert prefix to uint256 range: %w", err)
	}

	ipPrefix := acquireIPPrefix()
	ipPrefix.Prefix = prefix
	ipPrefix.Min.Set(min)
	ipPrefix.Max.Set(max)

	return ipPrefix, nil
}

func prefixToUint256Range(prefix netip.Prefix) (*uint256.Int, *uint256.Int, error) {
	addr := prefix.Addr()
	bits := prefix.Bits()

	if addr.Is4() {
		return ipv4PrefixToUint256Range(addr, bits)
	} else if addr.Is6() {
		return ipv6PrefixToUint256Range(addr, bits)
	}

	return nil, nil, fmt.Errorf("%w: unsupported address type", ErrUnsupportedIPVersion)
}

func ipv4PrefixToUint256Range(addr netip.Addr, bits int) (*uint256.Int, *uint256.Int, error) {
	if bits < 0 || bits > 32 {
		return nil, nil, fmt.Errorf("%w: IPv4 prefix length must be 0-32, got %d", ErrInvalidPrefix, bits)
	}

	ipv4Bytes := addr.As4()

	minAddr := uint256.NewInt(0)
	minAddr.SetBytes(ipv4Bytes[:])

	if bits == 32 {
		return minAddr, new(uint256.Int).Set(minAddr), nil
	}

	hostBits := 32 - bits
	mask := uint32(0xFFFFFFFF) << hostBits

	minAddrVal := (uint32(ipv4Bytes[0])<<24 | uint32(ipv4Bytes[1])<<16 | uint32(ipv4Bytes[2])<<8 | uint32(ipv4Bytes[3])) & mask
	maxAddrVal := minAddrVal | ((1 << hostBits) - 1)

	minAddr = uint256.NewInt(uint64(minAddrVal))
	maxAddr := uint256.NewInt(uint64(maxAddrVal))

	return minAddr, maxAddr, nil
}

func ipv6PrefixToUint256Range(addr netip.Addr, bits int) (*uint256.Int, *uint256.Int, error) {
	if bits < 0 || bits > 128 {
		return nil, nil, fmt.Errorf("%w: IPv6 prefix length must be 0-128, got %d", ErrInvalidPrefix, bits)
	}

	ipv6Bytes := addr.As16()

	minAddr := uint256.NewInt(0)
	minAddr.SetBytes(ipv6Bytes[:])

	if bits == 128 {
		return minAddr, new(uint256.Int).Set(minAddr), nil
	}

	hostBits := 128 - bits

	maxAddr := new(uint256.Int).Set(minAddr)

	if hostBits >= 64 {
		ones := new(uint256.Int).SetUint64(0xFFFFFFFFFFFFFFFF)
		maxAddr.Or(maxAddr, ones)

		if hostBits > 64 {
			shift := hostBits - 64
			highOnes := new(uint256.Int).SetUint64((1 << shift) - 1)
			highOnes.Lsh(highOnes, 64)
			maxAddr.Or(maxAddr, highOnes)
		}
	} else {
		ones := new(uint256.Int).SetUint64((1 << hostBits) - 1)
		maxAddr.Or(maxAddr, ones)
	}

	bytesToClear := hostBits / 8
	if bytesToClear > 0 {
		mask := make([]byte, 16)
		for i := 0; i < 16-bytesToClear; i++ {
			mask[i] = 0xFF
		}

		maskInt := uint256.NewInt(0)
		maskInt.SetBytes(mask)
		minAddr.And(minAddr, maskInt)
	}

	if hostBits%8 != 0 {
		byteIndex := 15 - (hostBits / 8)
		bitMask := uint8(0xFF) << (hostBits % 8)

		minBytes := minAddr.Bytes()
		if len(minBytes) > byteIndex {
			minBytes[byteIndex] &= bitMask
		}
		minAddr.SetBytes(minBytes)
	}

	return minAddr, maxAddr, nil
}

func validatePrefixLength(isIPv4 bool, bits int) error {
	if isIPv4 {
		if bits < 0 || bits > 32 {
			return fmt.Errorf("%w: IPv4 prefix length must be 0-32, got %d", ErrInvalidPrefix, bits)
		}
	} else {
		if bits < 0 || bits > 128 {
			return fmt.Errorf("%w: IPv6 prefix length must be 0-128, got %d", ErrInvalidPrefix, bits)
		}
	}
	return nil
}

func isValidIPPrefix(prefixStr string) bool {
	_, err := parseIPPrefix(prefixStr)
	return err == nil
}

func uint256RangeToPrefix(min, max *uint256.Int, isIPv4 bool) (netip.Prefix, error) {
	if isIPv4 {
		return uint256RangeToIPv4Prefix(min, max)
	}
	return uint256RangeToIPv6Prefix(min, max)
}

func uint256RangeToIPv4Prefix(min, max *uint256.Int) (netip.Prefix, error) {
	if min.Cmp(max) > 0 {
		return netip.Prefix{}, fmt.Errorf("%w: min > max in range", ErrInvalidPrefix)
	}

	minVal := min.Uint64()
	maxVal := max.Uint64()

	if minVal > 0xFFFFFFFF || maxVal > 0xFFFFFFFF {
		return netip.Prefix{}, fmt.Errorf("%w: IPv4 address out of range", ErrInvalidPrefix)
	}

	if minVal == maxVal {
		minBytes := [4]byte{
			byte(minVal >> 24),
			byte(minVal >> 16),
			byte(minVal >> 8),
			byte(minVal),
		}
		addr := netip.AddrFrom4(minBytes)
		return netip.PrefixFrom(addr, 32), nil
	}

	rangeSize := maxVal - minVal + 1

	if (rangeSize & (rangeSize - 1)) != 0 {
		return netip.Prefix{}, fmt.Errorf("%w: range is not a power of 2", ErrInvalidPrefix)
	}

	prefixBits := 32
	for rangeSize > 1 {
		rangeSize >>= 1
		prefixBits--
	}

	networkAddr := minVal
	mask := uint32(0xFFFFFFFF) << (32 - prefixBits)
	if (networkAddr & uint64(^mask)) != 0 {
		return netip.Prefix{}, fmt.Errorf("%w: range not aligned to prefix boundary", ErrInvalidPrefix)
	}

	addrBytes := [4]byte{
		byte(networkAddr >> 24),
		byte(networkAddr >> 16),
		byte(networkAddr >> 8),
		byte(networkAddr),
	}
	addr := netip.AddrFrom4(addrBytes)

	return netip.PrefixFrom(addr, prefixBits), nil
}

func uint256RangeToIPv6Prefix(min, max *uint256.Int) (netip.Prefix, error) {
	if min.Cmp(max) > 0 {
		return netip.Prefix{}, fmt.Errorf("%w: min > max in range", ErrInvalidPrefix)
	}

	if min.Cmp(max) == 0 {
		minBytes := make([]byte, 32)
		min.WriteToSlice(minBytes)

		var addr16 [16]byte
		copy(addr16[:], minBytes[len(minBytes)-16:])
		addr := netip.AddrFrom16(addr16)
		return netip.PrefixFrom(addr, 128), nil
	}

	diff := new(uint256.Int).Sub(max, min)
	diff.Add(diff, uint256.NewInt(1))

	if !isPowerOfTwo(diff) {
		return netip.Prefix{}, fmt.Errorf("%w: range is not a power of 2", ErrInvalidPrefix)
	}

	prefixBits := 128
	temp := new(uint256.Int).Set(diff)
	for temp.Cmp(uint256.NewInt(1)) > 0 {
		temp.Rsh(temp, 1)
		prefixBits--
	}

	hostBits := 128 - prefixBits
	if hostBits > 0 {
		mask := new(uint256.Int).Lsh(uint256.NewInt(1), uint(hostBits))
		mask.Sub(mask, uint256.NewInt(1))
		remainder := new(uint256.Int).And(min, mask)
		if !remainder.IsZero() {
			return netip.Prefix{}, fmt.Errorf("%w: range not aligned to prefix boundary", ErrInvalidPrefix)
		}
	}

	minBytes := make([]byte, 32)
	min.WriteToSlice(minBytes)

	var addr16 [16]byte
	copy(addr16[:], minBytes[len(minBytes)-16:])
	addr := netip.AddrFrom16(addr16)

	return netip.PrefixFrom(addr, prefixBits), nil
}

func isPowerOfTwo(n *uint256.Int) bool {
	if n.IsZero() {
		return false
	}
	temp := new(uint256.Int).Sub(n, uint256.NewInt(1))
	temp.And(n, temp)
	return temp.IsZero()
}
