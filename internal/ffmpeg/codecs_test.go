package ffmpeg

import "testing"

func TestAudioCodecFor(t *testing.T) {
	cases := map[string]string{
		"mp3":  "libmp3lame",
		"wav":  "pcm_s16le",
		"flac": "flac",
	}
	for format, want := range cases {
		if got := AudioCodecFor(format); got != want {
			t.Errorf("AudioCodecFor(%q) = %q, want %q", format, got, want)
		}
	}
}

func TestCleanCodecChoice(t *testing.T) {
	cases := map[string]string{
		"libx264 (H.264)":      "libx264",
		"libmp3lame (MP3)":     "libmp3lame",
		"srt (SubRip)":         "srt",
		"mov_text (MP4)":       "mov_text",
		"(SubRip)":             "srt",
		"aac":                  "aac",
		"":                     "",
		"  libvpx-vp9 (VP9)  ": "libvpx-vp9",
	}
	for in, want := range cases {
		if got := CleanCodecChoice(in); got != want {
			t.Errorf("CleanCodecChoice(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeCodecForContainer(t *testing.T) {
	cases := []struct {
		name        string
		trackType   TrackType
		codec       string
		sourceCodec string
		outputPath  string
		want        string
	}{
		{"mp4 keeps mov_text copy", TrackSubtitle, "copy", "mov_text", "out.mp4", "copy"},
		{"mp4 converts srt copy to mov_text", TrackSubtitle, "copy", "subrip", "out.mp4", "mov_text"},
		{"mp4 converts srt choice to mov_text", TrackSubtitle, "srt", "subrip", "out.mp4", "mov_text"},
		{"mkv converts mov_text choice to srt", TrackSubtitle, "mov_text", "mov_text", "out.mkv", "srt"},
		{"mkv converts mov_text copy to srt", TrackSubtitle, "copy", "mov_text", "out.mkv", "srt"},
		{"mkv keeps srt", TrackSubtitle, "srt", "subrip", "out.mkv", "srt"},
		{"webm keeps vp9 copy", TrackVideo, "copy", "vp9", "out.webm", "copy"},
		{"webm re-encodes h264 copy", TrackVideo, "copy", "h264", "out.webm", "libvpx-vp9"},
		{"webm converts x264 choice", TrackVideo, "libx264", "h264", "out.webm", "libvpx-vp9"},
		{"webm keeps opus copy", TrackAudio, "copy", "opus", "out.webm", "copy"},
		{"webm converts aac copy", TrackAudio, "copy", "aac", "out.webm", "libopus"},
		{"webm drops subtitles", TrackSubtitle, "srt", "subrip", "out.webm", ""},
		{"mp4 drops pgs copy", TrackSubtitle, "copy", "hdmv_pgs_subtitle", "out.mp4", ""},
		{"mp4 drops pgs text choice", TrackSubtitle, "srt", "hdmv_pgs_subtitle", "out.mp4", ""},
		{"mp4 drops dvb copy", TrackSubtitle, "copy", "dvb_sub", "out.mov", ""},
		{"mp4 drops dvd text choice", TrackSubtitle, "mov_text", "dvd_subtitle", "out.m4v", ""},
		{"mkv keeps pgs copy", TrackSubtitle, "copy", "hdmv_pgs_subtitle", "out.mkv", "copy"},
		{"mkv drops pgs srt choice", TrackSubtitle, "srt", "hdmv_pgs_subtitle", "out.mkv", ""},
		{"mkv drops pgs mov_text choice", TrackSubtitle, "mov_text", "hdmv_pgs_subtitle", "out.mkv", ""},
		{"mkv drops dvb srt choice", TrackSubtitle, "srt", "dvb_sub", "out.mkv", ""},
		{"webm drops pgs", TrackSubtitle, "copy", "hdmv_pgs_subtitle", "out.webm", ""},
		{"mp4 converts vorbis choice", TrackAudio, "libvorbis", "vorbis", "out.mp4", "aac"},
		{"mp4 converts vorbis copy", TrackAudio, "copy", "vorbis", "out.mp4", "aac"},
		{"mp4 keeps aac copy", TrackAudio, "copy", "aac", "out.mp4", "copy"},
		{"mp4 re-encodes opus copy (no mux mapping)", TrackAudio, "copy", "opus", "out.mp4", "aac"},
		{"mp4 re-encodes opus copy into m4a", TrackAudio, "copy", "opus", "out.m4a", "aac"},
		{"mp4 re-encodes dts copy", TrackAudio, "copy", "dts", "out.mp4", "aac"},
		{"mp4 re-encodes truehd copy", TrackAudio, "copy", "truehd", "out.mov", "aac"},
		{"mp4 re-encodes pcm copy", TrackAudio, "copy", "pcm_s16le", "out.mp4", "aac"},
		{"mp4 keeps eac3 copy", TrackAudio, "copy", "eac3", "out.mp4", "copy"},
		{"mp4 re-encodes flv1 copy", TrackVideo, "copy", "flv1", "out.mp4", "libx264"},
		{"mp4 re-encodes theora copy", TrackVideo, "copy", "theora", "out.mov", "libx264"},
		{"mp4 keeps h264 copy", TrackVideo, "copy", "h264", "out.mp4", "copy"},
		{"unknown ext untouched", TrackVideo, "libx264", "h264", "out.bin", "libx264"},
		{"avi untouched", TrackSubtitle, "srt", "subrip", "out.avi", "srt"},
	}
	for _, c := range cases {
		if got := NormalizeCodecForContainer(c.trackType, c.codec, c.sourceCodec, c.outputPath); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestCodecOptionsAvoidUnusableEncoders(t *testing.T) {
	for _, opt := range CodecOptions(TrackAudio) {
		switch CleanCodecChoice(opt) {
		case "libflac":
			t.Errorf("libflac encoder no longer exists in ffmpeg: %q", opt)
		case "libfdk_aac":
			t.Errorf("libfdk_aac requires nonfree builds and is unavailable to most users: %q", opt)
		}
	}
}
