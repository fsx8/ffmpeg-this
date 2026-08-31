package ffmpeg

import (
	"reflect"
	"testing"
)

func TestCompressOutputName(t *testing.T) {
	if got, want := CompressOutputName("/a/b/clip.mp4", 28), "clip_crf28.mp4"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got, want := CompressOutputName("movie.mkv", 18), "movie_crf18.mkv"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildCompressCmd_Crf28Veryfast(t *testing.T) {
	cmd := BuildCompressCmd("in.mp4", 28, "veryfast", "out.mp4")
	want := []string{
		"-i", "in.mp4",
		"-c:v", "libx264",
		"-crf", "28",
		"-preset", "veryfast",
		"-pix_fmt", "yuv420p",
		"-c:a", "copy",
		"-y", "out.mp4",
	}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("args = %#v, want %#v", cmd.Args, want)
	}
}

func TestBuildCompressCmd_Crf18Medium(t *testing.T) {
	cmd := BuildCompressCmd("in.mp4", 18, "medium", "out.mp4")
	if got := flagValue(cmd.Args, "-crf"); got != "18" {
		t.Fatalf("-crf = %q, want 18", got)
	}
	if got := flagValue(cmd.Args, "-preset"); got != "medium" {
		t.Fatalf("-preset = %q, want medium", got)
	}
	if got := flagValue(cmd.Args, "-c:v"); got != "libx264" {
		t.Fatalf("-c:v = %q, want libx264", got)
	}
	if got := flagValue(cmd.Args, "-pix_fmt"); got != "yuv420p" {
		t.Fatalf("-pix_fmt = %q, want yuv420p", got)
	}
	if got := flagValue(cmd.Args, "-c:a"); got != "copy" {
		t.Fatalf("-c:a = %q, want copy", got)
	}
}
