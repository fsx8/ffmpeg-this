package ffmpeg

import (
	"fmt"
	"strings"
)

func CleanCodecChoice(selectedOption string) string {
	cleaned := strings.TrimSpace(selectedOption)
	if cleaned == "" {
		return cleaned
	}

	if strings.HasPrefix(cleaned, "(") && strings.HasSuffix(cleaned, ")") {
		inner := strings.TrimSpace(strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(cleaned, "("), ")")))
		switch {
		case strings.Contains(inner, "subrip"):
			return "srt"
		case inner == "ass":
			return "ass"
		case strings.Contains(inner, "mp4"):
			return "mov_text"
		default:
			return inner
		}
	}

	first, _, _ := strings.Cut(cleaned, " ")
	return strings.TrimSpace(first)
}

type outTrack struct {
	track   Track
	action  TrackAction
	codec   string // raw (possibly label) codec choice; "" for keep
	dropped bool   // cannot be muxed into the target container
	final   string // resolved codec after cleaning + container normalization
	convert bool   // final != copy
}

// BuildInteractiveConvertCmd maps the kept tracks of the input and applies
// per-track actions. Codecs are normalized for the output container (see
// NormalizeCodecForContainer); the confirm screen should preview the
// command returned here so users see the final arguments.
// Returns nil when nothing remains to be written.
func BuildInteractiveConvertCmd(inputPath, outputPath string, tracks []Track, actions map[int]TrackActionInfo) *Cmd {
	var kept []outTrack
	for i, t := range tracks {
		ot := outTrack{track: t, action: ActionKeep}
		if info, ok := actions[i]; ok {
			ot.action = info.Action
			ot.codec = info.Codec
		}
		if ot.action == ActionRemove {
			continue
		}
		kept = append(kept, ot)
	}

	// Resolve codecs: keep => copy, convert => cleaned choice, then adjust
	// for the output container (may drop streams webm cannot carry).
	for i := range kept {
		codec := "copy"
		if kept[i].action == ActionConvert {
			codec = strings.ToLower(CleanCodecChoice(kept[i].codec))
			if codec == "" {
				codec = "copy"
			}
		}
		kept[i].final = NormalizeCodecForContainer(kept[i].track.Type, codec, kept[i].track.Codec, outputPath)
		if kept[i].final == "" {
			kept[i].dropped = true
		} else if kept[i].final != "copy" {
			kept[i].convert = true
		}
	}

	kept = filterDropped(kept)
	if len(kept) == 0 {
		return nil
	}

	// Group by type so the output gets video streams first, then audio,
	// then subtitles (this is also the -map order).
	var videos, audios, subs []outTrack
	for _, ot := range kept {
		switch ot.track.Type {
		case TrackVideo:
			videos = append(videos, ot)
		case TrackAudio:
			audios = append(audios, ot)
		case TrackSubtitle:
			subs = append(subs, ot)
		}
	}

	args := []string{"-i", inputPath}
	for _, group := range [][]outTrack{videos, audios, subs} {
		for _, ot := range group {
			args = append(args, "-map", fmt.Sprintf("0:%d", ot.track.Index))
		}
	}

	for i, ot := range videos {
		args = appendVideoArgs(args, i, ot)
	}
	for i, ot := range audios {
		args = appendAudioArgs(args, i, ot)
	}
	for i, ot := range subs {
		args = appendSubtitleArgs(args, i, ot)
	}

	args = append(args, "-y", outputPath)
	return &Cmd{Args: args}
}

func filterDropped(kept []outTrack) []outTrack {
	out := kept[:0]
	for _, ot := range kept {
		if !ot.dropped {
			out = append(out, ot)
		}
	}
	return out
}

func appendVideoArgs(args []string, outIndex int, ot outTrack) []string {
	if !ot.convert {
		return append(args, fmt.Sprintf("-c:v:%d", outIndex), "copy")
	}
	args = append(args, fmt.Sprintf("-c:v:%d", outIndex), ot.final)
	switch ot.final {
	case "libx264":
		args = append(args,
			fmt.Sprintf("-crf:v:%d", outIndex), "23",
			fmt.Sprintf("-preset:v:%d", outIndex), "medium",
			fmt.Sprintf("-pix_fmt:v:%d", outIndex), "yuv420p",
		)
	case "libx265":
		args = append(args,
			fmt.Sprintf("-crf:v:%d", outIndex), "28",
			fmt.Sprintf("-preset:v:%d", outIndex), "medium",
		)
	case "libvpx-vp9":
		args = append(args,
			fmt.Sprintf("-crf:v:%d", outIndex), "31",
			fmt.Sprintf("-b:v:%d", outIndex), "0",
		)
	}
	return args
}

func appendAudioArgs(args []string, outIndex int, ot outTrack) []string {
	if !ot.convert {
		return append(args, fmt.Sprintf("-c:a:%d", outIndex), "copy")
	}
	args = append(args, fmt.Sprintf("-c:a:%d", outIndex), ot.final)
	switch ot.final {
	case "aac", "libmp3lame", "libvorbis":
		args = append(args, fmt.Sprintf("-b:a:%d", outIndex), "192k")
	case "libopus":
		args = append(args, fmt.Sprintf("-b:a:%d", outIndex), "160k")
	}
	return args
}

func appendSubtitleArgs(args []string, outIndex int, ot outTrack) []string {
	if !ot.convert {
		return append(args, fmt.Sprintf("-c:s:%d", outIndex), "copy")
	}
	codec := ot.final
	if strings.Contains(codec, "subrip") {
		codec = "srt"
	}
	return append(args, fmt.Sprintf("-c:s:%d", outIndex), codec)
}
