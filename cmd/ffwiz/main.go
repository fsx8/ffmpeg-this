package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fsx8/ffwiz/internal/app"
	"github.com/fsx8/ffwiz/internal/execx"
	"github.com/fsx8/ffwiz/internal/ffprobe"
)

// version is set at build time via -ldflags "-X main.version=..." (goreleaser).
var version = "dev"

func effectiveVersion() string {
	if version != "dev" {
		return version
	}
	// go install does not pass ldflags; fall back to module version info.
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}

// openLogFile creates ffmpeg_log.txt in the working directory, falling back
// to a unique file in the system temp dir when the CWD is not writable
// (read-only mounts, owned directories, …). Returns nil if logging is
// impossible; a missing log must never keep the app from starting.
func openLogFile() *os.File {
	f, err := os.Create("ffmpeg_log.txt")
	if err == nil {
		return f
	}
	f, err = os.CreateTemp("", "ffwiz-ffmpeg_log-*.txt")
	if err != nil {
		return nil
	}
	fmt.Fprintln(os.Stderr, "note: working directory not writable; logging to", f.Name())
	return f
}

func main() {
	var initialPath string
	var showVersion bool
	flag.StringVar(&initialPath, "path", "", "optional file or directory to open directly")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Printf("ffwiz %s\n", effectiveVersion())
		return
	}

	if flag.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "usage: ffwiz [optional_file_or_folder]")
		os.Exit(2)
	}
	if initialPath == "" && flag.NArg() == 1 {
		initialPath = flag.Arg(0)
	}

	var logger *log.Logger
	if logFile := openLogFile(); logFile != nil {
		defer logFile.Close()
		logger = log.New(logFile, "", log.LstdFlags)
	}

	runner := execx.New()
	prober := ffprobe.New(runner)

	if err := app.CheckFFmpegFFprobe(context.Background(), runner); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	absInitial := ""
	if initialPath != "" {
		p, err := filepath.Abs(initialPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid path:", err)
			os.Exit(1)
		}
		if _, err := os.Stat(p); err != nil {
			fmt.Fprintln(os.Stderr, "error: path does not exist:", p)
			os.Exit(1)
		}
		absInitial = p
	}

	model := app.New(app.Config{
		Logger:      logger,
		Runner:      runner,
		Prober:      prober,
		InitialPath: absInitial,
	})

	prog := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		if !errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "fatal:", err)
		}
		os.Exit(1)
	}
}
