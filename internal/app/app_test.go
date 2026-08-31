package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fsx8/ffwiz/internal/execx"
)

// pickyRunner fails the -version probe for selected binaries only, so both
// branches of the startup check (ffmpeg vs ffprobe) can be exercised.
type pickyRunner struct {
	failFor map[string]bool
}

func (f pickyRunner) Run(_ context.Context, cmd execx.Cmd) (string, string, error) {
	if f.failFor[cmd.Name] {
		return "", "", errors.New("exec: " + cmd.Name + ": executable file not found in $PATH")
	}
	return "", "", nil
}

func (f pickyRunner) RunStreaming(_ context.Context, cmd execx.Cmd, _ func(string), _ func(string)) (int, error) {
	if f.failFor[cmd.Name] {
		return 1, errors.New("exec: " + cmd.Name + ": executable file not found in $PATH")
	}
	return 0, nil
}

func TestCheckFFmpegFFprobe_BothToolsFound(t *testing.T) {
	if err := CheckFFmpegFFprobe(context.Background(), fakeRunner{}); err != nil {
		t.Fatalf("expected no error when both tools respond, got %v", err)
	}
}

func TestCheckFFmpegFFprobe_MissingFFmpeg(t *testing.T) {
	err := CheckFFmpegFFprobe(context.Background(), pickyRunner{failFor: map[string]bool{"ffmpeg": true}})
	if err == nil {
		t.Fatal("a missing ffmpeg must abort the start")
	}
	if !strings.Contains(err.Error(), "ffmpeg not found in PATH") {
		t.Fatalf("error must name the missing tool, got %v", err)
	}
	if !strings.Contains(err.Error(), "Install ffmpeg") {
		t.Fatalf("error must include an install hint, got %v", err)
	}
}

func TestCheckFFmpegFFprobe_MissingFFprobe(t *testing.T) {
	// ffprobe ships with ffmpeg but a broken install can lack it; it must
	// be checked too and named in the error.
	err := CheckFFmpegFFprobe(context.Background(), pickyRunner{failFor: map[string]bool{"ffprobe": true}})
	if err == nil {
		t.Fatal("a missing ffprobe must abort the start")
	}
	if !strings.Contains(err.Error(), "ffprobe not found in PATH") {
		t.Fatalf("error must name the missing tool, got %v", err)
	}
}
