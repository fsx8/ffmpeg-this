//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
)

func TestScreenshotPngAtTimestamp(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "frame.png")
	cmd := ffx.BuildScreenshotCmd(fx(t, "basic.mp4"), "00:00:05", out)
	runFFmpeg(t, 2*time.Minute, cmd.Args)

	if _, err := os.Stat(out); err != nil {
		t.Fatalf("screenshot was not written: %v", err)
	}
	res := probeFile(t, out)
	v := firstStream(t, res, "video")
	if v.CodecName != "png" {
		t.Fatalf("codec = %s, want png", v.CodecName)
	}
	if v.Width != 640 || v.Height != 360 {
		t.Fatalf("frame = %dx%d, want 640x360", v.Width, v.Height)
	}
}

func TestScreenshotJpgAtSeconds(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "frame.jpg")
	cmd := ffx.BuildScreenshotCmd(fx(t, "join_a.mp4"), "2", out)
	runFFmpeg(t, 2*time.Minute, cmd.Args)

	if _, err := os.Stat(out); err != nil {
		t.Fatalf("screenshot was not written: %v", err)
	}
	res := probeFile(t, out)
	v := firstStream(t, res, "video")
	if v.CodecName != "mjpeg" {
		t.Fatalf("codec = %s, want mjpeg", v.CodecName)
	}
	if v.Width != 640 || v.Height != 480 {
		t.Fatalf("frame = %dx%d, want 640x480", v.Width, v.Height)
	}
}
