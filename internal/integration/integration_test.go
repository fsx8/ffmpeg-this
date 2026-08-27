//go:build integration

package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fsx8/ffwiz/internal/execx"
	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
	ffprobe "github.com/fsx8/ffwiz/internal/ffprobe"
)

var runner = execx.New()

var prober = ffprobe.New(execx.New())

func requireTools(t *testing.T) {
	t.Helper()
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("%s not in PATH: %v", name, err)
		}
	}
}

func fixtures(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("FFWIZ_TESTMEDIA")
	if dir == "" {
		dir = filepath.Join("..", "..", "testmedia")
	}
	if _, err := os.Stat(filepath.Join(dir, "basic.mp4")); err != nil {
		t.Skipf("fixtures missing in %s — run scripts/testmedia.sh first", dir)
	}
	return dir
}

func fx(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(fixtures(t), name)
}

func runFFmpeg(t *testing.T, timeout time.Duration, args []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, stderr, err := runner.Run(ctx, execx.Cmd{Name: "ffmpeg", Args: args})
	if err != nil {
		t.Fatalf("ffmpeg failed: %v\n%s", err, stderr)
	}
}

func probeFile(t *testing.T, path string) *ffprobe.ProbeResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := prober.Probe(ctx, path)
	if err != nil {
		t.Fatalf("ffprobe failed for %s: %v", path, err)
	}
	return res
}

func streamsOfType(res *ffprobe.ProbeResult, typ string) []ffprobe.Stream {
	var out []ffprobe.Stream
	for _, s := range res.Streams {
		if s.CodecType == typ {
			out = append(out, s)
		}
	}
	return out
}

func firstStream(t *testing.T, res *ffprobe.ProbeResult, typ string) ffprobe.Stream {
	t.Helper()
	ss := streamsOfType(res, typ)
	if len(ss) == 0 {
		t.Fatalf("no %s stream in probe result: %+v", typ, res.Streams)
	}
	return ss[0]
}

func durationOf(t *testing.T, res *ffprobe.ProbeResult) float64 {
	t.Helper()
	d, err := strconv.ParseFloat(strings.TrimSpace(res.Format.Duration), 64)
	if err != nil {
		t.Fatalf("bad duration %q: %v", res.Format.Duration, err)
	}
	return d
}

func assertDuration(t *testing.T, res *ffprobe.ProbeResult, want float64, tol float64) {
	t.Helper()
	got := durationOf(t, res)
	if got < want-tol || got > want+tol {
		t.Fatalf("duration = %.2fs, want %.2fs ±%.2f", got, want, tol)
	}
}

func TestProbeFixtureExpectations(t *testing.T) {
	requireTools(t)
	cases := []struct {
		file      string
		video     string
		audio     string
		width     int
		sampleHz  string
		extraName string
	}{
		{"basic.mp4", "h264", "aac", 640, "48000", ""},
		{"noaudio.mp4", "h264", "", 640, "", ""},
		{"audio_only.mp3", "", "mp3", 0, "44100", ""},
		{"audio_only.flac", "", "flac", 0, "48000", ""},
		{"audio_only.wav", "", "pcm_s16le", 0, "48000", ""},
		{"multiaudio.mkv", "h264", "aac", 640, "48000", ""},
		{"subs.mkv", "h264", "aac", 640, "48000", "subrip"},
		{"hevc.mkv", "hevc", "", 640, "", ""},
		{"vp9.webm", "vp9", "opus", 640, "48000", ""},
		{"mpeg4.avi", "mpeg4", "mp3", 640, "44100", ""},
		{"join_a.mp4", "h264", "aac", 640, "44100", ""},
		{"join_b.mp4", "h264", "aac", 1280, "48000", ""},
		{"long.mp4", "h264", "aac", 320, "48000", ""},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			res := probeFile(t, fx(t, c.file))
			if c.video != "" {
				v := firstStream(t, res, "video")
				if v.CodecName != c.video || v.Width != c.width {
					t.Fatalf("video = %s %dx%d, want %s w=%d", v.CodecName, v.Width, v.Height, c.video, c.width)
				}
			} else if got := len(streamsOfType(res, "video")); got != 0 {
				t.Fatalf("expected no video stream, got %d", got)
			}
			if c.audio != "" {
				a := firstStream(t, res, "audio")
				if a.CodecName != c.audio || a.SampleRate != c.sampleHz {
					t.Fatalf("audio = %s %s Hz, want %s %s Hz", a.CodecName, a.SampleRate, c.audio, c.sampleHz)
				}
			} else if got := len(streamsOfType(res, "audio")); got != 0 {
				t.Fatalf("expected no audio stream, got %d", got)
			}
			if c.extraName != "" {
				s := firstStream(t, res, "subtitle")
				if s.CodecName != c.extraName {
					t.Fatalf("subtitle = %s, want %s", s.CodecName, c.extraName)
				}
			}
		})
	}
}

