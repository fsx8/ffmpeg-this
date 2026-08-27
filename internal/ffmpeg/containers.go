package ffmpeg

import (
	"path/filepath"
	"strings"
)

var (
	webmVideoCodecs  = map[string]bool{"vp8": true, "vp9": true, "av1": true, "libvpx": true, "libvpx-vp9": true, "libaom-av1": true}
	webmAudioCodecs  = map[string]bool{"vorbis": true, "libvorbis": true, "opus": true, "libopus": true}
	mp4FamilyVideo   = map[string]bool{"copy": true, "libx264": true, "libx265": true, "libvpx-vp9": true, "libaom-av1": true}
	streamableSubsMP = map[string]bool{"mov_text": true}
)

// NormalizeCodecForContainer adjusts a resolved codec choice ("copy" when
// keeping) so that the combination with the output container is accepted by
// ffmpeg's muxers. The interactive convert wizard picks codecs before the
// output name is known, so this runs at command build time; the on-screen
// command preview always shows the final, normalized command.
//
// An empty return value means the stream cannot be muxed into the container
// at all (e.g. subtitles into webm) and must be dropped.
//
// Unknown extensions (including .avi, which has very limited stream support)
// are left untouched.
func NormalizeCodecForContainer(trackType TrackType, codec, sourceCodec, outputPath string) string {
	ext := strings.ToLower(filepath.Ext(outputPath))

	switch ext {
	case ".mp4", ".mov", ".m4a", ".m4v":
		switch trackType {
		case TrackVideo:
			if codec == "copy" || mp4FamilyVideo[codec] {
				return codec
			}
			return "libx264"
		case TrackAudio:
			// Vorbis has no MP4 muxer support; everything else common works.
			if codec == "libvorbis" || (codec == "copy" && sourceCodec == "vorbis") {
				return "aac"
			}
			return codec
		case TrackSubtitle:
			// MP4-family containers only carry mov_text subtitles.
			if codec == "copy" && sourceCodec == "mov_text" {
				return "copy"
			}
			return "mov_text"
		}
	case ".mkv":
		if trackType == TrackSubtitle {
			// Matroska does not accept mov_text; convert it to srt.
			if codec == "mov_text" || (codec == "copy" && sourceCodec == "mov_text") {
				return "srt"
			}
		}
		return codec
	case ".webm":
		switch trackType {
		case TrackVideo:
			if codec == "copy" {
				if webmVideoCodecs[sourceCodec] {
					return "copy"
				}
				return "libvpx-vp9"
			}
			if webmVideoCodecs[codec] {
				return codec
			}
			return "libvpx-vp9"
		case TrackAudio:
			if codec == "copy" {
				if webmAudioCodecs[sourceCodec] {
					return "copy"
				}
				return "libopus"
			}
			if webmAudioCodecs[codec] {
				return codec
			}
			return "libopus"
		case TrackSubtitle:
			// webm cannot carry subtitle streams at all.
			return ""
		}
	}
	return codec
}
