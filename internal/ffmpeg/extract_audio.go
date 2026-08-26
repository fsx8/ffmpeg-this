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
	acodec := format
	if format == "mp3" {
		acodec = "libmp3lame"
	}
	return Cmd{
		Args: []string{"-i", inputPath, "-vn", "-acodec", acodec, "-y", outputPath},
	}
}
