package ffmpeg

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// RotateOutputName derives the default output name for a rotated copy:
// <base>_rot<degrees><ext>, e.g. clip.mp4 + 90 -> clip_rot90.mp4.
func RotateOutputName(inputPath string, degrees int) string {
	return transformOutputName(inputPath, "rot"+strconv.Itoa(degrees))
}

// FlipOutputName derives the default output name for a flipped copy:
// <base>_flip<direction><ext>, e.g. clip.mp4 + "h" -> clip_fliph.mp4.
func FlipOutputName(inputPath, direction string) string {
	return transformOutputName(inputPath, "flip"+direction)
}

// CropOutputName derives the default output name for a cropped copy:
// <base>_cropped<ext>, e.g. clip.mp4 -> clip_cropped.mp4.
func CropOutputName(inputPath string) string {
	return transformOutputName(inputPath, "cropped")
}

func transformOutputName(inputPath, suffix string) string {
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), ext)
	return base + "_" + suffix + ext
}

// BuildRotateCmd rotates the video by 90, 180 or 270 degrees and re-encodes
// it with the standard encoding tail. Any other angle yields a zero Cmd.
func BuildRotateCmd(inputPath string, degrees int, outputPath string) Cmd {
	var filter string
	switch degrees {
	case 90:
		filter = "transpose=1"
	case 180:
		filter = "hflip,vflip"
	case 270:
		filter = "transpose=2"
	default:
		return Cmd{}
	}
	return Cmd{
		Args: appendReencodeArgs([]string{"-i", inputPath, "-vf", filter}, outputPath),
	}
}

// BuildFlipCmd mirrors the video horizontally ("h") or vertically ("v") and
// re-encodes it with the standard encoding tail. Any other direction yields
// a zero Cmd.
func BuildFlipCmd(inputPath, direction, outputPath string) Cmd {
	var filter string
	switch direction {
	case "h":
		filter = "hflip"
	case "v":
		filter = "vflip"
	default:
		return Cmd{}
	}
	return Cmd{
		Args: appendReencodeArgs([]string{"-i", inputPath, "-vf", filter}, outputPath),
	}
}

// BuildCropCmd cuts a width x height window whose top-left corner sits at
// (x, y) and re-encodes it with the standard encoding tail. All values must
// be non-negative; anything else yields a zero Cmd. x=0/y=0 (top-left
// origin) is the most common crop and fully valid. Whether the window fits
// the source is ffmpeg's job to reject.
func BuildCropCmd(inputPath string, x, y, width, height int, outputPath string) Cmd {
	if width <= 0 || height <= 0 || x < 0 || y < 0 {
		return Cmd{}
	}
	filter := fmt.Sprintf("crop=w=%d:h=%d:x=%d:y=%d", width, height, x, y)
	return Cmd{
		Args: appendReencodeArgs([]string{"-i", inputPath, "-vf", filter}, outputPath),
	}
}

func appendReencodeArgs(args []string, outputPath string) []string {
	return append(args,
		"-c:v", "libx264",
		"-crf", "23",
		"-preset", "medium",
		"-pix_fmt", "yuv420p",
		"-c:a", "copy",
		"-y", outputPath,
	)
}
