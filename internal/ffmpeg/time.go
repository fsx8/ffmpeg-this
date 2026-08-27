package ffmpeg

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseTimeSpec parses an ffmpeg-style timestamp: either plain seconds
// ("90", "90.5") or up to HH:MM:SS(.frac) with any trailing components
// omitted ("1:30", "01:00:10.5").
func ParseTimeSpec(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty time value")
	}
	parts := strings.Split(s, ":")
	if len(parts) > 3 {
		return 0, fmt.Errorf("invalid time %q (expected [HH:]MM:SS or seconds)", s)
	}

	var total float64
	for i, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("invalid time %q: %q is not a number", s, p)
		}
		// Only the final component (seconds) may be fractional.
		if i < len(parts)-1 {
			if v != float64(int64(v)) || int64(v) > 59 {
				return 0, fmt.Errorf("invalid time %q: %q must be an integer below 60", s, p)
			}
		}
		total = total*60 + v
	}
	return total, nil
}

// ValidateTrim checks that both timestamps parse and that the cut is
// non-empty (start strictly before end).
func ValidateTrim(start, end string) error {
	startSec, err := ParseTimeSpec(start)
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	endSec, err := ParseTimeSpec(end)
	if err != nil {
		return fmt.Errorf("end: %w", err)
	}
	if startSec >= endSec {
		return fmt.Errorf("start (%s) must be before end (%s)", start, end)
	}
	return nil
}