func TestProbeMultiaudioLanguageTags(t *testing.T) {
	requireTools(t)
	res := probeFile(t, fx(t, "multiaudio.mkv"))
	audios := streamsOfType(res, "audio")
	if len(audios) != 2 {
		t.Fatalf("want 2 audio streams, got %d", len(audios))
	}
	for i, want := range []string{"eng", "deu"} {
		got := audios[i].Tags["language"]
		if got != want {
			t.Fatalf("audio %d language = %q, want %q", i, got, want)
		}
	}
}

func streamDuration(t *testing.T, res *ffprobe.ProbeResult, typ string) float64 {
	t.Helper()
	s := firstStream(t, res, typ)
	d, err := strconv.ParseFloat(strings.TrimSpace(s.Duration), 64)
	if err != nil {
		t.Fatalf("bad %s stream duration %q: %v", typ, s.Duration, err)
	}
	return d
}

// Keyframe-aligned lossless trim must reproduce the exact window.
// The fixtures use a 2 s GOP, so 4→8 starts on a keyframe.
func TestTrimKeyframeAlignedIsExact(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "trimmed.mp4")
	cmd := ffx.BuildTrimCmd(fx(t, "basic.mp4"), "00:00:04", "00:00:08", out)
	runFFmpeg(t, 2*time.Minute, cmd.Args)

	res := probeFile(t, out)
	assertDuration(t, res, 4, 0.5)
	if v := firstStream(t, res, "video"); v.CodecName != "h264" {
		t.Fatalf("trim must not re-encode, video codec = %s", v.CodecName)
	}
	if a := firstStream(t, res, "audio"); a.CodecName != "aac" {
		t.Fatalf("trim must not re-encode audio, codec = %s", a.CodecName)
	}
	if vd := streamDuration(t, res, "video"); vd < 3.5 {
		t.Fatalf("video stream duration = %.2fs, want ~4", vd)
	}
}

// Mid-GOP trims are snapped back to the previous keyframe: the video stream
// never comes up short or empty, and audio/video durations stay in sync.
// basic.mp4 has a 2 s GOP, so 5→9 snaps to 4→9.
func TestTrimMidGOPSnapsToPreviousKeyframe(t *testing.T) {
	requireTools(t)
	in := fx(t, "basic.mp4")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	kf, err := prober.Keyframes(ctx, in)
	if err != nil {
		t.Fatalf("keyframe probe failed: %v", err)
	}
	if len(kf) < 5 {
		t.Fatalf("fixture should expose several keyframes, got %v", kf)
	}

	snapped := ffx.SnapToKeyframe(5, kf)
	if snapped != 4 {
		t.Fatalf("snap(5) = %v, want 4", snapped)
	}
	out := filepath.Join(t.TempDir(), "trimmed.mp4")
	cmd := ffx.BuildTrimCmd(in, ffx.FormatTimeSpec(snapped), "00:00:09", out)
	runFFmpeg(t, 2*time.Minute, cmd.Args)

	res := probeFile(t, out)
	if got := len(streamsOfType(res, "video")); got != 1 {
		t.Fatalf("video stream must not vanish, got %d streams: %+v", got, res.Streams)
	}
	ad := streamDuration(t, res, "audio")
	vd := streamDuration(t, res, "video")
	if ad < 4.5 || ad > 5.5 {
		t.Fatalf("audio duration = %.2fs, want ~5 (4→9)", ad)
	}
	if diff := ad - vd; diff < -0.05 || diff > 0.3 {
		t.Fatalf("video must cover the whole snapped window: audio %.2fs vs video %.2fs", ad, vd)
	}
}

