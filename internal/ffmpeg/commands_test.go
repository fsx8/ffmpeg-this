package ffmpeg

import (
	"strings"
	"testing"
)

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

func TestSnapToKeyframe(t *testing.T) {
	kf := []float64{0, 2, 4, 6, 8, 10}
	cases := []struct {
		start float64
		want  float64
	}{
		{5, 4},
		{4, 4},
		{4.04, 4},
		{3.99, 4},
		{3.94, 2},
		{0, 0},
		{20, 10},
		{-1, 0},
	}
	for _, c := range cases {
		if got := SnapToKeyframe(c.start, kf); got != c.want {
			t.Fatalf("SnapToKeyframe(%v) = %v, want %v", c.start, got, c.want)
		}
	}
	if got := SnapToKeyframe(5, nil); got != 5 {
		t.Fatalf("empty keyframes must not alter start, got %v", got)
	}
	if got := SnapToKeyframe(5, []float64{6}); got != 6 {
		t.Fatalf("start before first keyframe must snap forward, got %v", got)
	}
}

func TestFormatTimeSpec(t *testing.T) {
	cases := []struct {
		sec  float64
		want string
	}{
		{0, "00:00:00"},
		{4, "00:00:04"},
		{65.5, "00:01:05.500"},
		{3661.25, "01:01:01.250"},
		{-3, "00:00:00"},
	}
	for _, c := range cases {
		if got := FormatTimeSpec(c.sec); got != c.want {
			t.Fatalf("FormatTimeSpec(%v) = %q, want %q", c.sec, got, c.want)
		}
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
	if got := flagValue(cmd.Args, "-b:a"); got != "192k" {
		t.Fatalf("expected -b:a 192k, got %q", got)
	}
}

func TestBuildExtractAudioCmd_WavUsesPCM(t *testing.T) {
	cmd := BuildExtractAudioCmd("in.mp4", "wav", "out.wav")
	if got := flagValue(cmd.Args, "-acodec"); got != "pcm_s16le" {
		t.Fatalf("expected pcm_s16le (wav is a container, not an encoder), got %q", got)
	}
	if got := flagValue(cmd.Args, "-b:a"); got != "" {
		t.Fatalf("expected no bitrate for PCM, got %q", got)
	}
}

func TestBuildExtractAudioCmd_FlacUsesNativeEncoder(t *testing.T) {
	cmd := BuildExtractAudioCmd("in.mp4", "flac", "out.flac")
	if got := flagValue(cmd.Args, "-acodec"); got != "flac" {
		t.Fatalf("expected native flac encoder, got %q", got)
	}
}

func TestBuildJoinCmd_AllInputsHaveAudio(t *testing.T) {
	cmd := BuildJoinCmd(
		[]JoinInput{{Path: "a.mp4", HasAudio: true}, {Path: "b.mp4", HasAudio: true}},
		"out.mp4",
		JoinTargets{Width: 1920, Height: 1080, SampleRate: "48000", SAR: "1:1"},
	)
	fc := flagValue(cmd.Args, "-filter_complex")
	if fc == "" {
		t.Fatalf("expected filter_complex, args: %#v", cmd.Args)
	}
	if want := "concat=n=2:v=1:a=1[v][a]"; !strings.Contains(fc, want) {
		t.Fatalf("expected %q in %q", want, fc)
	}
	if want := "aformat=sample_rates=48000:channel_layouts=stereo"; !strings.Contains(fc, want) {
		t.Fatalf("expected audio normalization %q in %q", want, fc)
	}
	if strings.Contains(fc, "anullsrc") {
		t.Fatalf("did not expect synthesized silence: %q", fc)
	}
	if got := flagValue(cmd.Args, "-map"); got != "[v]" {
		t.Fatalf("expected first -map [v], got %q", got)
	}
	if got := flagValue(cmd.Args, "-c:a"); got != "aac" {
		t.Fatalf("expected -c:a aac, got %q", got)
	}
}

func TestBuildJoinCmd_NoInputHasAudioDropsAudio(t *testing.T) {
	cmd := BuildJoinCmd(
		[]JoinInput{{Path: "a.mp4"}, {Path: "b.mp4"}},
		"out.mp4",
		JoinTargets{Width: 1280, Height: 720, SampleRate: "44100"},
	)
	fc := flagValue(cmd.Args, "-filter_complex")
	if want := "concat=n=2:v=1:a=0[v]"; !strings.Contains(fc, want) {
		t.Fatalf("expected video-only concat %q in %q", want, fc)
	}
	for _, a := range cmd.Args {
		if a == "-c:a" || a == "-b:a" {
			t.Fatalf("expected no audio encoder args, got %#v", cmd.Args)
		}
	}
}

func TestBuildJoinCmd_MixedAudioSynthesizesSilence(t *testing.T) {
	cmd := BuildJoinCmd(
		[]JoinInput{
			{Path: "a.mp4", HasAudio: true, DurationSec: 12},
			{Path: "b.mp4", HasAudio: false, DurationSec: 2.5},
		},
		"out.mp4",
		JoinTargets{Width: 1280, Height: 720, SampleRate: "48000"},
	)
	fc := flagValue(cmd.Args, "-filter_complex")
	if want := "anullsrc=channel_layout=stereo:sample_rate=48000,atrim=duration=2.5"; !strings.Contains(fc, want) {
		t.Fatalf("expected synthesized silence %q in %q", want, fc)
	}
	if want := "concat=n=2:v=1:a=1[v][a]"; !strings.Contains(fc, want) {
		t.Fatalf("expected %q in %q", want, fc)
	}
}

func TestBuildJoinCmd_MixedAudioUnknownDurationFallsBackToVideoOnly(t *testing.T) {
	cmd := BuildJoinCmd(
		[]JoinInput{
			{Path: "a.mp4", HasAudio: true, DurationSec: 12},
			{Path: "b.mp4"}, // no audio, duration unknown
		},
		"out.mp4",
		JoinTargets{Width: 1280, Height: 720, SampleRate: "48000"},
	)
	fc := flagValue(cmd.Args, "-filter_complex")
	if want := "concat=n=2:v=1:a=0[v]"; !strings.Contains(fc, want) {
		t.Fatalf("expected video-only fallback %q in %q", want, fc)
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

func TestBuildBatchConvertCmd_WebmUsesVP9AndOpus(t *testing.T) {
	cmd := BuildBatchConvertCmd("in.mp4", "out.webm", "webm", QualityMedium, true)
	if got := flagValue(cmd.Args, "-c:v"); got != "libvpx-vp9" {
		t.Fatalf("expected libvpx-vp9 for webm, got %q", got)
	}
	if got := flagValue(cmd.Args, "-c:a"); got != "libopus" {
		t.Fatalf("expected libopus for webm, got %q", got)
	}
	if got := flagValue(cmd.Args, "-crf"); got != "31" {
		t.Fatalf("expected VP9-appropriate CRF, got %q", got)
	}
	if got := flagValue(cmd.Args, "-b:v"); got != "0" {
		t.Fatalf("expected -b:v 0 (quality mode) for VP9, got %q", got)
	}
	for _, a := range cmd.Args {
		if a == "aac" || a == "libx264" {
			t.Fatalf("webm cannot hold H.264/AAC: %#v", cmd.Args)
		}
	}
}

func TestBuildBatchConvertCmd_WebmSameQualityReEncodes(t *testing.T) {
	// Stream copy would put H.264/AAC into the webm muxer, which fails;
	// fall back to a re-encode instead of producing a broken command.
	cmd := BuildBatchConvertCmd("in.mp4", "out.webm", "webm", QualitySame, true)
	for _, a := range cmd.Args {
		if a == "copy" {
			t.Fatalf("expected re-encode for webm copy preset, got %#v", cmd.Args)
		}
	}
	if got := flagValue(cmd.Args, "-c:v"); got != "libvpx-vp9" {
		t.Fatalf("expected libvpx-vp9, got %q", got)
	}
}

func TestBuildBatchConvertCmd_WebmNoAudioDisablesAudio(t *testing.T) {
	cmd := BuildBatchConvertCmd("in.mp4", "out.webm", "webm", QualityLow, false)
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

func TestBuildBatchConvertCmd_WavUsesPCM(t *testing.T) {
	cmd := BuildBatchConvertCmd("in.mp4", "out.wav", "wav", QualitySame, true)
	if got := flagValue(cmd.Args, "-c:a"); got != "pcm_s16le" {
		t.Fatalf("expected pcm_s16le, got %q", got)
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
