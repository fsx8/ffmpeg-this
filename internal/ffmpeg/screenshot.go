package ffmpeg

import (
	"path/filepath"
	"strings"
)

// ScreenshotOutputName derives the default output name for a frame grab:
// <base>_frame_<ts>.<format> where <ts> is the raw timestamp with ':' and
// '.' replaced by '-' so it is safe for filenames,
// e.g. clip.mp4 + "00:05:00" + png -> clip_frame_00-05-00.png.
func ScreenshotOutputName(inputPath, timestamp, format string) string {
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), ext)
	ts := strings.NewReplacer(":", "-", ".", "-", "/", "-", "\\", "-").Replace(timestamp)
	return base + "_frame_" + ts + "." + format
}

// BuildScreenshotCmd grabs a single frame at the given timestamp:
// -ss <timestamp> -i <input> -frames:v 1 -y <output>. The seek is an input
// option on purpose: a single-frame grab always re-encodes, so fast input
// seeking is safe and works on every ffmpeg version.
func BuildScreenshotCmd(inputPath, timestamp, outputPath string) Cmd {
	return Cmd{
		Args: []string{"-ss", timestamp, "-i", inputPath, "-frames:v", "1", "-y", outputPath},
	}
}
