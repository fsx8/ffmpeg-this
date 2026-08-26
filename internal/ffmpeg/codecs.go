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
			"libfdk_aac (AAC)",
			"libopus (Opus)",
			"libflac (FLAC)",
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

func DefaultCodec(trackType TrackType) string {
	switch trackType {
	case TrackVideo:
		return "libx264 (H.264)"
	case TrackAudio:
		return "aac"
	case TrackSubtitle:
		return "srt (SubRip)"
	default:
		return "copy"
	}
}
