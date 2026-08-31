package ffmpeg

func CodecOptions(trackType TrackType) []string {
	switch trackType {
	case TrackVideo:
		return []string{
			"libx264 (H.264)",
			"libx265 (H.265/HEVC)",
			"libvpx-vp9 (VP9)",
			"libaom-av1 (AV1)",
			"libvpx (VP8)",
		}
	case TrackAudio:
		return []string{
			"aac",
			"eac3",
			"libmp3lame (MP3)",
			"libopus (Opus)",
			"flac (FLAC)",
			"libvorbis (Vorbis)",
		}
	case TrackSubtitle:
		return []string{
			"srt (SubRip)",
			"ass (ASS)",
			"mov_text (MP4)",
		}
	default:
		return nil
	}
}

// AudioCodecFor maps a user-facing audio format to an ffmpeg encoder name.
// mp3 and wav are containers/formats, not encoders: mp3 encodes via
// libmp3lame and wav via signed 16-bit PCM. flac and others use the native
// encoder of the same name (the external libflac encoder was removed from
// ffmpeg long ago).
func AudioCodecFor(format string) string {
	switch format {
	case "mp3":
		return "libmp3lame"
	case "wav":
		return "pcm_s16le"
	default:
		return format
	}
}
