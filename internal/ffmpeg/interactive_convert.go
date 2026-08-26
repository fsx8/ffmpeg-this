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

func BuildInteractiveConvertCmd(inputPath, outputPath string, tracks []Track, actions map[int]TrackActionInfo) *Cmd {
	type indexedTrack struct {
		track   Track
		trackID int
	}

	var videos, audios, subs []indexedTrack
	for i, t := range tracks {
		info, ok := actions[i]
		action := ActionKeep
		codec := ""
		if ok {
			action = info.Action
			codec = info.Codec
		}
		if action == ActionRemove {
			continue
		}
		_ = codec

		switch t.Type {
		case TrackVideo:
			videos = append(videos, indexedTrack{track: t, trackID: i})
		case TrackAudio:
			audios = append(audios, indexedTrack{track: t, trackID: i})
		case TrackSubtitle:
			subs = append(subs, indexedTrack{track: t, trackID: i})
		}
	}

	if len(videos)+len(audios)+len(subs) == 0 {
		return nil
	}

	args := []string{"-i", inputPath}

	addMaps := func(ts []indexedTrack) {
		for _, it := range ts {
			args = append(args, "-map", fmt.Sprintf("0:%d", it.track.Index))
		}
	}
	addMaps(videos)
	addMaps(audios)
	addMaps(subs)

	// Per-type output stream indexes must be compact.
	videoOutIdx, audioOutIdx, subOutIdx := 0, 0, 0

	setOutputArgsFor := func(track Track, outIndex int, action TrackAction, codec string) {
		clean := strings.ToLower(CleanCodecChoice(codec))
		switch track.Type {
		case TrackVideo:
			if action != ActionConvert || clean == "" || clean == "copy" {
				args = append(args, fmt.Sprintf("-c:v:%d", outIndex), "copy")
				return
			}
			args = append(args, fmt.Sprintf("-c:v:%d", outIndex), clean)
			switch clean {
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
			}
		case TrackAudio:
			if action != ActionConvert || clean == "" || clean == "copy" {
				args = append(args, fmt.Sprintf("-c:a:%d", outIndex), "copy")
				return
			}
			args = append(args, fmt.Sprintf("-c:a:%d", outIndex), clean)
			switch clean {
			case "aac", "libmp3lame", "libfdk_aac", "libvorbis":
				args = append(args, fmt.Sprintf("-b:a:%d", outIndex), "192k")
			case "libopus":
				args = append(args, fmt.Sprintf("-b:a:%d", outIndex), "160k")
			}
		case TrackSubtitle:
			if action != ActionConvert || clean == "" || clean == "copy" {
				args = append(args, fmt.Sprintf("-c:s:%d", outIndex), "copy")
				return
			}
			if strings.Contains(clean, "subrip") {
				clean = "srt"
			}
			args = append(args, fmt.Sprintf("-c:s:%d", outIndex), clean)
		}
	}

	for _, it := range videos {
		info, ok := actions[it.trackID]
		action := ActionKeep
		codec := ""
		if ok {
			action = info.Action
			codec = info.Codec
		}
		setOutputArgsFor(it.track, videoOutIdx, action, codec)
		videoOutIdx++
	}
	for _, it := range audios {
		info, ok := actions[it.trackID]
		action := ActionKeep
		codec := ""
		if ok {
			action = info.Action
			codec = info.Codec
		}
		setOutputArgsFor(it.track, audioOutIdx, action, codec)
		audioOutIdx++
	}
	for _, it := range subs {
		info, ok := actions[it.trackID]
		action := ActionKeep
		codec := ""
		if ok {
			action = info.Action
			codec = info.Codec
		}
		setOutputArgsFor(it.track, subOutIdx, action, codec)
		subOutIdx++
	}

	args = append(args, "-y", outputPath)
	return &Cmd{Args: args}
}