func TestKeyframesProbeMatchesFixtureGOP(t *testing.T) {
	requireTools(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	kf, err := prober.Keyframes(ctx, fx(t, "basic.mp4"))
	if err != nil {
		t.Fatalf("keyframe probe failed: %v", err)
	}
	if len(kf) != 10 {
		t.Fatalf("basic.mp4 (20s, 2s GOP) should have keyframes 0..18s, got %v", kf)
	}
	for i, want := 0, 0.0; i < len(kf); i, want = i+1, want+2 {
		if kf[i] < want-0.1 || kf[i] > want+0.1 {
			t.Fatalf("keyframe %d = %v, want ~%v (full set: %v)", i, kf[i], want, kf)
		}
	}
}

func TestProbeHDR4KFeatureFixture(t *testing.T) {
	requireTools(t)
	res := probeFile(t, fx(t, "hdr4k.mkv"))

	v := firstStream(t, res, "video")
	if v.CodecName != "hevc" || v.Width != 3840 || v.Height != 2160 {
		t.Fatalf("video = %s %dx%d, want hevc 3840x2160", v.CodecName, v.Width, v.Height)
	}
	if v.PixFmt != "yuv420p10le" {
		t.Fatalf("pix_fmt = %s, want yuv420p10le (10-bit)", v.PixFmt)
	}
	if v.ColorTransfer != "smpte2084" || v.ColorPrimaries != "bt2020" {
		t.Fatalf("HDR10 signaling missing: trc=%s primaries=%s", v.ColorTransfer, v.ColorPrimaries)
	}

	audios := streamsOfType(res, "audio")
	if len(audios) != 3 {
		t.Fatalf("want 3 audio layers, got %d", len(audios))
	}
	wantAudio := []struct {
		codec string
		ch    int
		lang  string
		title string
	}{{"dts", 6, "eng", "DTS 5.1"}, {"eac3", 6, "deu", "EAC3 5.1"}, {"aac", 2, "eng", "Commentary"}}
	for i, want := range wantAudio {
		a := audios[i]
		if a.CodecName != want.codec || a.Channels != want.ch {
			t.Fatalf("audio %d = %s %dch, want %s %dch", i, a.CodecName, a.Channels, want.codec, want.ch)
		}
		if a.Tags["language"] != want.lang || a.Tags["title"] != want.title {
			t.Fatalf("audio %d tags = %q/%q, want %q/%q", i, a.Tags["language"], a.Tags["title"], want.lang, want.title)
		}
	}

	subs := streamsOfType(res, "subtitle")
	if len(subs) != 4 {
		t.Fatalf("want 4 subtitle tracks, got %d", len(subs))
	}
	wantSubs := []struct {
		codec string
		lang  string
	}{{"subrip", "eng"}, {"subrip", "deu"}, {"subrip", "spa"}, {"ass", "eng"}}
	for i, want := range wantSubs {
		if subs[i].CodecName != want.codec || subs[i].Tags["language"] != want.lang {
			t.Fatalf("subtitle %d = %s %q, want %s %q", i, subs[i].CodecName, subs[i].Tags["language"], want.codec, want.lang)
		}
	}
}

// The flagship real-world flow: a DTS-family track converted to EAC3 for
// device compatibility while the 4K HDR video, the other audio layers and
// all subtitle tracks pass through untouched.
func TestDTSAudioConvertsToEAC3(t *testing.T) {
	requireTools(t)
	in := fx(t, "hdr4k.mkv")
	res := probeFile(t, in)

	dtsIndex := -1
	for _, s := range res.Streams {
		if s.CodecType == "audio" && s.CodecName == "dts" {
			dtsIndex = s.Index
		}
	}
	if dtsIndex < 0 {
		t.Fatal("fixture lacks the DTS audio track")
	}

	out := filepath.Join(t.TempDir(), "converted.mkv")
	cmd := ffx.BuildInteractiveConvertCmd(in, out, tracksFromProbe(res),
		map[int]ffx.TrackActionInfo{dtsIndex: {Action: ffx.ActionConvert, Codec: "eac3"}})
	if cmd == nil {
		t.Fatal("expected a command")
	}
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	outRes := probeFile(t, out)
	if v := firstStream(t, outRes, "video"); v.CodecName != "hevc" || v.PixFmt != "yuv420p10le" {
		t.Fatalf("4K HDR video must pass through untouched, got %s %s", v.CodecName, v.PixFmt)
	}
	audios := streamsOfType(outRes, "audio")
	if len(audios) != 3 {
		t.Fatalf("all 3 audio layers must survive, got %d", len(audios))
	}
	converted := audios[0]
	if converted.CodecName != "eac3" {
		t.Fatalf("DTS track must convert to eac3, got %s", converted.CodecName)
	}
	if converted.Channels != 6 {
		t.Fatalf("5.1 layout must be preserved through conversion, got %d channels", converted.Channels)
	}
	if audios[1].CodecName != "eac3" || audios[2].CodecName != "aac" {
		t.Fatalf("untouched layers must stay eac3/aac, got %s/%s", audios[1].CodecName, audios[2].CodecName)
	}
	if got := len(streamsOfType(outRes, "subtitle")); got != 4 {
		t.Fatalf("all 4 subtitle tracks must survive, got %d", got)
	}
}

func TestExtractAudioFormats(t *testing.T) {
	requireTools(t)
	cases := map[string]string{"mp3": "mp3", "flac": "flac", "wav": "pcm_s16le"}
	for format, codec := range cases {
		t.Run(format, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "audio."+format)
			cmd := ffx.BuildExtractAudioCmd(fx(t, "basic.mp4"), format, out)
			runFFmpeg(t, 2*time.Minute, cmd.Args)
			res := probeFile(t, out)
			if got := len(streamsOfType(res, "video")); got != 0 {
				t.Fatalf("extracted audio must have no video stream, got %d", got)
			}
			a := firstStream(t, res, "audio")
			if a.CodecName != codec {
				t.Fatalf("audio codec = %s, want %s", a.CodecName, codec)
			}
			assertDuration(t, res, 20, 1)
		})
	}
}

