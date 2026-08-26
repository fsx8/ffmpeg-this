# 🎬 ffwiz

<p align="center">
    <a href="https://github.com/fsx8/ffwiz/releases">
        <img src="https://img.shields.io/github/v/release/fsx8/ffwiz?label=release" alt="GitHub Release">
    </a>
    <a href="https://github.com/fsx8/ffwiz/blob/main/LICENSE">
        <img src="https://img.shields.io/github/license/fsx8/ffwiz" alt="License">
    </a>
</p>

> Your Video editor within CLI 🚀

A powerful and user-friendly TUI (Bubble Tea) for converting, manipulating, and inspecting media files using the power of FFmpeg. It provides a simple menu-driven interface to perform common audio and video tasks without needing to memorize complex FFmpeg commands.

## ✨ Features

- **Inspect Media Properties**: View detailed information about video and audio streams, including codecs, resolution, frame rate, bitrates, and more.
- **Convert & Transcode**: Convert videos and audio to a wide range of popular formats (MP4, MKV, WebM, MP3, FLAC, WAV, GIF) with simple quality presets.
- **Join Videos (Concatenate)**: Combine two or more videos into a single file. The tool automatically handles differences in resolution and audio sample rates for a seamless join.
- **Trim (Cut) Videos**: Easily cut a video to a specific start and end time without re-encoding for fast, lossless clips.
- **Interactive Track Editing**: Keep/remove/convert individual video/audio/subtitle tracks and generate an FFmpeg command deterministically.
- **TUI Interface**: A modern, keyboard-friendly terminal UI built with Bubble Tea.

## 🚀 Usage

### Prerequisite: Install FFmpeg

> [NOTE] > `ffwiz` shells out to the `ffmpeg` and `ffprobe` executables. It does not bundle FFmpeg. Therefore, you must have FFmpeg installed on your system and available in your terminal's PATH.

For **macOS** users, the easiest way to install it is with [Homebrew](https://brew.sh/):

```bash
brew install ffmpeg
```

For **Windows** users, you can use a package manager like [Chocolatey](https://chocolatey.org/) or [Scoop](https://scoop.sh/):

```bash
# Using Chocolatey
choco install ffmpeg

# Using Scoop
scoop install ffmpeg
```

For other systems, please see the official download page: **[ffmpeg.org/download.html](https://ffmpeg.org/download.html)**

There are several ways to use `ffwiz`:

### 1. Homebrew (macOS)

```bash
brew install fsx8/tap/ffwiz
```

### 2. Debian/Ubuntu — APT repository (recommended for servers)

One command — adds the signed repository and installs (or upgrades) `ffwiz`:

```bash
curl -fsSL https://fsx8.github.io/ffwiz/install.sh | sudo bash
```

Works on any apt-based system (Ubuntu, Debian, derivatives) on `amd64` or `arm64`. The repository is GPG-signed and auto-published on every GitHub Release by `.github/workflows/apt.yml`, so `sudo apt upgrade` picks up new versions.

<details>
<summary>Manual setup (what the script does)</summary>

```bash
curl -fsSL https://fsx8.github.io/ffwiz/KEY.gpg | sudo gpg --dearmor -o /usr/share/keyrings/ffwiz-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/ffwiz-archive-keyring.gpg] https://fsx8.github.io/ffwiz/apt stable main" | sudo tee /etc/apt/sources.list.d/ffwiz.list
sudo apt update && sudo apt install ffwiz
```
</details>

For a one-off install without adding the repo, you can also `apt install` a `.deb` directly:

```bash
# Alternative: direct .deb install (no apt upgrade path)
curl -LO https://github.com/fsx8/ffwiz/releases/latest/download/ffwiz_<version>_linux_<arch>.deb
sudo apt install ./ffwiz_<version>_linux_<arch>.deb
```

### 3. Docker

```bash
docker run -it --rm -v "$PWD:/media" ghcr.io/fsx8/ffwiz
```

Multi-arch image (`linux/amd64`, `linux/arm64`) built from `Dockerfile` on every release, with ffmpeg bundled. Useful as a base image layer too: `FROM ghcr.io/fsx8/ffwiz`.

### 4. go install

```bash
go install github.com/fsx8/ffwiz/cmd/ffwiz@latest
```

Requires a Go toolchain; the binary lands in `$(go env GOPATH)/bin` (make sure it's on your PATH). ffmpeg must still be installed separately.

### 5. npm (optional)

```bash
npm install -g ffwiz
```

A thin wrapper package that downloads the matching prebuilt binary from the latest release at install time (needs `tar`, which ships with macOS, Linux, and Windows 10+).

### 6. Download from Release

If you prefer not to install the package, you can download a pre-built executable from the [Releases](https://github.com/fsx8/ffwiz/releases/latest) page.

1.  Download the executable for your operating system (Windows, macOS, or Linux).
2.  Place it in a directory with your media files.
3.  Run the executable directly from your terminal.

### 7. Run from Source

If you want to run the script directly from the source code:

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/fsx8/ffwiz.git
    cd ffwiz
    ```
2.  **Run the app:**
    ```bash
    go run ./cmd/ffwiz
    ```

You can also pass an optional file or folder:

```bash
ffwiz /path/to/video.mp4
ffwiz /path/to/folder
```

## 📈 Star History

<p align="center">
  <a href="https://star-history.com/#fsx8/ffwiz&Date">
    <img src="https://api.star-history.com/svg?repos=fsx8/ffwiz&type=Date" alt="Star History Chart">
  </a>
</p>

## ✨ Sponsor

<p align="center">
    <a href="https://github.com/sponsors/fsx8">
        <img src="https://img.shields.io/github/sponsors/fsx8?style=for-the-badge&logo=github&color=white" alt="GitHub Sponsors">
    </a>
</p>

## 👥 Contributors

<a href="https://github.com/fsx8/ffwiz/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=fsx8/ffwiz" />
</a>

## 🤝 Contributing

Contributions are welcome! Please see the [Contributing Guidelines](CONTRIBUTING.md) for more information.

## 🧪 Unit Testing

The repository includes fast unit tests that validate FFmpeg argument generation without running FFmpeg.

```bash
go test ./...
```

## 📄 License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

<p align="center">
    <h2>Made with ❤️ by <a href="https://github.com/fsx8">fsx8</a></h2>
</p>
