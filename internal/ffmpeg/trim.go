package ffmpeg

import (
	"path/filepath"
	"strings"
)

func TrimOutputName(inputPath string) string {
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), ext)
	return base + "_trimmed" + ext
}

func BuildTrimCmd(inputPath, start, end, outputPath string) Cmd {
	// -ss/-to are output options: input-side -to requires ffmpeg >= 5.0
	// (e.g. Ubuntu 22.04 ships 4.x), so seek on the output side instead.
	return Cmd{
		Args: []string{"-i", inputPath, "-ss", start, "-to", end, "-c", "copy", "-y", outputPath},
	}
}
