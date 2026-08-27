package ffprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/fsx8/ffwiz/internal/execx"
)

type keyframeReport struct {
	Frames []struct {
		PTSTime string `json:"pts_time"`
	} `json:"frames"`
}

// Keyframes probes the keyframe timestamps of the first video stream.
// -skip_frame nokey makes ffprobe decode keyframes only, so the cost stays
// proportional to the GOP count rather than the frame count.
func (p *prober) Keyframes(ctx context.Context, path string) ([]float64, error) {
	cmd := execx.Cmd{
		Name: "ffprobe",
		Args: []string{
			"-v", "quiet",
			"-skip_frame", "nokey",
			"-select_streams", "v:0",
			"-show_entries", "frame=pts_time",
			"-of", "json",
			path,
		},
	}
	stdout, stderr, err := p.runner.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("ffprobe keyframes failed: %w\n%s", err, stderr)
	}
	var report keyframeReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		return nil, fmt.Errorf("ffprobe keyframes parse failed: %w", err)
	}
	var out []float64
	for _, f := range report.Frames {
		if f.PTSTime == "" || f.PTSTime == "N/A" {
			continue
		}
		v, err := strconv.ParseFloat(f.PTSTime, 64)
		if err != nil || v < 0 {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}
