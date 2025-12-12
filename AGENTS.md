# ffmPEG-this Project Context

## Project Overview

**ffmPEG-this** (also known as **peg_this**) is a powerful and user-friendly command-line interface (CLI) built on top of FFmpeg. It provides an intuitive menu-driven interface that allows users to perform common audio and video manipulation tasks without needing to memorize complex FFmpeg commands.

### Key Features

- **Inspect Media Properties**: Detailed information about video and audio streams
- **Join Videos**: Concatenate multiple videos with automatic resolution and sample rate handling
- **Trim Videos**: Lossless cutting of video clips
- **Extract Audio**: Rip audio tracks into various formats
- **Interactive Conversion**: Granular track-level control with codec selection

### Architecture

The project follows a modular architecture with the following structure:

- `src/peg_this/`: Main Python package
  - `peg_this.py`: Main entry point with CLI menu system
  - `features/`: Modular feature implementations (audio, batch, convert, crop, inspect, join, trim)
  - `utils/`: Utility functions (FFmpeg utilities, UI utilities)

### Technologies Used

- Python 3.8+
- FFmpeg (external dependency)
- Third-party libraries: questionary, rich, Pillow, ffmpeg-python

## Building and Running

### Prerequisites

- Python 3.8 or higher
- FFmpeg installed and available in system PATH

### Installation Methods

#### 1. Pip Install (Recommended)

```bash
pip install peg_this
peg_this  # Run the application
```

#### 2. From Source

```bash
git clone https://github.com/hariharen9/ffmpeg-this.git
cd ffmpeg-this
pip install -r requirements.txt
python -m src.peg_this.peg_this
```

#### 3. Using PyInstaller (for executable)

The project includes PyInstaller in requirements for creating standalone executables.

### Running the Application

- If installed via pip: `peg_this`
- From source: `python -m src.peg_this.peg_this`

## Development Conventions

### Code Structure

- Functions are organized by feature in separate modules within the features directory
- Each feature module contains standalone functions for specific operations
- UI interactions are handled by questionary and rich libraries
- FFmpeg operations are performed using the ffmpeg-python library

### Error Handling

- Comprehensive error handling with logging to `ffmpeg_log.txt`
- Graceful handling of keyboard interrupts (Ctrl+C)
- FFmpeg errors are captured and displayed to the user

### Testing

- Unit tests live in `tests/` and are run with Python’s built-in `unittest` (no extra dev dependency).
- Run locally: `python -m unittest discover -s tests`
- CI runs the same command via `.github/workflows/tests.yml`
- Tests focus on deterministic command/argument generation (they do not execute FFmpeg).

### Contribution Guidelines

- Fork the repository and create a descriptive branch name
- Follow the existing code style and conventions
- Make pull requests to the main branch after updating from upstream

### Dependencies

- **Runtime**: questionary, rich, Pillow, ffmpeg-python
- **Build**: setuptools, pyinstaller
- All dependencies are specified in pyproject.toml and requirements.txt

## Key Files and Directories

- `pyproject.toml`: Package configuration and dependencies
- `requirements.txt`: Python dependencies
- `README.md`: Main documentation
- `CONTRIBUTING.md`: Contribution guidelines
- `INTERACTIVE_CONVERT_GUIDE.md`: Detailed guide for the interactive conversion feature
- `src/peg_this/peg_this.py`: Main application entry point
- `src/peg_this/features/`: Directory containing feature-specific implementations
- `src/peg_this/utils/`: Utility functions

## Important Notes

1. FFmpeg must be separately installed on the user's system - it's not bundled with the Python package
2. The application creates a log file (`ffmpeg_log.txt`) in the working directory for each session
3. The interactive conversion feature provides granular track-level control in media files
4. The application supports various video, audio, and subtitle codecs through FFmpeg
5. Visual cropping feature allows interactive selection of video regions
