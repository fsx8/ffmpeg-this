package ffmpeg

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// AddProgressArgs prepends the machine-readable progress flags so callers can
// parse completion updates from stdout while stderr keeps its usual log text.
// The flags are global options; every generated command tolerates them first.
func AddProgressArgs(args []string) []string {
	out := make([]string, 0, len(args)+3)
	out = append(out, "-nostats", "-progress", "pipe:1")
	return append(out, args...)
}

// ProgressSample is one observation from ffmpeg's `-progress` key=value stream.
// Individual lines carry a subset of the fields.
type ProgressSample struct {
	Frame   int
	FPS     float64
	OutTime time.Duration // processed output time so far
	Speed   float64       // encoding speed multiplier, e.g. 2.5 means 2.5x realtime
	Done    bool          // progress=end marker seen
}

func ParseProgressLine(line string) (ProgressSample, bool) {
	line = strings.TrimSpace(line)
	k, v, found := strings.Cut(line, "=")
	if !found {
		return ProgressSample{}, false
	}
	v = strings.TrimSpace(v)

	var s ProgressSample
	switch k {
	case "frame":
		n, err := strconv.Atoi(v)
		if err != nil {
			return ProgressSample{}, false
		}
		s.Frame = n
	case "fps":
		f, ok := parseFiniteFloat(v)
		if !ok || f < 0 {
			return ProgressSample{}, false
		}
		s.FPS = f
	case "out_time_us", "out_time_ms":
		us, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return ProgressSample{}, false
		}
		if us < 0 {
			return ProgressSample{}, false
		}
		s.OutTime = time.Duration(us) * time.Microsecond
	case "out_time":
		d, ok := parseClock(v)
		if !ok {
			return ProgressSample{}, false
		}
		s.OutTime = d
	case "speed":
		f, ok := parseFiniteFloat(strings.TrimSuffix(v, "x"))
		if !ok || f < 0 {
			return ProgressSample{}, false
		}
		s.Speed = f
	case "progress":
		s.Done = v == "end"
	default:
		return ProgressSample{}, false
	}
	return s, true
}

// parseFiniteFloat rejects NaN and Inf, which ffmpeg can emit for fps/speed
// in corner cases and strconv.ParseFloat would otherwise accept.
func parseFiniteFloat(v string) (float64, bool) {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// ProgressTracker merges successive progress lines into one coherent sample.
type ProgressTracker struct {
	cur ProgressSample
}

// Observe folds a raw line into the current state and reports whether the
// line was recognized as progress output.
func (t *ProgressTracker) Observe(line string) (ProgressSample, bool) {
	s, ok := ParseProgressLine(line)
	if !ok {
		return t.cur, false
	}
	if s.Frame > t.cur.Frame {
		t.cur.Frame = s.Frame
	}
	if s.FPS > t.cur.FPS {
		t.cur.FPS = s.FPS
	}
	if s.OutTime > t.cur.OutTime {
		t.cur.OutTime = s.OutTime
	}
	if s.Speed > 0 {
		t.cur.Speed = s.Speed
	}
	if s.Done {
		t.cur.Done = true
	}
	return t.cur, true
}

func (t ProgressTracker) Sample() ProgressSample { return t.cur }

// Complete snaps the sample to a finished full-length state so a successful
// run always ends the bar at 100%.
func (t *ProgressTracker) Complete(total time.Duration) {
	t.cur.Done = true
	if total > t.cur.OutTime {
		t.cur.OutTime = total
	}
}

// Percent converts the processed time into 0..1 against a known total;
// an unknown or non-positive total yields 0 (indeterminate).
func (s ProgressSample) Percent(total time.Duration) float64 {
	if total <= 0 || s.OutTime <= 0 {
		return 0
	}
	p := float64(s.OutTime) / float64(total)
	if p > 1 {
		p = 1
	}
	return p
}

func parseClock(v string) (time.Duration, bool) {
	parts := strings.Split(v, ":")
	if len(parts) < 3 {
		return 0, false
	}
	h, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, false
	}
	m, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, false
	}
	sec, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	if err != nil {
		return 0, false
	}
	d := (time.Duration(h)*time.Hour +
		time.Duration(m)*time.Minute +
		time.Duration(sec*float64(time.Second)))
	if d < 0 {
		return 0, false
	}
	return d, true
}
