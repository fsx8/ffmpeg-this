//go:build integration

package integration

import (
	"path/filepath"
	"testing"
	"time"

	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
)

func TestTransformRotate90(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "basic_rot90.mp4")
	cmd := ffx.BuildRotateCmd(fx(t, "basic.mp4"), 90, out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	v := firstStream(t, res, "video")
	if v.Width != 360 || v.Height != 640 {
		t.Fatalf("rotated video = %dx%d, want 360x640", v.Width, v.Height)
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

// 180° keeps the source dimensions (hflip+vflip is dimension-neutral).
func TestTransformRotate180(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "basic_rot180.mp4")
	cmd := ffx.BuildRotateCmd(fx(t, "basic.mp4"), 180, out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	v := firstStream(t, res, "video")
	if v.Width != 640 || v.Height != 360 {
		t.Fatalf("rotated video = %dx%d, want 640x360", v.Width, v.Height)
	}
	if v.CodecName != "h264" {
		t.Fatalf("video codec = %s, want h264", v.CodecName)
	}
	if v.PixFmt != "yuv420p" {
		t.Fatalf("pix_fmt = %s, want yuv420p", v.PixFmt)
	}
	assertDuration(t, res, 20, 1)
}

// 270° swaps the dimensions exactly like 90° (transpose=2).
func TestTransformRotate270(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "basic_rot270.mp4")
	cmd := ffx.BuildRotateCmd(fx(t, "basic.mp4"), 270, out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	v := firstStream(t, res, "video")
	if v.Width != 360 || v.Height != 640 {
		t.Fatalf("rotated video = %dx%d, want 360x640", v.Width, v.Height)
	}
	if v.CodecName != "h264" {
		t.Fatalf("video codec = %s, want h264", v.CodecName)
	}
	assertDuration(t, res, 20, 1)
}

func TestTransformFlipHorizontal(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "join_a_fliph.mp4")
	cmd := ffx.BuildFlipCmd(fx(t, "join_a.mp4"), "h", out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	v := firstStream(t, res, "video")
	if v.Width != 640 || v.Height != 480 {
		t.Fatalf("flipped video = %dx%d, want 640x480", v.Width, v.Height)
	}
	if v.CodecName != "h264" {
		t.Fatalf("video codec = %s, want h264", v.CodecName)
	}
	assertDuration(t, res, 5, 1)
}

// A vertical flip mirrors on the x-axis and must keep the dimensions
// (vflip is dimension-neutral, mirroring the fliph case).
func TestTransformFlipVertical(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "join_a_flipv.mp4")
	cmd := ffx.BuildFlipCmd(fx(t, "join_a.mp4"), "v", out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	v := firstStream(t, res, "video")
	if v.Width != 640 || v.Height != 480 {
		t.Fatalf("flipped video = %dx%d, want 640x480", v.Width, v.Height)
	}
	if v.CodecName != "h264" {
		t.Fatalf("video codec = %s, want h264", v.CodecName)
	}
	assertDuration(t, res, 5, 1)
}

func TestTransformCrop(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "join_a_cropped.mp4")
	cmd := ffx.BuildCropCmd(fx(t, "join_a.mp4"), 160, 120, 320, 240, out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	v := firstStream(t, res, "video")
	if v.Width != 320 || v.Height != 240 {
		t.Fatalf("cropped video = %dx%d, want 320x240", v.Width, v.Height)
	}
	if v.CodecName != "h264" {
		t.Fatalf("video codec = %s, want h264", v.CodecName)
	}
	assertDuration(t, res, 5, 1)
}
