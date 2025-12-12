import os
import sys
import logging
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

import questionary
from rich.console import Console

from peg_this.features.audio import extract_audio
from peg_this.features.interactive_convert import convert_file_interactive
from peg_this.features.inspect import inspect_file
from peg_this.features.join import join_videos
from peg_this.features.trim import trim_video
from peg_this.utils.ffmpeg_utils import check_ffmpeg_ffprobe
from peg_this.utils.ui_utils import select_media_file

# --- Global Configuration ---
# Configure logging
log_file = os.path.join(os.path.dirname(os.path.realpath(__file__)), "ffmpeg_log.txt")
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler(log_file, mode='w') # Overwrite log on each run
    ]
)

# Initialize Rich Console
console = Console()
# --- End Global Configuration ---


def action_menu(file_path):
    """Display the menu of actions for a selected file."""
    while True:
        console.rule(f"[bold]Actions for: {os.path.basename(file_path)}[/bold]")
        action = questionary.select(
            "Choose an action:",
            choices=[
                "Inspect File Details",
                "Modify Tracks",
                "Trim Video",
                "Extract Audio",
                questionary.Separator(),
                "Go back to file list"
            ],
            use_indicator=True
        ).ask()

        if action is None or action == "Go back to file list":
            break

        actions = {
            "Inspect File Details": inspect_file,
            "Modify Tracks": convert_file_interactive,
            "Trim Video": trim_video,
            "Extract Audio": extract_audio,
        }
        # Ensure we have a valid action before calling
        if action in actions:
            try:
                result = actions[action](file_path)
                # If the action returns "quit_to_main", we should return to main menu
                if result == "quit_to_main":
                    return
            except KeyboardInterrupt:
                console.print("\n[bold yellow]Operation cancelled by user.[/bold yellow]")
                break


def main_menu():
    """Display the main menu."""
    check_ffmpeg_ffprobe()
    while True:
        console.rule("[bold magenta]ffmPEG-this[/bold magenta]")
        choice = questionary.select(
            "What would you like to do?",
            choices=[
                "Process a Single Media File",
                "Join Multiple Videos",
                "Exit"
            ],
            use_indicator=True
        ).ask()

        if choice is None or choice == "Exit":
            console.print("[bold]Goodbye![/bold]")
            console.print("\n[italic cyan]Built with ❤️ by Hariharen[/italic cyan]")
            break
        elif choice == "Process a Single Media File":
            selected_file = select_media_file()
            if selected_file:
                action_menu(selected_file)
        elif choice == "Join Multiple Videos":
            try:
                join_videos()
            except KeyboardInterrupt:
                console.print("\n[bold yellow]Operation cancelled by user.[/bold yellow]")


def main():
    """Main entry point for the application script."""
    try:
        # Check command line arguments
        if len(sys.argv) == 1:
            # No arguments provided, show main menu
            main_menu()
        elif len(sys.argv) == 2:
            # One argument provided: treat as a file path (process) or directory (join)
            input_path = sys.argv[1]

            if os.path.isdir(input_path):
                console.print(f"[bold green]Joining videos in: {os.path.abspath(input_path)}[/bold green]")
                join_videos(input_path)
                return

            file_path = input_path
            
            # Validate that the file exists
            if not os.path.isfile(file_path):
                console.print(f"[bold red]Error: File '{file_path}' does not exist.[/bold red]")
                return
            
            # Skip the main menu and go directly to action menu for the provided file
            console.print(f"[bold green]Processing file: {os.path.basename(file_path)}[/bold green]")
            action_menu(file_path)
        else:
            # More than one argument provided, show an error
            console.print(f"[bold red]Error: Too many arguments provided. Usage: {sys.argv[0]} [optional_file_or_folder][/bold red]")
            return
    except (KeyboardInterrupt, EOFError):
        logging.info("Operation cancelled by user.")
        console.print("[bold]Operation cancelled. Goodbye![/bold]")
    except Exception as e:
        logging.exception("An unexpected error occurred.")
        console.print(f"[bold red]An unexpected error occurred: {e}[/bold red]")
        console.print(f"Details have been logged to {log_file}")

if __name__ == "__main__":
    main()
