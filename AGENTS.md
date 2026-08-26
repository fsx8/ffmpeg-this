# ffwiz Project Context

## Project Overview

**ffwiz** (ffmpeg wizard) is a powerful and user-friendly terminal UI (TUI) built on top of FFmpeg. It provides an intuitive, menu-driven interface that allows users to perform common audio and video manipulation tasks without needing to memorize complex FFmpeg commands.

### Key Features

- **Inspect Media Properties**: Detailed information about video and audio streams
- **Join Videos**: Concatenate multiple videos with automatic resolution and sample rate handling
- **Trim Videos**: Lossless cutting of video clips
- **Extract Audio**: Rip audio tracks into various formats
- **Interactive Conversion**: Granular track-level control with codec selection

### Architecture

The project follows a modular architecture with the following structure:

- `cmd/ffwiz/`: Application entrypoint
- `internal/app/`: Bubble Tea screens and navigation (menus + wizards)
- `internal/ffprobe/`: ffprobe JSON parsing and probing
- `internal/ffmpeg/`: Deterministic FFmpeg argument generation for features
- `internal/media/`: Media file discovery helpers

### Technologies Used

- Go 1.22+
- FFmpeg (external dependency)
- Third-party libraries: bubbletea, bubbles, lipgloss

## Building and Running

### Prerequisites

- Python 3.8 or higher
- Go 1.22 or higher
- FFmpeg installed and available in system PATH

### Installation Methods

#### From Source

```bash
git clone https://github.com/fsx8/ffwiz.git
cd ffwiz
go run ./cmd/ffwiz
```

### Running the Application

- From source: `go run ./cmd/ffwiz`
- With a path: `ffwiz /path/to/video.mp4` or `ffwiz /path/to/folder`

## Development Conventions

### Code Structure

- Core FFmpeg argument generation lives in `internal/ffmpeg/`
- UI flows live in `internal/app/` and are built on Bubble Tea components
- ffprobe parsing and probing live in `internal/ffprobe/`

### Error Handling

- Comprehensive error handling with logging to `ffmpeg_log.txt`
- Graceful handling of keyboard interrupts (Ctrl+C)
- FFmpeg errors are captured and displayed to the user

### Testing

- Unit tests are run with Go’s built-in test tooling.
- Run locally: `go test ./...`
- CI runs the same command via `.github/workflows/tests.yml`
- Tests focus on deterministic command/argument generation (they do not execute FFmpeg).

### Contribution Guidelines

- Fork the repository and create a descriptive branch name
- Follow the existing code style and conventions
- Make pull requests to the main branch after updating from upstream

### Dependencies

- **Runtime**: bubbletea, bubbles, lipgloss
- Dependencies are specified in `go.mod`

## Key Files and Directories

- `go.mod`: Go module configuration and dependencies
- `README.md`: Main documentation
- `CONTRIBUTING.md`: Contribution guidelines
- `cmd/ffwiz/main.go`: Main application entry point
- `internal/`: Application implementation

## Important Notes

1. FFmpeg must be separately installed on the user's system - it's not bundled with the Python package
2. The application creates a log file (`ffmpeg_log.txt`) in the working directory for each session
3. The interactive conversion feature provides granular track-level control in media files
4. The application supports various video, audio, and subtitle codecs through FFmpeg
