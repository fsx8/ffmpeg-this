package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func formatDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds() + 0.5)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// dim returns a-b floored at 1, so view dimensions derived from the
// terminal size stay positive on very small terminals.
func dim(a, b int) int {
	if v := a - b; v > 0 {
		return v
	}
	return 1
}

const (
	barFill rune = '█'
	barRest rune = '░'
)

func renderBar(width int, pct float64) string {
	if width < 6 {
		width = 6
	}
	if width > 80 {
		width = 80
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	inner := width - 2
	filled := int(pct*float64(inner) + 0.5)
	if filled < 0 {
		filled = 0
	}
	if filled > inner {
		filled = inner
	}
	return "[" + strings.Repeat(string(barFill), filled) + strings.Repeat(string(barRest), inner-filled) + "]"
}

func parseProbeSeconds(v string) (time.Duration, bool) {
	secs, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || secs <= 0 {
		return 0, false
	}
	return time.Duration(secs * float64(time.Second)), true
}