func joinInput(t *testing.T, path string) ffx.JoinInput {
	t.Helper()
	res := probeFile(t, path)
	hasAudio := len(streamsOfType(res, "audio")) > 0
	return ffx.JoinInput{Path: path, HasAudio: hasAudio, DurationSec: durationOf(t, res)}
}

func joinTargetFromFirst(t *testing.T, path string) ffx.JoinTargets {
	t.Helper()
	res := probeFile(t, path)
	v := firstStream(t, res, "video")
	target := ffx.JoinTargets{Width: v.Width, Height: v.Height, SAR: v.SampleAspectRatio}
	if target.SAR == "" || target.SAR == "0:1" {
		target.SAR = "1:1"
	}
	if a := streamsOfType(res, "audio"); len(a) > 0 {
		target.SampleRate = a[0].SampleRate
	}
	return target
}

func TestJoinNormalizesMixedInputs(t *testing.T) {
	requireTools(t)
	a, b := fx(t, "join_a.mp4"), fx(t, "join_b.mp4")
	out := filepath.Join(t.TempDir(), "joined.mp4")
	cmd := ffx.BuildJoinCmd([]ffx.JoinInput{joinInput(t, a), joinInput(t, b)}, out, joinTargetFromFirst(t, a))
	runFFmpeg(t, 4*time.Minute, cmd.Args)

	res := probeFile(t, out)
	assertDuration(t, res, 10, 1)
	v := firstStream(t, res, "video")
	if v.Width != 640 || v.Height != 480 {
		t.Fatalf("join must target first input geometry, got %dx%d", v.Width, v.Height)
	}
	aS := firstStream(t, res, "audio")
	if aS.SampleRate != "44100" {
		t.Fatalf("join must target first input sample rate, got %s", aS.SampleRate)
	}
	if aS.Channels != 2 {
		t.Fatalf("join must normalize to stereo, got %d channels", aS.Channels)
	}
}

