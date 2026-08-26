package ffmpeg

import "testing"

func TestTrimOutputName(t *testing.T) {
	if got, want := TrimOutputName("/a/b/c.mp4"), "c_trimmed.mp4"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildTrimCmd(t *testing.T) {
	cmd := BuildTrimCmd("in.mp4", "00:00:01", "00:00:02", "out.mp4")
	if got, want := cmd.Args[0], "-i"; got != want {
		t.Fatalf("unexpected args: %#v", cmd.Args)
	}
	if got := flagValue(cmd.Args, "-ss"); got != "00:00:01" {
		t.Fatalf("expected -ss 00:00:01, got %q", got)
	}
	if got := flagValue(cmd.Args, "-to"); got != "00:00:02" {
		t.Fatalf("expected -to 00:00:02, got %q", got)
	}
	if got := flagValue(cmd.Args, "-c"); got != "copy" {
		t.Fatalf("expected -c copy, got %q", got)
	}
}

func TestExtractAudioOutputName(t *testing.T) {
	if got, want := ExtractAudioOutputName("movie.mkv", "mp3"), "movie_audio.mp3"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildExtractAudioCmd_MP3UsesLibmp3lame(t *testing.T) {
	cmd := BuildExtractAudioCmd("in.mp4", "mp3", "out.mp3")
	if got := flagValue(cmd.Args, "-acodec"); got != "libmp3lame" {
		t.Fatalf("expected libmp3lame, got %q", got)
	}
}

func TestBuildJoinCmd_FilterComplexIncludesConcat(t *testing.T) {
	cmd := BuildJoinCmd([]string{"a.mp4", "b.mp4"}, "out.mp4", JoinTargets{
		Width:      1920,
		Height:     1080,
		SampleRate: "48000",
		SAR:        "1:1",
	})
	fc := flagValue(cmd.Args, "-filter_complex")
	if fc == "" {
		t.Fatalf("expected filter_complex, args: %#v", cmd.Args)
	}
	if want := "concat=n=2:v=1:a=1[v][a]"; !contains(fc, want) {
		t.Fatalf("expected %q in %q", want, fc)
	}
	if got := flagValue(cmd.Args, "-map"); got != "[v]" {
		t.Fatalf("expected first -map [v], got %q", got)
	}
}

func TestBatchOutputName(t *testing.T) {
	if got, want := BatchOutputName("x/y/z.mov", "mp4"), "z_batch.mp4"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildBatchConvertCmd_VideoSameAsSourceUsesCopy(t *testing.T) {
	cmd := BuildBatchConvertCmd("in.mkv", "out.mp4", "mp4", QualitySame, true)
	if got := flagValue(cmd.Args, "-c"); got != "copy" {
		t.Fatalf("expected -c copy, got %q", got)
	}
}

func TestBuildBatchConvertCmd_VideoQualityNoAudioDisablesAudio(t *testing.T) {
	cmd := BuildBatchConvertCmd("in.mkv", "out.mp4", "mp4", QualityMedium, false)
	foundAn := false
	for _, a := range cmd.Args {
		if a == "-an" {
			foundAn = true
		}
	}
	if !foundAn {
		t.Fatalf("expected -an, got %#v", cmd.Args)
	}
}

func TestBuildGifPaletteCmd(t *testing.T) {
	cmd := BuildGifPaletteCmd("in.mp4", "palette.png")
	if got := flagValue(cmd.Args, "-vf"); got == "" {
		t.Fatalf("expected -vf, args: %#v", cmd.Args)
	}
}

func TestBuildGifFromPaletteCmd(t *testing.T) {
	cmd := BuildGifFromPaletteCmd("in.mp4", "palette.png", "out.gif")
	if got := flagValue(cmd.Args, "-filter_complex"); got == "" {
		t.Fatalf("expected -filter_complex, args: %#v", cmd.Args)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (index(s, sub) >= 0))
}

func index(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
