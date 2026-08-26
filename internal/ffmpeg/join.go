package ffmpeg

import (
	"fmt"
	"strings"
)

type JoinTargets struct {
	Width      int
	Height     int
	SampleRate string
	SAR        string // "1:1"
}

func BuildJoinCmd(inputPaths []string, outputPath string, target JoinTargets) Cmd {
	args := []string{}
	for _, p := range inputPaths {
		args = append(args, "-i", p)
	}

	sar := strings.TrimSpace(target.SAR)
	if sar == "" {
		sar = "1:1"
	}
	sar = strings.ReplaceAll(sar, ":", "/")

	var filters []string
	var concatInputs strings.Builder
	for i := range inputPaths {
		filters = append(filters,
			fmt.Sprintf("[%d:v]scale=w=%d:h=%d:force_original_aspect_ratio=decrease,pad=w=%d:h=%d:x=(ow-iw)/2:y=(oh-ih)/2,setsar=sar=%s,setpts=PTS-STARTPTS[v%d]",
				i, target.Width, target.Height, target.Width, target.Height, sar, i,
			),
		)
		filters = append(filters,
			fmt.Sprintf("[%d:a]aresample=%s,asetpts=PTS-STARTPTS[a%d]", i, target.SampleRate, i),
		)
		concatInputs.WriteString(fmt.Sprintf("[v%d][a%d]", i, i))
	}
	filters = append(filters, fmt.Sprintf("%sconcat=n=%d:v=1:a=1[v][a]", concatInputs.String(), len(inputPaths)))

	args = append(args,
		"-filter_complex", strings.Join(filters, ";"),
		"-map", "[v]",
		"-map", "[a]",
		"-c:v", "libx264",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "192k",
		"-y", outputPath,
	)

	return Cmd{Args: args}
}
