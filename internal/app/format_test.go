package app

import (
	"context"

	"testing"
	"time"
	"unicode/utf8"

	"github.com/fsx8/ffwiz/internal/execx"
	ffprobe "github.com/fsx8/ffwiz/internal/ffprobe"
)

func TestFormatDur(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "00:00"},
		{59 * time.Second, "00:59"},
		{time.Minute, "01:00"},
		{3661 * time.Second, "1:01:01"},
		{-5 * time.Second, "00:00"},
	}
	for _, c := range cases {
		if got := formatDur(c.d); got != c.want {
			t.Fatalf("formatDur(%v) = %q want %q", c.d, got, c.want)
		}
	}
}

func TestRenderBar(t *testing.T) {
	if got := renderBar(12, 0); got != "[░░░░░░░░░░]" {
		t.Fatalf("empty bar: %q", got)
	}
	if got := renderBar(12, 1); got != "[██████████]" {
		t.Fatalf("full bar: %q", got)
	}
	half := renderBar(12, 0.5)
	if got, want := utf8.RuneCountInString(half), 12; got != want {
		t.Fatalf("half bar width: %d want %d (%q)", got, want, half)
	}
	if half != "[█████░░░░░]" {
		t.Fatalf("half bar fill: %q", half)
	}
	if got := renderBar(3, 0.7); got != "[███░]" {
		t.Fatalf("narrow clamp bar: %q", got)
	}
	if got := renderBar(200, 0.5); utf8.RuneCountInString(got) != 80 {
		t.Fatalf("wide clamp width: %q", got)
	}
}

func TestParseProbeSeconds(t *testing.T) {
	if d, ok := parseProbeSeconds("  12.5 "); !ok || d != 12500*time.Millisecond {
		t.Fatalf("got %v ok=%v", d, ok)
	}
	for _, v := range []string{"N/A", "", "-3", "abc", "0"} {
		if _, ok := parseProbeSeconds(v); ok {
			t.Fatalf("%q should not parse", v)
		}
	}
}

type stubProber struct {
	durs map[string]string
}

func (s stubProber) Probe(_ context.Context, path string) (*ffprobe.ProbeResult, error) {
	dur, ok := s.durs[path]
	if !ok {
		return &ffprobe.ProbeResult{}, nil
	}
	return &ffprobe.ProbeResult{Format: ffprobe.Format{Duration: dur}}, nil
}

func (s stubProber) HasAudio(ctx context.Context, path string) (bool, error) {
	res, err := s.Probe(ctx, path)
	if err != nil {
		return false, err
	}
	for _, st := range res.Streams {
		if st.CodecType == "audio" {
			return true, nil
		}
	}
	return false, nil
}

func (s stubProber) Keyframes(_ context.Context, _ string) ([]float64, error) {
	return nil, nil
}

func TestExecTotalDuration(t *testing.T) {
	p := stubProber{durs: map[string]string{
		"a.mp4": "10",
		"b.mp4": "2.5",
	}}
	got := execTotalDuration(context.Background(), p, []string{"-i", "a.mp4", "-i", "b.mp4", "-y", "out.mp4"})
	if got != 12500*time.Millisecond {
		t.Fatalf("join sum: got %v", got)
	}

	got = execTotalDuration(context.Background(), p, []string{"-nostats", "-progress", "pipe:1", "-i", "b.mp4", "-y", "out.mp4"})
	if got != 2500*time.Millisecond {
		t.Fatalf("single input: got %v", got)
	}

	if got := execTotalDuration(context.Background(), p, []string{"-y", "out.mp4"}); got != 0 {
		t.Fatalf("no inputs: got %v", got)
	}
}

func TestExecModelProgressFlow(t *testing.T) {
	m := newExecScreen(Config{Prober: stubProber{}}, "t", execx.Cmd{Name: "ffmpeg", Args: []string{"-i", "in.mp4"}})
	m.Update(execProbeMsg{total: time.Minute})
	m.Update(execProgMsg{line: "out_time_us=30000000"})
	s := m.tracker.Sample()
	if s.Percent(m.dur) != 0.5 {
		t.Fatalf("expected 50%% after 30s/60s, got %v", s.Percent(m.dur))
	}
	if m.progressLine() == "" {
		t.Fatal("progress line should render once duration and out_time are known")
	}
	m.tracker.Complete(m.dur)
	if m.tracker.Sample().Percent(m.dur) != 1 {
		t.Fatal("complete must reach 100%")
	}
}
