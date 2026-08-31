//go:build integration

package integration

import (
	"path/filepath"
	"testing"
	"time"

	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
)

func TestEffectSpeedDoublesDuration(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "basic_2x.mp4")
	cmd := ffx.BuildSpeedCmd(fx(t, "basic.mp4"), 2, true, 1, out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	assertDuration(t, res, 10, 1)
	if got := len(streamsOfType(res, "audio")); got != 1 {
		t.Fatalf("want 1 audio stream, got %d", got)
	}
	if v := firstStream(t, res, "video"); v.CodecName != "h264" {
		t.Fatalf("video codec = %s, want h264", v.CodecName)
	}
}

// Speed must keep every audio layer of a multi-track file: the filter
// used to bind [0:a] (first track only) and map just one output.
func TestEffectSpeed2xKeepsAllAudioTracks(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "hdr4k_2x.mp4")
	cmd := ffx.BuildSpeedCmd(fx(t, "hdr4k.mkv"), 2, true, 3, out)
	runFFmpeg(t, 6*time.Minute, cmd.Args)

	res := probeFile(t, out)
	assertDuration(t, res, 2, 0.5)
	audios := streamsOfType(res, "audio")
	if len(audios) != 3 {
		t.Fatalf("all 3 audio layers must survive the speed change, got %d", len(audios))
	}
	for i, a := range audios {
		if a.CodecName != "aac" {
			t.Fatalf("audio %d = %s, want aac (atempo re-encode)", i, a.CodecName)
		}
	}
	if v := firstStream(t, res, "video"); v.CodecName != "h264" {
		t.Fatalf("video codec = %s, want h264", v.CodecName)
	}
}

func TestEffectSpeedHalfOnVideoOnlyInput(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "noaudio_0.5x.mp4")
	cmd := ffx.BuildSpeedCmd(fx(t, "noaudio.mp4"), 0.5, false, 0, out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	assertDuration(t, res, 12, 1)
	if got := len(streamsOfType(res, "audio")); got != 0 {
		t.Fatalf("video-only input must stay video-only, audio streams = %d", got)
	}
	if v := firstStream(t, res, "video"); v.CodecName != "h264" {
		t.Fatalf("video codec = %s, want h264", v.CodecName)
	}
}

// 4x sits at the top of the wizard's factor range and forces a two-element
// atempo chain (0.25-4x beyond atempo's 0.5-2.0 per-element limit): the
// output must play in a quarter of the time with audio intact.
func TestEffectSpeed4xChainsAtempo(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "basic_4x.mp4")
	cmd := ffx.BuildSpeedCmd(fx(t, "basic.mp4"), 4, true, 1, out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	assertDuration(t, res, 5, 0.75)
	if got := len(streamsOfType(res, "audio")); got != 1 {
		t.Fatalf("chained atempo must keep the audio track, got %d", got)
	}
	if a := firstStream(t, res, "audio"); a.CodecName != "aac" {
		t.Fatalf("audio codec = %s, want aac (atempo re-encode)", a.CodecName)
	}
}

func TestEffectReverseKeepsDurationAndAudio(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "basic_reversed.mp4")
	cmd := ffx.BuildReverseCmd(fx(t, "basic.mp4"), true, 1, out)
	runFFmpeg(t, 4*time.Minute, cmd.Args)

	res := probeFile(t, out)
	assertDuration(t, res, 20, 1)
	if got := len(streamsOfType(res, "audio")); got != 1 {
		t.Fatalf("audio must survive reversing, audio streams = %d", got)
	}
	v := firstStream(t, res, "video")
	if v.CodecName != "h264" {
		t.Fatalf("video codec = %s, want h264", v.CodecName)
	}
	if v.PixFmt != "yuv420p" {
		t.Fatalf("pix_fmt = %s, want yuv420p", v.PixFmt)
	}
}

// Reversing a multi-track file must keep every audio layer.
func TestEffectReverseKeepsAllAudioTracks(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "hdr4k_reversed.mp4")
	cmd := ffx.BuildReverseCmd(fx(t, "hdr4k.mkv"), true, 3, out)
	runFFmpeg(t, 6*time.Minute, cmd.Args)

	res := probeFile(t, out)
	audios := streamsOfType(res, "audio")
	if len(audios) != 3 {
		t.Fatalf("all 3 audio layers must survive reversing, got %d", len(audios))
	}
	for i, a := range audios {
		if a.CodecName != "aac" {
			t.Fatalf("audio %d = %s, want aac (areverse re-encode)", i, a.CodecName)
		}
	}
	assertDuration(t, res, 4, 1)
}

// Reversing a video-only file must stay audio-free and keep the source
// duration (the reverse filter alone, no areverse branch).
func TestEffectReverseWithoutAudio(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "noaudio_reversed.mp4")
	cmd := ffx.BuildReverseCmd(fx(t, "noaudio.mp4"), false, 0, out)
	runFFmpeg(t, 3*time.Minute, cmd.Args)

	res := probeFile(t, out)
	if got := len(streamsOfType(res, "audio")); got != 0 {
		t.Fatalf("video-only reverse must stay audio-free, got %d audio streams", got)
	}
	if v := firstStream(t, res, "video"); v.CodecName != "h264" {
		t.Fatalf("video codec = %s, want h264", v.CodecName)
	}
	assertDuration(t, res, 6, 1)
}

func TestEffectMuteRemovesAudioLosslessly(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "basic_muted.mp4")
	cmd := ffx.BuildMuteCmd(fx(t, "basic.mp4"), out)
	runFFmpeg(t, 2*time.Minute, cmd.Args)

	res := probeFile(t, out)
	assertDuration(t, res, 20, 1)
	if got := len(streamsOfType(res, "audio")); got != 0 {
		t.Fatalf("mute must remove every audio stream, got %d", got)
	}
	if v := firstStream(t, res, "video"); v.CodecName != "h264" {
		t.Fatalf("mute must stream-copy the video, codec = %s", v.CodecName)
	}
}

// Muting removes the audio layers but must keep every video and subtitle
// stream: plain -an with default stream selection would drop all but the
// "best" subtitle track.
func TestEffectMuteKeepsSubtitlesAndVideo(t *testing.T) {
	requireTools(t)
	out := filepath.Join(t.TempDir(), "hdr4k_muted.mkv")
	cmd := ffx.BuildMuteCmd(fx(t, "hdr4k.mkv"), out)
	runFFmpeg(t, 2*time.Minute, cmd.Args)

	res := probeFile(t, out)
	if got := len(streamsOfType(res, "audio")); got != 0 {
		t.Fatalf("mute must remove all audio, got %d streams", got)
	}
	if got := len(streamsOfType(res, "video")); got != 1 {
		t.Fatalf("mute must keep the video stream, got %d", got)
	}
	if got := len(streamsOfType(res, "subtitle")); got != 4 {
		t.Fatalf("mute must keep all 4 subtitle streams, got %d", got)
	}
	assertDuration(t, res, 4, 0.5)
}
