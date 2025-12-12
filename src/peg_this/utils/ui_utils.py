
import os
from pathlib import Path

import questionary
from rich.console import Console

console = Console()


def get_media_files(directory="."):
    """Scan a directory for media files."""
    media_extensions = [".mkv", ".mp4", ".avi", ".mov", ".webm", ".flv", ".wmv", ".mp3", ".flac", ".wav", ".ogg", ".gif"]
    if not os.path.isdir(directory):
        return []
    files = [
        f
        for f in os.listdir(directory)
        if os.path.isfile(os.path.join(directory, f)) and Path(f).suffix.lower() in media_extensions
    ]
    return files


def select_media_file(directory="."):
    """Display a menu to select a media file."""
    media_files = get_media_files(directory)
    if not media_files:
        console.print("[bold yellow]No media files found in this directory.[/bold yellow]")
        manual_path = questionary.text("Enter the path to a media file (or press Enter to skip):").ask()
        if manual_path and os.path.isfile(manual_path):
            return os.path.abspath(manual_path)
        return None

    choices = media_files + [questionary.Separator(), "Specify different file", "Go back"]
    file = questionary.select("Select a media file to process:", choices=choices, use_indicator=True).ask()
    
    if file == "Specify different file":
        manual_path = questionary.text("Enter the path to a media file:").ask()
        if manual_path and os.path.isfile(manual_path):
            return os.path.abspath(manual_path)
        return None
    
    # Return the absolute path to prevent "file not found" errors
    return os.path.abspath(os.path.join(directory, file)) if file and file != "Go back" else None
