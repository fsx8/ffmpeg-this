//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsx8/ffwiz/internal/execx"
	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
)

// chapterTitles ffprobes the chapter list directly (-show_chapters is not
// part of the app's Probe call) and returns the chapter titles in order.
func chapterTitles(t *testing.T, path string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	stdout, stderr, err := runner.Run(ctx, execx.Cmd{Name: "ffprobe", Args: []string{
		"-v", "quiet", "-print_format", "json", "-show_chapters", path,
	}})
	if err != nil {
		t.Fatalf("ffprobe -show_chapters failed for %s: %v\n%s", path, err, stderr)
	}
	var report struct {
		Chapters []struct {
			Tags struct {
				Title string `json:"title"`
			} `json:"tags"`
		} `json:"chapters"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("chapter report parse failed: %v", err)
	}
	var titles []string
	for _, ch := range report.Chapters {
		titles = append(titles, ch.Tags.Title)
	}
	return titles
}

func TestMetadataSetTitleAndArtist(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "basic_tagged.mp4")
	cmd := ffx.BuildSetMetadataCmd(fx(t, "basic.mp4"), map[string]string{
		"title":  "Integration Test Title",
		"artist": "ffwiz",
	}, out)
	runFFmpeg(t, 2*time.Minute, cmd.Args)

	res := probeFile(t, out)
	if got := res.Format.Tags["title"]; got != "Integration Test Title" {
		t.Fatalf("format title = %q, want %q", got, "Integration Test Title")
	}
	if got := res.Format.Tags["artist"]; got != "ffwiz" {
		t.Fatalf("format artist = %q, want %q", got, "ffwiz")
	}
	if v := firstStream(t, res, "video"); v.CodecName != "h264" {
		t.Fatalf("streams must pass through untouched, video = %s", v.CodecName)
	}
	if a := firstStream(t, res, "audio"); a.CodecName != "aac" {
		t.Fatalf("streams must pass through untouched, audio = %s", a.CodecName)
	}
	assertDuration(t, res, 20, 1)
}

func TestMetadataStripRemovesStreamAndGlobalTags(t *testing.T) {
	requireTools(t)
	in := fx(t, "multiaudio.mkv")
	res := probeFile(t, in)
	out := filepath.Join(t.TempDir(), "multiaudio_stripped.mkv")
	cmd := ffx.BuildStripMetadataCmd(in, len(res.Streams), out)
	runFFmpeg(t, 2*time.Minute, cmd.Args)

	outRes := probeFile(t, out)
	if got := len(outRes.Streams); got != 3 {
		t.Fatalf("strip must keep all 3 streams, got %d", got)
	}
	assertSeq(t, "video codecs", codecNames(outRes, "video"), []string{"h264"})
	assertSeq(t, "audio codecs", codecNames(outRes, "audio"), []string{"aac", "aac"})
	for i, lang := range tagValues(outRes, "audio", "language") {
		if lang == "eng" || lang == "deu" {
			t.Fatalf("audio %d language = %q, want it cleared", i, lang)
		}
	}
	assertDuration(t, outRes, 6, 1)
}

func TestMetadataRoundTripSetThenStrip(t *testing.T) {
	requireTools(t)
	dir := t.TempDir()
	tagged := filepath.Join(dir, "basic_tagged.mp4")
	runFFmpeg(t, 2*time.Minute, ffx.BuildSetMetadataCmd(
		fx(t, "basic.mp4"), map[string]string{"title": "Integration Test Title"}, tagged).Args)
	if res := probeFile(t, tagged); res.Format.Tags["title"] != "Integration Test Title" {
		t.Fatalf("set step failed, title = %q", res.Format.Tags["title"])
	}

	stripped := filepath.Join(dir, "basic_stripped.mp4")
	runFFmpeg(t, 2*time.Minute, ffx.BuildStripMetadataCmd(tagged, 2, stripped).Args)

	res := probeFile(t, stripped)
	if got := res.Format.Tags["title"]; got == "Integration Test Title" {
		t.Fatalf("title must be gone after stripping, got %q", got)
	}
}

// Tagging a multi-stream file must keep EVERY stream: ffmpeg's default
// selection drops all "non-best" streams unless -map 0 is passed.
func TestMetadataEditKeepsEveryStream(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "hdr4k_tagged.mkv")
	cmd := ffx.BuildSetMetadataCmd(fx(t, "hdr4k.mkv"), map[string]string{
		"title": "Edited Title",
	}, out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	if got := res.Format.Tags["title"]; got != "Edited Title" {
		t.Fatalf("title = %q, want %q", got, "Edited Title")
	}
	if got := len(res.Streams); got != 8 {
		t.Fatalf("tag edit must keep all 8 streams, got %d", got)
	}
	if got := len(streamsOfType(res, "audio")); got != 3 {
		t.Fatalf("all 3 audio streams must survive, got %d", got)
	}
	if got := len(streamsOfType(res, "subtitle")); got != 4 {
		t.Fatalf("all 4 subtitle streams must survive, got %d", got)
	}
}

// An empty -metadata value deletes a tag; the wizard uses this to let the
// user clear the title/artist/comment fields.
func TestMetadataEditCanClearTag(t *testing.T) {
	requireTools(t)
	dir := t.TempDir()
	tagged := filepath.Join(dir, "tagged.mp4")
	runFFmpeg(t, 2*time.Minute, ffx.BuildSetMetadataCmd(
		fx(t, "basic.mp4"), map[string]string{"title": "To Be Removed"}, tagged).Args)
	if res := probeFile(t, tagged); res.Format.Tags["title"] != "To Be Removed" {
		t.Fatalf("setup failed, title = %q", res.Format.Tags["title"])
	}

	cleared := filepath.Join(dir, "cleared.mp4")
	runFFmpeg(t, 2*time.Minute, ffx.BuildSetMetadataCmd(
		tagged, map[string]string{"title": ""}, cleared).Args)

	if got := probeFile(t, cleared).Format.Tags["title"]; got != "" {
		t.Fatalf("title must be deleted by an empty value, got %q", got)
	}
}

// --- chapters: strip removes them, tag edits retain them ---

func TestChaptersFixtureCarriesTwoChapters(t *testing.T) {
	requireTools(t)
	got := chapterTitles(t, fx(t, "chapters.mkv"))
	want := []string{"Part One", "Part Two"}
	if len(got) != len(want) {
		t.Fatalf("fixture chapters = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("fixture chapters = %v, want %v", got, want)
		}
	}
}

// The metadata strip must also remove chapter marks (-map_chapters -1).
func TestMetadataStripRemovesChapters(t *testing.T) {
	requireTools(t)
	in := fx(t, "chapters.mkv")
	res := probeFile(t, in)
	out := filepath.Join(t.TempDir(), "chapters_stripped.mkv")
	cmd := ffx.BuildStripMetadataCmd(in, len(res.Streams), out)
	runFFmpeg(t, 2*time.Minute, cmd.Args)

	outRes := probeFile(t, out)
	if got := chapterTitles(t, out); len(got) != 0 {
		t.Fatalf("strip must remove every chapter, got %v", got)
	}
	if got := len(outRes.Streams); got != 2 {
		t.Fatalf("strip must keep all 2 streams, got %d", got)
	}
	assertSeq(t, "video codecs", codecNames(outRes, "video"), []string{"h264"})
	assertSeq(t, "audio codecs", codecNames(outRes, "audio"), []string{"aac"})
	assertDuration(t, outRes, 20, 1)
}

// A plain tag edit must leave the chapter marks in place.
func TestMetadataSetTagsRetainsChapters(t *testing.T) {
	requireTools(t)
	in := fx(t, "chapters.mkv")
	out := filepath.Join(t.TempDir(), "chapters_tagged.mkv")
	cmd := ffx.BuildSetMetadataCmd(in, map[string]string{"title": "Chaptered Fixture"}, out)
	runFFmpeg(t, 2*time.Minute, cmd.Args)

	res := probeFile(t, out)
	if got := res.Format.Tags["title"]; got != "Chaptered Fixture" {
		t.Fatalf("title = %q, want %q", got, "Chaptered Fixture")
	}
	got := chapterTitles(t, out)
	want := []string{"Part One", "Part Two"}
	if len(got) != len(want) {
		t.Fatalf("tag edit must retain the chapters, got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tag edit must retain the chapters, got %v want %v", got, want)
		}
	}
}
