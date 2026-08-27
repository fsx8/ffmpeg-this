package ffmpeg

import (
	"fmt"
	"strconv"
	"strings"
)

type JoinTargets struct {
	Width      int
	Height     int
	SampleRate string
	SAR        string // "1:1"
}

// JoinInput describes one file selected for a join. HasAudio and
// DurationSec come from probing; the duration is needed to synthesize
// silence for inputs without an audio stream.
type JoinInput struct {
	Path        string
	HasAudio    bool
	DurationSec float64
}

// BuildJoinCmd builds a concat filter command that normalizes every input
// to the target resolution and audio sample rate.
//
// Audio handling:
//   - all inputs have audio:  all streams are normalized and concatenated
//   - some inputs have audio: silence of the input's duration is
//     synthesized for the others (concat requires an audio stream per
//     segment); if any such duration is unknown, audio is dropped instead
//   - no input has audio:     the output is video-only
//
// Real audio streams are normalized to stereo so that silence and sources
// with differing channel layouts can be concatenated safely.
func BuildJoinCmd(inputs []JoinInput, outputPath string, target JoinTargets) Cmd {
	audioSegments := true
	withAudio := 0
	for _, in := range inputs {
		if in.HasAudio {
			withAudio++
		}
	}
	switch {
	case withAudio == 0:
		audioSegments = false
	case withAudio < len(inputs):
		for _, in := range inputs {
			if !in.HasAudio && in.DurationSec <= 0 {
				audioSegments = false
				break
			}
		}
	}

	args := []string{}
	for _, in := range inputs {
		args = append(args, "-i", in.Path)
	}

	sar := strings.TrimSpace(target.SAR)
	if sar == "" {
		sar = "1:1"
	}
	sar = strings.ReplaceAll(sar, ":", "/")

	var filters []string
	var concatInputs strings.Builder
	for i, in := range inputs {
		filters = append(filters,
			fmt.Sprintf("[%d:v]scale=w=%d:h=%d:force_original_aspect_ratio=decrease,pad=w=%d:h=%d:x=(ow-iw)/2:y=(oh-ih)/2,setsar=sar=%s,setpts=PTS-STARTPTS[v%d]",
				i, target.Width, target.Height, target.Width, target.Height, sar, i,
			),
		)
		if audioSegments {
			if in.HasAudio {
				filters = append(filters,
					fmt.Sprintf("[%d:a]aformat=sample_rates=%s:channel_layouts=stereo,asetpts=PTS-STARTPTS[a%d]",
						i, target.SampleRate, i,
					),
				)
			} else {
				filters = append(filters,
					fmt.Sprintf("anullsrc=channel_layout=stereo:sample_rate=%s,atrim=duration=%s,asetpts=PTS-STARTPTS[a%d]",
						target.SampleRate, strconv.FormatFloat(in.DurationSec, 'f', -1, 64), i,
					),
				)
			}
		}
	}

	if audioSegments {
		for i := range inputs {
			concatInputs.WriteString(fmt.Sprintf("[v%d][a%d]", i, i))
		}
		filters = append(filters, fmt.Sprintf("%sconcat=n=%d:v=1:a=1[v][a]", concatInputs.String(), len(inputs)))
		args = append(args,
			"-filter_complex", strings.Join(filters, ";"),
			"-map", "[v]",
			"-map", "[a]",
			"-c:v", "libx264",
			"-crf", "23",
			"-c:a", "aac",
			"-b:a", "192k",
		)
	} else {
		for i := range inputs {
			concatInputs.WriteString(fmt.Sprintf("[v%d]", i))
		}
		filters = append(filters, fmt.Sprintf("%sconcat=n=%d:v=1:a=0[v]", concatInputs.String(), len(inputs)))
		args = append(args,
			"-filter_complex", strings.Join(filters, ";"),
			"-map", "[v]",
			"-c:v", "libx264",
			"-crf", "23",
		)
	}

	args = append(args, "-y", outputPath)
	return Cmd{Args: args}
}
