//go:build integration

package integration

import (
	"path/filepath"
	"testing"
	"time"

	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
)

func TestResize480pOnBasic(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "basic_480p.mp4")
	cmd := ffx.BuildResizeCmd(fx(t, "basic.mp4"), -2, 480, out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	v := firstStream(t, res, "video")
	if v.Width < 852 || v.Width > 856 {
		t.Fatalf("width = %d, want 854 ±2 (16:9 aspect preserved)", v.Width)
	}
	if v.Height != 480 {
		t.Fatalf("height = %d, want 480", v.Height)
	}
	if v.CodecName != "h264" {
		t.Fatalf("video codec = %s, want h264", v.CodecName)
	}
	if v.PixFmt != "yuv420p" {
		t.Fatalf("pix_fmt = %s, want yuv420p", v.PixFmt)
	}
	assertDuration(t, res, 20, 1)
	if a := firstStream(t, res, "audio"); a.CodecName != "aac" {
		t.Fatalf("audio codec = %s, want aac (stream copy)", a.CodecName)
	}
}

func TestResize720pPreserves43Aspect(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "join_a_720p.mp4")
	cmd := ffx.BuildResizeCmd(fx(t, "join_a.mp4"), -2, 720, out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	v := firstStream(t, probeFile(t, out), "video")
	if v.Width != 960 || v.Height != 720 {
		t.Fatalf("4:3 source resized to %dx%d, want 960x720 (aspect preserved, even)", v.Width, v.Height)
	}
}

func TestResizeCustomWidthAutoHeight(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "noaudio_resized.mp4")
	cmd := ffx.BuildResizeCmd(fx(t, "noaudio.mp4"), 320, -2, out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	v := firstStream(t, res, "video")
	if v.Width != 320 || v.Height != 180 {
		t.Fatalf("custom width resize produced %dx%d, want 320x180", v.Width, v.Height)
	}
	if got := len(streamsOfType(res, "audio")); got != 0 {
		t.Fatalf("audio-less input must stay audio-less, got %d audio streams", got)
	}
}
