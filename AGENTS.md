# ffwiz Project Context

## Project Overview

**ffwiz** (ffmpeg wizard) is a terminal UI (TUI) built on top of FFmpeg. It provides an intuitive, menu-driven interface for common audio and video manipulation tasks without needing to memorize FFmpeg commands. ffwiz is a standalone Go project (it originated as a fork of a Python tool; the history was restarted and the fork network left — MIT `LICENSE` intentionally retains the original copyright notice).

### Key Features

- **Inspect Media Properties**: Detailed information about video and audio streams
- **Join Videos**: Concatenate multiple videos with automatic resolution and sample rate handling
- **Trim Videos**: Lossless cutting (`-c copy`, output-side `-ss`/`-to` for ffmpeg 4.x compat)
- **Extract Audio**: Rip audio tracks to mp3/flac/wav
- **Interactive Conversion**: Granular per-track control (keep/remove/convert with codec selection)
- **Batch Conversion**: Convert a whole directory with quality presets (incl. 2-step GIF palette)

### Architecture

- `cmd/ffwiz/`: Application entrypoint (flag parsing, `-version`, `-path`)
- `internal/app/`: Bubble Tea screens and navigation (stack-based: menus + wizards + exec view)
- `internal/ffmpeg/`: Deterministic FFmpeg argument generation (one file per feature, no subprocesses here)
- `internal/ffprobe/`: ffprobe JSON parsing and probing
- `internal/execx/`: Process execution (buffered + streaming stderr, context-aware cancellation)
- `internal/media/`: Media file discovery helpers

### Technologies

- Go (see `go.mod` for the exact minimum version)
- FFmpeg/ffprobe as external runtime dependencies (not bundled)
- bubbletea, bubbles, lipgloss ( Charm )

## Building and Running

### Prerequisites

- Go toolchain (version per `go.mod`)
- FFmpeg + ffprobe in PATH (ffprobe ships with ffmpeg)

### From Source

```bash
git clone https://github.com/fsx8/ffwiz.git
cd ffwiz
go run ./cmd/ffwiz            # or: go build ./cmd/ffwiz
```

- Optional arg: `ffwiz [file_or_folder]` opens the action menu / join wizard directly
- `-version` prints the version (goreleaser ldflags, or module build info via `go install`)

## Development Conventions

### Git

- **Default branch: `main`** — the only development branch
- **`gh-pages` is auto-generated** (hosts the apt repository); never edit manually
- Commit identity must be `fsx8 <fsx8@users.noreply.github.com>` (already set in this repo's local git config — do not override with a global/other identity)
- History was deliberately restarted (single root commit); do not re-introduce old refs

### Code Style

- Core FFmpeg argument generation lives in `internal/ffmpeg/` and must stay deterministic (pure functions -> `Cmd{Args}`)
- UI flows live in `internal/app/` on Bubble Tea components; navigation via `push`/`pop`/`replace` cmds in `nav.go`
- All subprocess execution goes through `internal/execx` and must respect context cancellation (Ctrl+C/Esc must never orphan a running ffmpeg)

### Error Handling

- Errors are logged to `ffmpeg_log.txt` in the working directory
- Ctrl+C cancels in-flight ffmpeg; screens show streamed stderr in a viewport

### Testing

- `go test ./...` — deterministic command/argument generation tests; no FFmpeg execution
- CI: `.github/workflows/tests.yml` on every push/PR
- Before handing back work: `gofmt -l .`, `go vet ./...`, `go build ./...`, `go test ./...`

## Release & Distribution

Releasing = push a `v*` tag on `main`. goreleaser (`.goreleaser.yaml`) then builds everything; downstream workflows pick up the rest automatically.

| Channel | Mechanism |
|---|---|
| Binaries / `.deb` / `.rpm` | goreleaser -> GitHub Release assets (11 files) |
| Homebrew (macOS) | goreleaser `homebrew_casks` -> `fsx8/homebrew-tap` (needs `HOMEBREW_TAP_TOKEN` secret) |
| Docker | goreleaser `dockers_v2` -> `ghcr.io/fsx8/ffwiz` (multi-arch, ffmpeg bundled, `Dockerfile`) |
| apt (Ubuntu/Debian servers) | `apt.yml` workflow auto-runs on Release completion (`workflow_run` trigger — `on: release` does NOT fire because goreleaser publishes with `GITHUB_TOKEN`), signs and pushes the repo to `gh-pages` (needs `APT_GPG_PRIVATE_KEY` secret) |
| npm | manual: `gh workflow run npm.yml -f tag=vX.Y.Z` — uses OIDC **trusted publishing** (no npm token); stages README+LICENSE into `npm/` and rewrites relative links; registered publisher: fsx8/ffwiz, workflow `npm.yml` |
| go install | works automatically via the module path `github.com/fsx8/ffwiz/cmd/ffwiz` |

One-line installer (served from Pages): `curl -fsSL https://fsx8.github.io/ffwiz/install.sh | sudo bash` — source in `scripts/install.sh`, re-copied to `gh-pages` by `apt.yml` on each release.

Version injection: goreleaser sets `main.version` via ldflags; `go install` builds fall back to module build info (see `effectiveVersion()` in `cmd/ffwiz/main.go`).

## Key Files and Directories

- `go.mod` / `go.sum`: module `github.com/fsx8/ffwiz` and dependencies
- `.goreleaser.yaml`: builds, archives, nfpms, homebrew cask, docker
- `.github/workflows/`: `release.yml` (goreleaser), `apt.yml` (apt repo), `npm.yml` (npm publish), `tests.yml` (CI)
- `scripts/install.sh`: apt one-line installer
- `npm/`: npm wrapper package (binary-downloading launcher)
- `Dockerfile`: container image (used by goreleaser with prebuilt binaries)
- `gh-pages` branch: published apt repository (`apt/`, `KEY.gpg`, `install.sh`) — generated

## Important Notes

1. FFmpeg must be installed separately on the user's system (except in the Docker image, which bundles it)
2. The app creates `ffmpeg_log.txt` in the working directory each session
3. Trim uses output-side seeking deliberately — do not move `-ss`/`-to` before `-i` (breaks ffmpeg 4.x, e.g. Ubuntu 22.04)
4. Registry immutability: fixing a broken npm package or README requires a new version tag, never a force-publish
