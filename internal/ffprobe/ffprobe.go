package ffprobe

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fsx8/ffwiz/internal/execx"
)

type Prober interface {
	Probe(ctx context.Context, path string) (*ProbeResult, error)
	HasAudio(ctx context.Context, path string) (bool, error)
	// Keyframes returns the timestamps (in seconds) of the default video
	// stream's keyframes, probed via -skip_frame nokey (decodes keyframes
	// only). Used for lossless trim snapping.
	Keyframes(ctx context.Context, path string) ([]float64, error)
}

type prober struct {
	runner execx.Runner
}

func New(runner execx.Runner) Prober {
	return &prober{runner: runner}
}

func (p *prober) Probe(ctx context.Context, path string) (*ProbeResult, error) {
	cmd := execx.Cmd{
		Name: "ffprobe",
		Args: []string{"-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", path},
	}
	stdout, stderr, err := p.runner.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w\n%s", err, stderr)
	}
	var res ProbeResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		return nil, fmt.Errorf("ffprobe output parse failed: %w", err)
	}
	return &res, nil
}

func (p *prober) HasAudio(ctx context.Context, path string) (bool, error) {
	res, err := p.Probe(ctx, path)
	if err != nil {
		return false, err
	}
	for _, s := range res.Streams {
		if s.CodecType == "audio" {
			return true, nil
		}
	}
	return false, nil
}