func TestJoinSynthesizesSilenceForVideoOnlyInput(t *testing.T) {
	requireTools(t)
	a, silent := fx(t, "join_a.mp4"), fx(t, "noaudio.mp4")
	out := filepath.Join(t.TempDir(), "joined.mp4")
	cmd := ffx.BuildJoinCmd([]ffx.JoinInput{joinInput(t, a), joinInput(t, silent)}, out, joinTargetFromFirst(t, a))
	runFFmpeg(t, 4*time.Minute, cmd.Args)

	res := probeFile(t, out)
	assertDuration(t, res, 11, 1)
	if got := len(streamsOfType(res, "audio")); got != 1 {
		t.Fatalf("silence must be synthesized, audio streams = %d", got)
	}
}

func TestJoinVideoOnlyInputs(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "joined.mp4")
	ins := []ffx.JoinInput{joinInput(t, fx(t, "noaudio.mp4")), joinInput(t, fx(t, "noaudio.mp4"))}
	cmd := ffx.BuildJoinCmd(ins, out, joinTargetFromFirst(t, ins[0].Path))
	runFFmpeg(t, 4*time.Minute, cmd.Args)

	res := probeFile(t, out)
	assertDuration(t, res, 12, 1)
	if got := len(streamsOfType(res, "audio")); got != 0 {
		t.Fatalf("video-only join must stay video-only, audio streams = %d", got)
	}
}

func tracksFromProbe(res *ffprobe.ProbeResult) []ffx.Track {
	var tracks []ffx.Track
	for _, s := range res.Streams {
		tracks = append(tracks, ffx.Track{Index: s.Index, Type: ffx.TrackType(s.CodecType), Codec: s.CodecName})
	}
	return tracks
}

func TestInteractiveConvertRemovesTrack(t *testing.T) {
	requireTools(t)
	in := fx(t, "multiaudio.mkv")
	res := probeFile(t, in)
	secondAudio := -1
	for _, s := range res.Streams {
		if s.CodecType == "audio" && s.Tags["language"] == "deu" {
			secondAudio = s.Index
		}
	}
	if secondAudio < 0 {
		t.Fatal("fixture lacks the deu audio track")
	}
	out := filepath.Join(t.TempDir(), "modified.mkv")
	cmd := ffx.BuildInteractiveConvertCmd(in, out, tracksFromProbe(res),
		map[int]ffx.TrackActionInfo{secondAudio: {Action: ffx.ActionRemove}})
	if cmd == nil {
		t.Fatal("expected a command")
	}
	runFFmpeg(t, 2*time.Minute, cmd.Args)

	outRes := probeFile(t, out)
	if got := len(streamsOfType(outRes, "audio")); got != 1 {
		t.Fatalf("want 1 audio stream after removal, got %d", got)
	}
	if firstStream(t, outRes, "audio").Tags["language"] != "eng" {
		t.Fatal("removed the wrong audio track")
	}
}

func TestInteractiveConvertToWebm(t *testing.T) {
	requireTools(t)
	in := fx(t, "hevc.mkv")
	res := probeFile(t, in)
	out := filepath.Join(t.TempDir(), "converted.webm")
	cmd := ffx.BuildInteractiveConvertCmd(in, out, tracksFromProbe(res),
		map[int]ffx.TrackActionInfo{0: {Action: ffx.ActionConvert, Codec: "libvpx-vp9"}})
	if cmd == nil {
		t.Fatal("expected a command")
	}
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	if v := firstStream(t, probeFile(t, out), "video"); v.CodecName != "vp9" {
		t.Fatalf("video codec = %s, want vp9", v.CodecName)
	}
}

