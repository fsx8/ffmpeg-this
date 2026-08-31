package ffmpeg

import (
	"fmt"
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
	// -map 0 keeps every input stream: with plain -c copy ffmpeg's
	// automatic selection would retain only the "best" stream of each
	// type, silently dropping the rest of a multi-track file.
	return Cmd{
		Args: []string{"-i", inputPath, "-map", "0", "-ss", start, "-to", end, "-c", "copy", "-y", outputPath},
	}
}

// SnapToKeyframe maps a requested cut start to the latest keyframe at or
// before it (within epsilon): -c copy can only begin a video at a keyframe,
// so an unsnapped mid-GOP start would drop video packets up to the next
// keyframe — up to an entire GOP of video, or the whole stream when the
// next keyframe lies beyond the cut end. When start precedes every keyframe
// it snaps forward to the first one; an empty keyframe list (probe failed
// or unavailable) returns start unchanged so trimming degrades gracefully.
func SnapToKeyframe(start float64, keyframes []float64) float64 {
	if len(keyframes) == 0 {
		return start
	}
	const eps = 0.05
	snap := keyframes[0]
	for _, k := range keyframes {
		if k <= start+eps {
			snap = k
		}
	}
	return snap
}

// FormatTimeSpec renders seconds as HH:MM:SS(.mmm) for ffmpeg arguments,
// omitting the fractional part when it is zero so snapped cut points stay
// human-readable.
func FormatTimeSpec(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	totalMs := int64(sec*1000 + 0.5)
	ms := totalMs % 1000
	total := totalMs / 1000
	base := fmt.Sprintf("%02d:%02d:%02d", total/3600, (total%3600)/60, total%60)
	if ms != 0 {
		return base + fmt.Sprintf(".%03d", ms)
	}
	return base
}
