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
		switch quality {
		case QualitySame:
			args = append(args, "-c", "copy")
		case QualityHigh, QualityMedium, QualityLow:
			crf := map[BatchVideoQuality]string{QualityHigh: "18", QualityMedium: "23", QualityLow: "28"}[quality]
			args = append(args, "-c:v", "libx264", "-crf", crf, "-pix_fmt", "yuv420p")
			if hasAudio {
				args = append(args, "-c:a", "aac", "-b:a", "192k")
			} else {
				args = append(args, "-an")
			}
		}
	case isAudio:
		acodec := format
		if format == "mp3" {
			acodec = "libmp3lame"
		}
		args = append(args, "-vn", "-c:a", acodec)
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
