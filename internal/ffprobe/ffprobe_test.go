package ffprobe

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/fsx8/ffwiz/internal/execx"
)

// recordingRunner stands in for a real execx.Runner: it records the last
// command it received and replies with canned output.
type recordingRunner struct {
	stdout string
	stderr string
	err    error

	lastCmd execx.Cmd
	runs    int
}

func (r *recordingRunner) Run(_ context.Context, cmd execx.Cmd) (string, string, error) {
	r.lastCmd = cmd
	r.runs++
	return r.stdout, r.stderr, r.err
}

func (r *recordingRunner) RunStreaming(context.Context, execx.Cmd, func(string), func(string)) (int, error) {
	return 0, errors.New("RunStreaming is not expected in these tests")
}

// --- arg construction of the real prober ---

func TestProber_ProbeBuildsJSONReportArgs(t *testing.T) {
	runner := &recordingRunner{stdout: `{"format": {"duration": "12.5"}}`}
	p := New(runner)

	res, err := p.Probe(context.Background(), "clip.mp4")
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	if runner.runs != 1 {
		t.Fatalf("probe must run ffprobe exactly once, ran %d times", runner.runs)
	}
	if got := runner.lastCmd.Name; got != "ffprobe" {
		t.Fatalf("command name = %q, want ffprobe", got)
	}
	wantArgs := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"clip.mp4",
	}
	if got := runner.lastCmd.Args; !reflect.DeepEqual(got, wantArgs) {
		t.Fatalf("args:\ngot  %#v\nwant %#v", got, wantArgs)
	}
	if d, ok := res.Duration(); !ok || d != 12.5 {
		t.Fatalf("parsed duration = %v %v, want 12.5 true", d, ok)
	}
}

func TestProber_KeyframesBuildsNokeyArgs(t *testing.T) {
	runner := &recordingRunner{stdout: `{"frames": [{"pts_time": "0"}, {"pts_time": "2"}]}`}
	p := New(runner)

	kf, err := p.Keyframes(context.Background(), "clip.mp4")
	if err != nil {
		t.Fatalf("keyframes probe failed: %v", err)
	}
	if got := runner.lastCmd.Name; got != "ffprobe" {
		t.Fatalf("command name = %q, want ffprobe", got)
	}
	wantArgs := []string{
		"-v", "quiet",
		"-skip_frame", "nokey",
		"-select_streams", "v:0",
		"-show_entries", "frame=pts_time",
		"-of", "json",
		"clip.mp4",
	}
	if got := runner.lastCmd.Args; !reflect.DeepEqual(got, wantArgs) {
		t.Fatalf("args:\ngot  %#v\nwant %#v", got, wantArgs)
	}
	if !reflect.DeepEqual(kf, []float64{0, 2}) {
		t.Fatalf("keyframes = %v, want [0 2]", kf)
	}
}

// --- error propagation ---

func TestProber_ProbeRunnerErrorSurfacesStderr(t *testing.T) {
	runner := &recordingRunner{err: errors.New("exit status 1"), stderr: "clip.mp4: Invalid data"}
	p := New(runner)

	if _, err := p.Probe(context.Background(), "clip.mp4"); err == nil || !strings.Contains(err.Error(), "ffprobe failed") || !strings.Contains(err.Error(), "Invalid data") {
		t.Fatalf("runner failure must surface as ffprobe error incl. stderr, got %v", err)
	}
}

func TestProber_ProbeGarbageOutputFailsToParse(t *testing.T) {
	runner := &recordingRunner{stdout: "<html>not json</html>"}
	p := New(runner)

	if _, err := p.Probe(context.Background(), "clip.mp4"); err == nil || !strings.Contains(err.Error(), "ffprobe output parse failed") {
		t.Fatalf("garbage output must fail with a parse error, got %v", err)
	}
}

func TestProber_ProbeEmptyStdoutFailsToParse(t *testing.T) {
	runner := &recordingRunner{stdout: ""}
	p := New(runner)

	if _, err := p.Probe(context.Background(), "clip.mp4"); err == nil {
		t.Fatal("empty stdout must not parse into a result")
	}
}

func TestProber_KeyframesRunnerErrorSurfacesStderr(t *testing.T) {
	runner := &recordingRunner{err: errors.New("exit status 1"), stderr: "no such file"}
	p := New(runner)

	if _, err := p.Keyframes(context.Background(), "clip.mp4"); err == nil || !strings.Contains(err.Error(), "ffprobe keyframes failed") || !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("runner failure must surface as keyframes error incl. stderr, got %v", err)
	}
}

func TestProber_KeyframesGarbageOutputFailsToParse(t *testing.T) {
	runner := &recordingRunner{stdout: "garbage"}
	p := New(runner)

	if _, err := p.Keyframes(context.Background(), "clip.mp4"); err == nil || !strings.Contains(err.Error(), "ffprobe keyframes parse failed") {
		t.Fatalf("garbage output must fail with a parse error, got %v", err)
	}
}

// ffprobe reports "N/A" for frames without a presentation timestamp; such
// entries (and unparseable or negative ones) must be skipped, not crash
// the trim wizard's keyframe snapping.
func TestProber_KeyframesFiltersInvalidPTSTimes(t *testing.T) {
	runner := &recordingRunner{stdout: `{"frames": [
		{"pts_time": "N/A"},
		{"pts_time": ""},
		{"pts_time": "0"},
		{"pts_time": "junk"},
		{"pts_time": "-1.5"},
		{"pts_time": "2.5"},
		{}
	]}`}
	p := New(runner)

	kf, err := p.Keyframes(context.Background(), "clip.mp4")
	if err != nil {
		t.Fatalf("keyframes probe failed: %v", err)
	}
	if !reflect.DeepEqual(kf, []float64{0, 2.5}) {
		t.Fatalf("keyframes = %v, want [0 2.5] (only valid pts_time entries survive)", kf)
	}
}

func TestProber_HasAudioReflectsProbe(t *testing.T) {
	cases := []struct {
		name   string
		stdout string
		want   bool
	}{
		{"with audio", `{"streams": [{"codec_type": "video"}, {"codec_type": "audio"}]}`, true},
		{"video only", `{"streams": [{"codec_type": "video"}]}`, false},
		{"no streams", `{"streams": []}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := New(&recordingRunner{stdout: c.stdout})
			got, err := p.HasAudio(context.Background(), "clip.mp4")
			if err != nil {
				t.Fatalf("HasAudio failed: %v", err)
			}
			if got != c.want {
				t.Fatalf("HasAudio = %v, want %v", got, c.want)
			}
		})
	}
}
