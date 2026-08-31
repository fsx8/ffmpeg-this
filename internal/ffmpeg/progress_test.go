package ffmpeg

import (
	"testing"
	"time"
)

func TestParseProgressLine_KnownKeys(t *testing.T) {
	cases := []struct {
		line string
		want ProgressSample
	}{
		{"frame=129", ProgressSample{Frame: 129}},
		{"fps=44.42", ProgressSample{FPS: 44.42}},
		{"out_time_us=12870000", ProgressSample{OutTime: 12870 * time.Millisecond}},
		{"out_time_ms=12870000", ProgressSample{OutTime: 12870 * time.Millisecond}},
		{"out_time=00:01:02.500000", ProgressSample{OutTime: 62*time.Second + 500*time.Millisecond}},
		{"speed=2.89x", ProgressSample{Speed: 2.89}},
		{"progress=continue", ProgressSample{}},
		{"progress=end", ProgressSample{Done: true}},
	}
	for _, c := range cases {
		got, ok := ParseProgressLine(c.line)
		if !ok {
			t.Fatalf("%q: expected recognized progress line", c.line)
		}
		if got != c.want {
			t.Fatalf("%q: got %+v want %+v", c.line, got, c.want)
		}
	}
}

func TestParseProgressLine_Rejects(t *testing.T) {
	lines := []string{
		"",
		"no equals sign",
		"stream_0_0_q=-1.0",
		"total_size=1234567",
		"frame=abc",
		"out_time_us=-9223372036854775808",
		"out_time=N/A",
		"speed=nan",
	}
	for _, l := range lines {
		if _, ok := ParseProgressLine(l); ok {
			t.Fatalf("%q: expected to be rejected", l)
		}
	}
}

func TestProgressTracker_MergesLines(t *testing.T) {
	var tr ProgressTracker
	lines := []string{
		"frame=10",
		"out_time_us=1000000",
		"speed=3x",
		"out_time_us=2500000",
		"progress=continue",
		"progress=end",
	}
	var last ProgressSample
	for _, l := range lines {
		s, ok := tr.Observe(l)
		if !ok {
			t.Fatalf("line %q not recognized", l)
		}
		last = s
	}
	if last.Frame != 10 || last.OutTime != 2500*time.Millisecond || !last.Done {
		t.Fatalf("unexpected merged sample: %+v", last)
	}
}

func TestProgressTracker_FPSKeepsLatestValue(t *testing.T) {
	var tr ProgressTracker
	tr.Observe("fps=10.0")
	tr.Observe("fps=40.0")
	tr.Observe("fps=20.0")
	if s := tr.Sample(); s.FPS != 20 {
		t.Fatalf("fps = %v, want the latest value 20", s.FPS)
	}
	tr.Observe("out_time_us=1000") // lines without fps must not reset it
	if s := tr.Sample(); s.FPS != 20 {
		t.Fatalf("fps = %v, want 20 preserved, got sample %+v", s.FPS, s)
	}
}

func TestProgressSample_Percent(t *testing.T) {
	cases := []struct {
		sample ProgressSample
		total  time.Duration
		want   float64
	}{{
		sample: ProgressSample{OutTime: 30 * time.Second}, total: time.Minute, want: 0.5,
	}, {
		sample: ProgressSample{OutTime: 90 * time.Second}, total: time.Minute, want: 1,
	}, {
		sample: ProgressSample{OutTime: 30 * time.Second}, total: 0, want: 0,
	}}
	for i, c := range cases {
		if got := c.sample.Percent(c.total); got != c.want {
			t.Fatalf("case %d: got %v want %v", i, got, c.want)
		}
	}
}

func TestAddProgressArgs_PrependsFlags(t *testing.T) {
	args := AddProgressArgs([]string{"-i", "in.mp4", "-c", "copy", "-y", "out.mp4"})
	want := []string{"-nostats", "-progress", "pipe:1", "-i", "in.mp4", "-c", "copy", "-y", "out.mp4"}
	if len(args) != len(want) {
		t.Fatalf("got %#v want %#v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("got %#v want %#v", args, want)
		}
	}
}
