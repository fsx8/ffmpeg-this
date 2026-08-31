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

// A lossless cut must keep every input stream: without -map 0 ffmpeg's
// automatic selection retains only the "best" stream of each type and
// silently drops the rest of a multi-track file.
func TestBuildTrimCmd_MapsEveryStream(t *testing.T) {
	cmd := BuildTrimCmd("in.mkv", "00:00:01", "00:00:02", "out.mkv")
	if got := flagValue(cmd.Args, "-map"); got != "0" {
		t.Fatalf("expected -map 0, got %q", got)
	}
}

// -ss/-to must sit AFTER -i (with -i's value occupying the next slot):
// input-side -to requires ffmpeg >= 5.0, so seeking deliberately happens
// on the output side to stay compatible with ffmpeg 4.x (Ubuntu 22.04).
func TestBuildTrimCmd_SeekFlagsAreOutputSide(t *testing.T) {
	cmd := BuildTrimCmd("in.mp4", "00:00:01", "00:00:02", "out.mp4")
	index := func(arg string) int {
		for i, a := range cmd.Args {
			if a == arg {
				return i
			}
		}
		return -1
	}
	i, ss, to := index("-i"), index("-ss"), index("-to")
	if i < 0 || ss < 0 || to < 0 {
		t.Fatalf("expected -i, -ss and -to, got %#v", cmd.Args)
	}
	if ss <= i+1 || to <= i+1 {
		t.Fatalf("-ss/-to must come after -i and its value (output-side seek): -i@%d -ss@%d -to@%d, args %#v", i, ss, to, cmd.Args)
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

func TestBuildJoinCmd_RoundsOddTargetDimensionsToEven(t *testing.T) {
	// libx264/yuv420p refuses odd dimensions; sources with odd sizes
	// (e.g. prores/444 or jpeg) must not produce a failing command.
	cmd := BuildJoinCmd(
		[]JoinInput{{Path: "a.mov", HasAudio: true}},
		"out.mp4",
		JoinTargets{Width: 853, Height: 479, SampleRate: "48000"},
	)
	fc := flagValue(cmd.Args, "-filter_complex")
	if want := "scale=w=852:h=478"; !strings.Contains(fc, want) {
		t.Fatalf("expected even target %q in %q", want, fc)
	}
	if want := "pad=w=852:h=478"; !strings.Contains(fc, want) {
		t.Fatalf("expected even pad target %q in %q", want, fc)
	}
}

func TestBatchOutputName(t *testing.T) {
	if got, want := BatchOutputName("x/y/z.mov", "mp4"), "z_batch.mp4"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildBatchConvertCmd_SameContainerCopyKeepsEveryStream(t *testing.T) {
	cmd := BuildBatchConvertCmd("in.mkv", "out.mkv", "mkv", QualitySame, true, BatchStreamInfo{})
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "-map 0") || !strings.Contains(joined, "-c copy") {
		t.Fatalf("same-container copy must keep every stream: %s", joined)
	}
}

// Cross-container copy must neither silently drop streams nor die at the
// muxer: muxable audio copies, DTS becomes AAC, the text subtitle converts
// to mov_text, and the bitmap subtitle is dropped explicitly (no muxable
// form exists).
func TestBuildBatchConvertCmd_MP4CopyNormalizesStreams(t *testing.T) {
	info := BatchStreamInfo{
		AudioCodecs:    []string{"aac", "dts"},
		SubtitleCodecs: []string{"subrip", "hdmv_pgs_subtitle"},
	}
	for _, format := range []string{"mp4", "mov"} {
		out := "out." + format
		cmd := BuildBatchConvertCmd("in.mkv", out, format, QualitySame, true, info)
		joined := strings.Join(cmd.Args, " ")
		if !strings.Contains(joined, "-map 0:V?") || !strings.Contains(joined, "-map 0:a?") {
			t.Fatalf("%s: video and audio must be mapped explicitly: %s", format, joined)
		}
		if got := flagValue(cmd.Args, "-c:v"); got != "copy" {
			t.Fatalf("%s: video must be stream-copied, got -c:v %q", format, got)
		}
		if !strings.Contains(joined, "-map 0:s:0") {
			t.Fatalf("%s: the text subtitle must be mapped: %s", format, joined)
		}
		if strings.Contains(joined, "0:s:1") {
			t.Fatalf("%s: the bitmap subtitle must be dropped, not mapped: %s", format, joined)
		}
		if got := flagValue(cmd.Args, "-c:a:0"); got != "copy" {
			t.Fatalf("%s: muxable audio must be copied, got -c:a:0 %q", format, got)
		}
		if got := flagValue(cmd.Args, "-c:a:1"); got != "aac" {
			t.Fatalf("%s: DTS must become an aac re-encode, got -c:a:1 %q", format, got)
		}
		if got := flagValue(cmd.Args, "-c:s"); got != "mov_text" {
			t.Fatalf("%s: text subtitles must convert to mov_text, got -c:s %q", format, got)
		}
	}
}

func TestBuildBatchConvertCmd_MP4CopyWithoutCodecInfoReencodesAudio(t *testing.T) {
	// No probe info means codecs cannot be classified as muxable; the safe
	// fallback is an AAC re-encode rather than a command that dies at the
	// muxer. Subtitles are not mapped (text vs bitmap is undecidable).
	cmd := BuildBatchConvertCmd("in.mkv", "out.mp4", "mp4", QualitySame, true, BatchStreamInfo{})
	if got := flagValue(cmd.Args, "-c:a"); got != "aac" {
		t.Fatalf("unknown audio must fall back to aac, got -c:a %q", got)
	}
	if strings.Contains(strings.Join(cmd.Args, " "), "0:s:") {
		t.Fatalf("subtitles must not be mapped without codec info: %s", strings.Join(cmd.Args, " "))
	}
}

func TestBuildBatchConvertCmd_AVICopyPresetReencodes(t *testing.T) {
	// AVI cannot carry subtitles and its tags reject modern codecs, so
	// "same quality" falls back to the container's classic codecs (the
	// webm precedent) instead of a doomed copy.
	cmd := BuildBatchConvertCmd("in.mkv", "out.avi", "avi", QualitySame, true,
		BatchStreamInfo{AudioCodecs: []string{"dts"}, SubtitleCodecs: []string{"subrip"}})
	if got := flagValue(cmd.Args, "-c:v"); got != "libx264" {
		t.Fatalf("avi copy preset must re-encode video, got -c:v %q", got)
	}
	if got := flagValue(cmd.Args, "-c:a"); got != "libmp3lame" {
		t.Fatalf("avi copy preset must re-encode audio to mp3, got -c:a %q", got)
	}
	for _, a := range cmd.Args {
		if a == "copy" {
			t.Fatalf("avi copy preset must not stream-copy: %#v", cmd.Args)
		}
	}
}

func TestBuildBatchConvertCmd_VideoQualityNoAudioDisablesAudio(t *testing.T) {
	for _, format := range []string{"mp4", "mov"} {
		cmd := BuildBatchConvertCmd("in.mkv", "out."+format, format, QualityMedium, false, BatchStreamInfo{})
		foundAn := false
		for _, a := range cmd.Args {
			if a == "-an" {
				foundAn = true
			}
		}
		if !foundAn {
			t.Fatalf("%s: expected -an, got %#v", format, cmd.Args)
		}
	}
}

// An explicit quality preset for the mp4 family is a deliberate re-encode:
// H.264 at the preset's CRF with yuv420p video and 192k AAC audio, for
// every preset level.
func TestBuildBatchConvertCmd_VideoQualityPresetsEncodeH264AAC(t *testing.T) {
	cases := []struct {
		quality BatchVideoQuality
		crf     string
	}{
		{QualityHigh, "18"},
		{QualityMedium, "23"},
		{QualityLow, "28"},
	}
	for _, format := range []string{"mp4", "mov"} {
		for _, c := range cases {
			cmd := BuildBatchConvertCmd("in.mkv", "out."+format, format, c.quality, true, BatchStreamInfo{})
			if got := flagValue(cmd.Args, "-c:v"); got != "libx264" {
				t.Fatalf("%s/%s: expected libx264, got -c:v %q", format, c.quality, got)
			}
			if got := flagValue(cmd.Args, "-crf"); got != c.crf {
				t.Fatalf("%s/%s: expected CRF %s, got %q", format, c.quality, c.crf, got)
			}
			if got := flagValue(cmd.Args, "-pix_fmt"); got != "yuv420p" {
				t.Fatalf("%s/%s: expected yuv420p, got -pix_fmt %q", format, c.quality, got)
			}
			if got := flagValue(cmd.Args, "-c:a"); got != "aac" {
				t.Fatalf("%s/%s: expected aac, got -c:a %q", format, c.quality, got)
			}
			if got := flagValue(cmd.Args, "-b:a"); got != "192k" {
				t.Fatalf("%s/%s: expected 192k audio, got -b:a %q", format, c.quality, got)
			}
		}
	}
}

// Audio-only batch targets drop the video and encode with the format's
// own encoder: mp3 via libmp3lame at 192k, flac natively and lossless
// (no bitrate).
func TestBuildBatchConvertCmd_MP3Target(t *testing.T) {
	cmd := BuildBatchConvertCmd("in.mp4", "out.mp3", "mp3", QualitySame, true, BatchStreamInfo{})
	foundVn := false
	for _, a := range cmd.Args {
		if a == "-vn" {
			foundVn = true
		}
	}
	if !foundVn {
		t.Fatalf("audio targets must drop the video with -vn, got %#v", cmd.Args)
	}
	if got := flagValue(cmd.Args, "-c:a"); got != "libmp3lame" {
		t.Fatalf("expected libmp3lame, got -c:a %q", got)
	}
	if got := flagValue(cmd.Args, "-b:a"); got != "192k" {
		t.Fatalf("expected 192k, got -b:a %q", got)
	}
}

func TestBuildBatchConvertCmd_FlacTarget(t *testing.T) {
	cmd := BuildBatchConvertCmd("in.mp4", "out.flac", "flac", QualitySame, true, BatchStreamInfo{})
	foundVn := false
	for _, a := range cmd.Args {
		if a == "-vn" {
			foundVn = true
		}
	}
	if !foundVn {
		t.Fatalf("audio targets must drop the video with -vn, got %#v", cmd.Args)
	}
	if got := flagValue(cmd.Args, "-c:a"); got != "flac" {
		t.Fatalf("expected the native flac encoder, got -c:a %q", got)
	}
	if got := flagValue(cmd.Args, "-b:a"); got != "" {
		t.Fatalf("flac is lossless and must carry no bitrate, got -b:a %q", got)
	}
}

func TestBuildBatchConvertCmd_WebmUsesVP9AndOpus(t *testing.T) {
	cmd := BuildBatchConvertCmd("in.mp4", "out.webm", "webm", QualityMedium, true, BatchStreamInfo{})
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
	cmd := BuildBatchConvertCmd("in.mp4", "out.webm", "webm", QualitySame, true, BatchStreamInfo{})
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
	cmd := BuildBatchConvertCmd("in.mp4", "out.webm", "webm", QualityLow, false, BatchStreamInfo{})
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
	cmd := BuildBatchConvertCmd("in.mp4", "out.wav", "wav", QualitySame, true, BatchStreamInfo{})
	if got := flagValue(cmd.Args, "-c:a"); got != "pcm_s16le" {
		t.Fatalf("expected pcm_s16le, got %q", got)
	}
}

func TestBuildGifPaletteCmd(t *testing.T) {
	// Step 1 of the 2-step GIF pipeline: build the palette from a
	// downscaled, fps-limited copy of the input.
	cmd := BuildGifPaletteCmd("in.mp4", "palette.png")
	want := []string{
		"-i", "in.mp4",
		"-vf", "fps=15,scale=480:-1:flags=lanczos,palettegen",
		"-y", "palette.png",
	}
	if got := cmd.Args; !equalArgs(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestBuildGifFromPaletteCmd(t *testing.T) {
	// Step 2: apply the palette through a filter_complex split/join graph.
	cmd := BuildGifFromPaletteCmd("in.mp4", "palette.png", "out.gif")
	want := []string{
		"-i", "in.mp4",
		"-i", "palette.png",
		"-filter_complex", "fps=15,scale=480:-1:flags=lanczos[x];[x][1:v]paletteuse",
		"-y", "out.gif",
	}
	if got := cmd.Args; !equalArgs(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}
