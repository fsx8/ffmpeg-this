package app

import (
	"context"
	"fmt"
	"log"
	"runtime"

	"github.com/fsx8/ffwiz/internal/execx"
	"github.com/fsx8/ffwiz/internal/ffprobe"
)

type Config struct {
	Logger      *log.Logger
	Runner      execx.Runner
	Prober      ffprobe.Prober
	InitialPath string
}

func CheckFFmpegFFprobe(ctx context.Context, runner execx.Runner) error {
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		_, _, err := runner.Run(ctx, execx.Cmd{Name: bin, Args: []string{"-version"}})
		if err == nil {
			continue
		}
		return fmt.Errorf("%s not found in PATH. Install ffmpeg (includes ffprobe) and ensure it is in PATH.\n%s", bin, installHint())
	}
	return nil
}

func installHint() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows: choco install ffmpeg  OR  scoop install ffmpeg\nSee https://ffmpeg.org/download.html"
	case "darwin":
		return "macOS: brew install ffmpeg\nSee https://ffmpeg.org/download.html"
	default:
		return "Linux: install via your package manager (e.g. apt/yum/pacman)\nSee https://ffmpeg.org/download.html"
	}
}
