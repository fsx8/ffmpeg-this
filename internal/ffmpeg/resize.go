package ffmpeg

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResizeOutputName derives the default output name for a resized copy:
// <base>_<label><ext>, e.g. clip.mp4 + "720p" -> clip_720p.mp4.
func ResizeOutputName(inputPath, label string) string {
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), ext)
	return base + "_" + label + ext
}

// BuildResizeCmd scales the video to width x height and re-encodes it with
// libx264 while stream-copying audio. Either dimension may be -2, which the
// scale filter interprets as "derive it automatically from the aspect ratio
// and round to an even value"; the caller decides what to auto-fill.
func BuildResizeCmd(inputPath string, width, height int, outputPath string) Cmd {
	return Cmd{
		Args: []string{
			"-i", inputPath,
			"-vf", fmt.Sprintf("scale=w=%d:h=%d", width, height),
			"-c:v", "libx264",
			"-crf", "23",
			"-preset", "medium",
			"-pix_fmt", "yuv420p",
			"-c:a", "copy",
			"-y", outputPath,
		},
	}
}