func TestInteractiveConvertSubtitleToMovText(t *testing.T) {
	requireTools(t)
	in := fx(t, "subs.mkv")
	res := probeFile(t, in)
	out := filepath.Join(t.TempDir(), "modified.mp4")
	cmd := ffx.BuildInteractiveConvertCmd(in, out, tracksFromProbe(res),
		map[int]ffx.TrackActionInfo{2: {Action: ffx.ActionConvert, Codec: "mov_text"}})
	if cmd == nil {
		t.Fatal("expected a command")
	}
	runFFmpeg(t, 2*time.Minute, cmd.Args)

	if s := firstStream(t, probeFile(t, out), "subtitle"); s.CodecName != "mov_text" {
		t.Fatalf("subtitle codec = %s, want mov_text", s.CodecName)
	}
}

func TestBatchConvertStreamCopy(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "copy_batch.mkv")
	cmd := ffx.BuildBatchConvertCmd(fx(t, "basic.mp4"), out, "mkv", ffx.QualitySame, true)
	runFFmpeg(t, 2*time.Minute, cmd.Args)

	res := probeFile(t, out)
	assertDuration(t, res, 20, 1)
	if v := firstStream(t, res, "video"); v.CodecName != "h264" {
		t.Fatalf("stream copy expected, video = %s", v.CodecName)
	}
	if a := firstStream(t, res, "audio"); a.CodecName != "aac" {
		t.Fatalf("stream copy expected, audio = %s", a.CodecName)
	}
}

func TestBatchConvertWebmReencodesToVP9(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "noaudio_batch.webm")
	cmd := ffx.BuildBatchConvertCmd(fx(t, "noaudio.mp4"), out, "webm", ffx.QualityLow, false)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	if v := firstStream(t, res, "video"); v.CodecName != "vp9" {
		t.Fatalf("webm batch must re-encode to vp9, got %s", v.CodecName)
	}
	if got := len(streamsOfType(res, "audio")); got != 0 {
		t.Fatalf("-an expected for audio-less input, got %d audio streams", got)
	}
}

func TestBatchGifPipeline(t *testing.T) {
	requireTools(t)
	dir := t.TempDir()
	palette := filepath.Join(dir, "palette.png")
	out := filepath.Join(dir, "out.gif")
	runFFmpeg(t, 2*time.Minute, ffx.BuildGifPaletteCmd(fx(t, "noaudio.mp4"), palette).Args)
	runFFmpeg(t, 2*time.Minute, ffx.BuildGifFromPaletteCmd(fx(t, "noaudio.mp4"), palette, out).Args)

	res := probeFile(t, out)
	v := firstStream(t, res, "video")
	if v.CodecName != "gif" {
		t.Fatalf("codec = %s, want gif", v.CodecName)
	}
	if v.RFrameRate != "15/1" {
		t.Fatalf("gif fps = %s, want 15/1", v.RFrameRate)
	}
}

func TestProgressStreamEndToEnd(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "reencoded.mp4")
	base := []string{
		"-i", fx(t, "long.mp4"),
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "35",
		"-c:a", "aac", "-b:a", "64k",
		"-y", out,
	}
	cmd := execx.Cmd{Name: "ffmpeg", Args: ffx.AddProgressArgs(base)}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	var tracker ffx.ProgressTracker
	progressLines := 0
	stderrLines := 0
	exitCode, err := runner.RunStreaming(ctx, cmd,
		func(line string) {
			if _, ok := tracker.Observe(line); ok {
				progressLines++
			}
		},
		func(line string) {
			if line != "" {
				stderrLines++
			}
		},
	)
	if err != nil {
		t.Fatalf("ffmpeg failed (exit %d): %v", exitCode, err)
	}
	if progressLines == 0 {
		t.Fatal("no -progress lines were parsed from stdout")
	}
	if stderrLines == 0 {
		t.Fatal("no stderr lines were streamed")
	}
	s := tracker.Sample()
	if !s.Done {
		t.Fatal("progress=end marker was not observed")
	}
	if s.OutTime < 55*time.Second {
		t.Fatalf("processed time = %v, want >= 55s of the 60s input", s.OutTime)
	}
	if p := s.Percent(60 * time.Second); p < 0.99 {
		t.Fatalf("final percent = %.2f, want ~1.0", p)
	}
}
