package ffmpeg

import (
	"path/filepath"
	"strings"
)

func ExtractAudioOutputName(inputPath, format string) string {
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), ext)
	return base + "_audio." + format
}

func BuildExtractAudioCmd(inputPath, format, outputPath string) Cmd {
	args := []string{"-i", inputPath, "-vn", "-acodec", AudioCodecFor(format)}
	if format == "mp3" {
		args = append(args, "-b:a", "192k")
	}
	args = append(args, "-y", outputPath)
	return Cmd{Args: args}
}
