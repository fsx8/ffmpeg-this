package ffmpeg

import (
	"fmt"
	"path/filepath"
	"strings"
)

type BatchVideoQuality string

const (
	QualitySame   BatchVideoQuality = "same"
	QualityHigh   BatchVideoQuality = "crf18"
	QualityMedium BatchVideoQuality = "crf23"
	QualityLow    BatchVideoQuality = "crf28"
)

func BatchOutputName(inputPath, format string) string {
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), ext)
	return fmt.Sprintf("%s_batch.%s", base, format)
}

// BatchStreamInfo carries the probe-derived stream layout of a source
// file, so the "same quality (copy)" preset can adapt to the target
// container instead of silently dropping streams (a plain -c copy keeps
// only the "best" of each type) or building a command that dies at the
// muxer (codecs the container cannot hold). Codec lists follow stream
// order; empty lists mean no probe info is available.
type BatchStreamInfo struct {
	AudioCodecs    []string
	SubtitleCodecs []string
}

func BuildBatchConvertCmd(inputPath, outputPath, format string, quality BatchVideoQuality, hasAudio bool, streams BatchStreamInfo) Cmd {
	isVideo := map[string]bool{"mp4": true, "mkv": true, "mov": true, "avi": true, "webm": true}[format]
	isAudio := map[string]bool{"mp3": true, "flac": true, "wav": true}[format]

	args := []string{"-i", inputPath}

	switch {
	case isVideo:
		// The webm muxer only accepts VP8/VP9/AV1 video and Vorbis/Opus
		// audio, so a stream copy of typical H.264/AAC sources cannot work;
		// re-encode instead (the UI hides the copy preset for webm).
		if format == "webm" && quality == QualitySame {
			quality = QualityMedium
		}
		switch quality {
		case QualitySame:
			args = appendCopyArgs(args, format, hasAudio, streams)
		case QualityHigh, QualityMedium, QualityLow:
			if format == "webm" {
				args = append(args, "-c:v", "libvpx-vp9", "-crf", webmCRF(quality), "-b:v", "0")
				args = appendAudio(args, hasAudio, "libopus", "128k")
			} else {
				args = append(args, "-c:v", "libx264", "-crf", h264CRF(quality), "-pix_fmt", "yuv420p")
				args = appendAudio(args, hasAudio, "aac", "192k")
			}
		}
	case isAudio:
		args = append(args, "-vn", "-c:a", AudioCodecFor(format))
		if format == "mp3" {
			args = append(args, "-b:a", "192k")
		}
	case format == "gif":
		// The GIF pipeline is a 2-step process in the app; this is for the final step only.
		// Callers should use BuildGifPaletteCmd + BuildGifFromPaletteCmd.
	}

	args = append(args, "-y", outputPath)
	return Cmd{Args: args}
}

func appendAudio(args []string, hasAudio bool, codec, bitrate string) []string {
	if !hasAudio {
		return append(args, "-an")
	}
	return append(args, "-c:a", codec, "-b:a", bitrate)
}

// appendCopyArgs builds the arguments for the "same quality (copy)" preset
// of a video target. Without an explicit map ffmpeg's automatic selection
// retains only the "best" stream of each type, silently dropping the rest,
// while a naive -map 0 -c copy turns that silent loss into loud muxer
// failures for cross-container copies — so every container gets an
// explicit policy: keep everything where anything muxes, normalize or
// selectively drop where the container is restrictive, and fall back to a
// re-encode where a copy cannot work at all.
func appendCopyArgs(args []string, format string, hasAudio bool, streams BatchStreamInfo) []string {
	switch format {
	case "mkv":
		// Matroska muxes virtually any codec: copy every stream as-is.
		return append(args, "-map", "0", "-c", "copy")
	case "mp4", "mov":
		return appendMP4FamilyCopyArgs(args, hasAudio, streams)
	default: // "avi"
		// AVI cannot carry subtitles and its codec tags reject most
		// modern streams (e.g. HEVC video, DTS/Opus audio), so a copy
		// would end in a muxer error; re-encode to the container's
		// classic codecs instead — the same fallback the webm target uses.
		args = append(args, "-c:v", "libx264", "-crf", h264CRF(QualityMedium), "-pix_fmt", "yuv420p")
		return appendAudio(args, hasAudio, "libmp3lame", "192k")
	}
}

// appendMP4FamilyCopyArgs adapts a stream copy to the MP4 family (mp4,
// mov): video always copies, audio codecs the muxer rejects become AAC
// re-encodes (see mp4FamilyNoAudioCopy), text subtitles convert to
// mov_text, and bitmap subtitles are dropped (IsBitmapSubtitleCodec).
// Kept streams are enumerated explicitly rather than by negative mapping,
// which older ffmpeg (4.x) does not support.
func appendMP4FamilyCopyArgs(args []string, hasAudio bool, streams BatchStreamInfo) []string {
	args = append(args,
		"-map", "0:V?", // V excludes attached cover art, which the muxer rejects
		"-map", "0:a?",
		"-c:v", "copy",
	)
	switch {
	case len(streams.AudioCodecs) > 0:
		for i, codec := range streams.AudioCodecs {
			if mp4FamilyNoAudioCopy[codec] {
				args = append(args, fmt.Sprintf("-c:a:%d", i), "aac", fmt.Sprintf("-b:a:%d", i), "192k")
			} else {
				args = append(args, fmt.Sprintf("-c:a:%d", i), "copy")
			}
		}
	case hasAudio:
		// Without codec names a stream cannot be classified as muxable,
		// so re-encode instead of risking a dead command.
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	}
	textSubs := 0
	for i, codec := range streams.SubtitleCodecs {
		if IsBitmapSubtitleCodec(codec) {
			continue // no muxable form: drop explicitly instead of failing
		}
		args = append(args, "-map", fmt.Sprintf("0:s:%d", i))
		textSubs++
	}
	if textSubs > 0 {
		args = append(args, "-c:s", "mov_text")
	}
	return args
}

func h264CRF(q BatchVideoQuality) string {
	return map[BatchVideoQuality]string{QualityHigh: "18", QualityMedium: "23", QualityLow: "28"}[q]
}

// webmCRF maps the quality presets to VP9-appropriate CRF values
// (VP9 CRF range is much higher than x264's for comparable quality).
func webmCRF(q BatchVideoQuality) string {
	return map[BatchVideoQuality]string{QualityHigh: "24", QualityMedium: "31", QualityLow: "38"}[q]
}

func BuildGifPaletteCmd(inputPath, palettePath string) Cmd {
	return Cmd{
		Args: []string{
			"-i", inputPath,
			"-vf", "fps=15,scale=480:-1:flags=lanczos,palettegen",
			"-y", palettePath,
		},
	}
}

func BuildGifFromPaletteCmd(inputPath, palettePath, outputPath string) Cmd {
	return Cmd{
		Args: []string{
			"-i", inputPath,
			"-i", palettePath,
			"-filter_complex", "fps=15,scale=480:-1:flags=lanczos[x];[x][1:v]paletteuse",
			"-y", outputPath,
		},
	}
}
