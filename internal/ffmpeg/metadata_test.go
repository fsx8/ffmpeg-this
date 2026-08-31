package ffmpeg

import (
	"reflect"
	"testing"
)

func TestSetMetadataOutputName(t *testing.T) {
	if got, want := SetMetadataOutputName("/a/b/clip.mp4"), "clip_tagged.mp4"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStripMetadataOutputName(t *testing.T) {
	if got, want := StripMetadataOutputName("/a/b/movie.mkv"), "movie_stripped.mkv"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildSetMetadataCmd_SortsTagsByKey(t *testing.T) {
	cmd := BuildSetMetadataCmd("in.mp4", map[string]string{
		"title":   "My Title",
		"artist":  "An Artist",
		"comment": "A Comment",
	}, "out.mp4")
	want := []string{
		"-i", "in.mp4",
		"-map", "0",
		"-c", "copy",
		"-metadata", "artist=An Artist",
		"-metadata", "comment=A Comment",
		"-metadata", "title=My Title",
		"-y", "out.mp4",
	}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("args = %#v, want %#v", cmd.Args, want)
	}
}

func TestBuildSetMetadataCmd_SingleTag(t *testing.T) {
	cmd := BuildSetMetadataCmd("in.mp4", map[string]string{"title": "Only"}, "out.mp4")
	want := []string{
		"-i", "in.mp4",
		"-map", "0",
		"-c", "copy",
		"-metadata", "title=Only",
		"-y", "out.mp4",
	}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("args = %#v, want %#v", cmd.Args, want)
	}
}

func TestBuildSetMetadataCmd_ValueWithSpacesStaysOneArg(t *testing.T) {
	cmd := BuildSetMetadataCmd("in.mp4", map[string]string{"title": "My Home Video 2024"}, "out.mp4")
	found := 0
	for _, a := range cmd.Args {
		if a == "title=My Home Video 2024" {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("expected exactly one unsplit arg %q, got %d in %#v", "title=My Home Video 2024", found, cmd.Args)
	}
}

func TestBuildStripMetadataCmd_ThreeStreams(t *testing.T) {
	cmd := BuildStripMetadataCmd("in.mkv", 3, "out.mkv")
	want := []string{
		"-i", "in.mkv",
		"-map", "0",
		"-c", "copy",
		"-map_metadata", "-1",
		"-map_chapters", "-1",
		"-metadata:s:0", "title=",
		"-metadata:s:0", "language=",
		"-metadata:s:1", "title=",
		"-metadata:s:1", "language=",
		"-metadata:s:2", "title=",
		"-metadata:s:2", "language=",
		"-y", "out.mkv",
	}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("args = %#v, want %#v", cmd.Args, want)
	}
}

func TestBuildStripMetadataCmd_NoStreamsSkipsPerStreamArgs(t *testing.T) {
	cmd := BuildStripMetadataCmd("in.mkv", 0, "out.mkv")
	want := []string{
		"-i", "in.mkv",
		"-map", "0",
		"-c", "copy",
		"-map_metadata", "-1",
		"-map_chapters", "-1",
		"-y", "out.mkv",
	}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("args = %#v, want %#v", cmd.Args, want)
	}
	for _, a := range cmd.Args {
		if len(a) >= 12 && a[:12] == "-metadata:s:" {
			t.Fatalf("unexpected per-stream arg %q in %#v", a, cmd.Args)
		}
	}
}
