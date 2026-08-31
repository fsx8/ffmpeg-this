package ffmpeg

import (
	"sort"
	"strconv"
)

// SetMetadataOutputName derives the default output name for a retagged
// copy: <base>_tagged<ext>, e.g. clip.mp4 -> clip_tagged.mp4.
func SetMetadataOutputName(inputPath string) string {
	return transformOutputName(inputPath, "tagged")
}

// StripMetadataOutputName derives the default output name for a copy with
// all metadata removed: <base>_stripped<ext>, e.g. clip.mkv -> clip_stripped.mkv.
func StripMetadataOutputName(inputPath string) string {
	return transformOutputName(inputPath, "stripped")
}

// BuildSetMetadataCmd stream-copies the input while writing the given
// global tags; one -metadata key=value argument per tag, sorted by key so
// the argument list is deterministic. Existing tags that are absent from
// the map are left untouched by ffmpeg; an empty value deletes the tag.
//
// -map 0 keeps every input stream: ffmpeg's automatic selection would
// otherwise retain only the best stream of each type, so tagging a movie
// with several audio/subtitle tracks would silently drop the rest (the
// same reason BuildStripMetadataCmd maps everything).
func BuildSetMetadataCmd(inputPath string, tags map[string]string, outputPath string) Cmd {
	args := []string{"-i", inputPath, "-map", "0", "-c", "copy"}
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-metadata", k+"="+tags[k])
	}
	return Cmd{Args: append(args, "-y", outputPath)}
}

// BuildStripMetadataCmd stream-copies the input with global metadata and
// chapter information removed (-map_metadata -1 -map_chapters -1) and
// clears the title and language tag of every stream, using empty values
// which ffmpeg treats as tag deletion. -map 0 keeps every input stream:
// ffmpeg's automatic selection would otherwise retain only the best stream
// of each type. streamCount is the total number of streams from a probe;
// values <= 0 skip the per-stream arguments.
func BuildStripMetadataCmd(inputPath string, streamCount int, outputPath string) Cmd {
	args := []string{"-i", inputPath, "-map", "0", "-c", "copy", "-map_metadata", "-1", "-map_chapters", "-1"}
	for i := 0; i < streamCount; i++ {
		s := "-metadata:s:" + strconv.Itoa(i)
		args = append(args, s, "title=")
		args = append(args, s, "language=")
	}
	return Cmd{Args: append(args, "-y", outputPath)}
}
