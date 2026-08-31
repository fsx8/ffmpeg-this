//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
)

func TestCompressBasicAtCrf34Veryfast(t *testing.T) {
	requireTools(t)
	in := fx(t, "basic.mp4")
	out := filepath.Join(t.TempDir(), "basic_crf34.mp4")
	cmd := ffx.BuildCompressCmd(in, 34, "veryfast", out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	if v := firstStream(t, res, "video"); v.CodecName != "h264" {
		t.Fatalf("video codec = %s, want h264", v.CodecName)
	}
	assertDuration(t, res, 20, 1)
	if a := firstStream(t, res, "audio"); a.CodecName != "aac" {
		t.Fatalf("audio codec = %s, want aac (stream copy)", a.CodecName)
	}
	inInfo, err := os.Stat(in)
	if err != nil {
		t.Fatal(err)
	}
	outInfo, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if outInfo.Size() >= inInfo.Size() {
		t.Fatalf("compressed output (%d bytes) must be smaller than the input (%d bytes)", outInfo.Size(), inInfo.Size())
	}
}

func TestCompressNoaudioKeepsVideoOnly(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "noaudio_crf23.mp4")
	cmd := ffx.BuildCompressCmd(fx(t, "noaudio.mp4"), 23, "veryfast", out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	if v := firstStream(t, res, "video"); v.CodecName != "h264" {
		t.Fatalf("video codec = %s, want h264", v.CodecName)
	}
	if got := len(streamsOfType(res, "audio")); got != 0 {
		t.Fatalf("audio-less input must stay audio-less, got %d audio streams", got)
	}
	assertDuration(t, res, 6, 1)
}
