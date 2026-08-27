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

func BuildBatchConvertCmd(inputPath, outputPath, format string, quality BatchVideoQuality, hasAudio bool) Cmd {
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
			args = append(args, "-c", "copy")
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
