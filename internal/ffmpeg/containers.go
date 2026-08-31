package ffmpeg

import (
	"path/filepath"
	"strings"
)

var (
	webmVideoCodecs = map[string]bool{"vp8": true, "vp9": true, "av1": true, "libvpx": true, "libvpx-vp9": true, "libaom-av1": true}
	webmAudioCodecs = map[string]bool{"vorbis": true, "libvorbis": true, "opus": true, "libopus": true}
	mp4FamilyVideo  = map[string]bool{"copy": true, "libx264": true, "libx265": true, "libvpx-vp9": true, "libaom-av1": true}

	// mp4FamilyNoAudioCopy lists source codecs that a plain stream copy
	// cannot mux into the MP4 family (opus has no standard MP4 mapping in
	// ffmpeg <= 5, DTS/TrueHD/PCM are not supported by the muxer), so
	// "keep" must quietly become an AAC re-encode instead of failing at
	// the muxing stage with a cryptic error.
	mp4FamilyNoAudioCopy = map[string]bool{
		"opus": true, "dts": true, "truehd": true,
		"pcm_s16le": true, "pcm_s24le": true, "pcm_s32le": true, "pcm_f32le": true,
	}

	// mp4FamilyNoVideoCopy lists legacy video codecs the MP4 muxer rejects
	// (FLV/Theora/VP6 sources); "keep" becomes an H.264 re-encode.
	mp4FamilyNoVideoCopy = map[string]bool{
		"flv1": true, "theora": true, "vp6": true, "vp6a": true, "vp6f": true,
	}

	hevcNames = map[string]bool{"hevc": true, "hvc1": true, "hev1": true}
)

// isMP4VideoExt reports whether the output extension can carry video and
// belongs to the MP4 family (relevant for the hvc1 compatibility tag).
func isMP4VideoExt(ext string) bool {
	switch ext {
	case ".mp4", ".mov", ".m4v":
		return true
	}
	return false
}

// IsBitmapSubtitleCodec reports whether a source subtitle codec is bitmap
// based (PGS/DVB/DVD subtitles). Bitmap subtitles cannot be re-encoded as
// text — ffmpeg only converts text to text or bitmap to bitmap — so a
// stream copy into a text-only target such as the MP4 family must drop
// them explicitly instead of building a command that dies at the mov_text
// encoder.
func IsBitmapSubtitleCodec(codec string) bool {
	switch codec {
	case "hdmv_pgs_subtitle", "dvb_sub", "dvd_subtitle":
		return true
	}
	return strings.Contains(codec, "pgs") ||
		strings.Contains(codec, "dvd_subtitle") ||
		strings.Contains(codec, "dvb_sub")
}

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
			if codec == "copy" {
				if mp4FamilyNoVideoCopy[sourceCodec] {
					return "libx264"
				}
				return "copy"
			}
			if mp4FamilyVideo[codec] {
				return codec
			}
			return "libx264"
		case TrackAudio:
			// Vorbis has no MP4 muxer support; several other common codecs
			// cannot be stream-copied into MP4 either, so "keep" becomes an
			// AAC re-encode for them (see mp4FamilyNoAudioCopy).
			if codec == "libvorbis" || (codec == "copy" && sourceCodec == "vorbis") {
				return "aac"
			}
			if codec == "copy" && mp4FamilyNoAudioCopy[sourceCodec] {
				return "aac"
			}
			return codec
		case TrackSubtitle:
			// MP4-family containers only carry text subtitles, and a bitmap
			// source cannot be converted to text — drop it (as with webm)
			// instead of building a command that dies in the mov_text
			// encoder.
			if IsBitmapSubtitleCodec(sourceCodec) {
				return ""
			}
			if codec == "copy" && sourceCodec == "mov_text" {
				return "copy"
			}
			return "mov_text"
		}
	case ".mkv":
		if trackType == TrackSubtitle {
			// Matroska carries bitmap subtitles fine, so a copy is kept;
			// a text target, however, cannot be reached from a bitmap
			// source and must be dropped.
			if IsBitmapSubtitleCodec(sourceCodec) && codec != "copy" {
				return ""
			}
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
