package mikrotik

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseMikrotikDuration(durationStr string) (time.Duration, error) {
	if durationStr == "" {
		return 0, errors.New("empty duration string")
	}

	var totalDuration time.Duration
	var currentVal strings.Builder

	for _, r := range durationStr {
		if (r >= '0' && r <= '9') || r == '.' {
			currentVal.WriteRune(r)
			continue
		}
		unit := r
		valStr := currentVal.String()
		if valStr == "" {
			return 0, fmt.Errorf("invalid duration format near unit '%c' in '%s'", unit, durationStr)
		}

		if unit == 's' {
			fVal, fErr := strconv.ParseFloat(valStr, 64)
			if fErr == nil {
				totalDuration += time.Duration(fVal * float64(time.Second))
				currentVal.Reset()
				continue
			}
		}

		val, err := strconv.ParseInt(valStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("could not parse value '%s' in duration '%s': %w", valStr, durationStr, err)
		}

		switch unit {
		case 'w':
			totalDuration += time.Duration(val) * 7 * 24 * time.Hour
		case 'd':
			totalDuration += time.Duration(val) * 24 * time.Hour
		case 'h':
			totalDuration += time.Duration(val) * time.Hour
		case 'm':
			totalDuration += time.Duration(val) * time.Minute
		case 's':
			totalDuration += time.Duration(val) * time.Second
		default:
			return 0, fmt.Errorf("unknown duration unit '%c' in '%s'", unit, durationStr)
		}
		currentVal.Reset()
	}
	if currentVal.Len() > 0 {
		return 0, fmt.Errorf("trailing number without unit in duration '%s'", durationStr)
	}

	return totalDuration, nil
}

func parseBytes(byteStr string) (uint64, error) {
	if byteStr == "" {
		return 0, errors.New("empty byte string")
	}
	bytes, err := strconv.ParseUint(byteStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("could not parse byte value '%s': %w", byteStr, err)
	}
	return bytes, nil
}

func parseBool(boolStr string) bool {
	return strings.ToLower(boolStr) == "true"
}

// ParseRateBps parses RouterOS rate strings such as "54Mbps", "1Gbps", "150000000", "6Mbps-40Mhz/1S/SGI".
func ParseRateBps(rateStr string) (float64, error) {
	rateStr = strings.TrimSpace(rateStr)
	if rateStr == "" {
		return 0, errors.New("empty rate string")
	}

	// Take the leading token before channel/stream suffixes.
	if idx := strings.IndexAny(rateStr, "-/"); idx >= 0 {
		rateStr = rateStr[:idx]
	}
	rateStr = strings.TrimSpace(rateStr)

	lower := strings.ToLower(rateStr)
	multiplier := 1.0
	numPart := rateStr

	switch {
	case strings.HasSuffix(lower, "gbps"):
		multiplier = 1e9
		numPart = rateStr[:len(rateStr)-4]
	case strings.HasSuffix(lower, "mbps"):
		multiplier = 1e6
		numPart = rateStr[:len(rateStr)-4]
	case strings.HasSuffix(lower, "kbps"):
		multiplier = 1e3
		numPart = rateStr[:len(rateStr)-4]
	case strings.HasSuffix(lower, "bps"):
		multiplier = 1
		numPart = rateStr[:len(rateStr)-3]
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(numPart), 64)
	if err != nil {
		return 0, fmt.Errorf("could not parse rate '%s': %w", rateStr, err)
	}
	return val * multiplier, nil
}

// ParseLinkSpeedBps parses link speeds such as "1Gbps", "100Mbps", "10M", "1G".
// Pure numeric values are treated as bits per second.
func ParseLinkSpeedBps(speedStr string) (uint64, error) {
	speedStr = strings.TrimSpace(speedStr)
	if speedStr == "" {
		return 0, errors.New("empty speed string")
	}

	// Numeric only: interpret as raw bps.
	if _, err := strconv.ParseUint(speedStr, 10, 64); err == nil {
		bps, err := ParseRateBps(speedStr + "bps")
		return uint64(bps), err
	}

	// Normalize short forms: 1G, 100M, 10Mbit
	lower := strings.ToLower(speedStr)
	lower = strings.TrimSuffix(lower, "bit")
	lower = strings.TrimSuffix(lower, "its")
	switch {
	case strings.HasSuffix(lower, "gbps") || strings.HasSuffix(lower, "g"):
		num := strings.TrimSuffix(strings.TrimSuffix(lower, "gbps"), "g")
		v, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return 0, err
		}
		return uint64(v * 1e9), nil
	case strings.HasSuffix(lower, "mbps") || strings.HasSuffix(lower, "m"):
		num := strings.TrimSuffix(strings.TrimSuffix(lower, "mbps"), "m")
		v, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return 0, err
		}
		return uint64(v * 1e6), nil
	default:
		bps, err := ParseRateBps(speedStr)
		return uint64(bps), err
	}
}

func firstNonEmpty(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != "" {
			return v
		}
	}
	return ""
}

func isUnsupportedCommand(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such command") ||
		strings.Contains(msg, "unknown command") ||
		strings.Contains(msg, "disabled") ||
		strings.Contains(msg, "not found")
}
