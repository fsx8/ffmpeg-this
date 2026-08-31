package ffmpeg

import (
	"path/filepath"
	"strconv"
	"strings"
)

// CompressOutputName derives the default output name for a compressed copy:
// <base>_crf<N><ext>, e.g. clip.mp4 + 28 -> clip_crf28.mp4.
func CompressOutputName(inputPath string, crf int) string {
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), ext)
	return base + "_crf" + strconv.Itoa(crf) + ext
}

// BuildCompressCmd re-encodes the video with libx264 at the given CRF and
// x264 speed preset while stream-copying audio. The caller validates the
// CRF range (0-51) and the preset name.
func BuildCompressCmd(inputPath string, crf int, preset string, outputPath string) Cmd {
	return Cmd{
		Args: []string{
			"-i", inputPath,
			"-c:v", "libx264",
			"-crf", strconv.Itoa(crf),
			"-preset", preset,
			"-pix_fmt", "yuv420p",
			"-c:a", "copy",
			"-y", outputPath,
		},
	}
}
