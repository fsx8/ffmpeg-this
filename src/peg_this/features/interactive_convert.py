import os
import sys
from pathlib import Path
from typing import List, Dict, Any, Optional

import ffmpeg
import questionary
from rich.console import Console
from rich.table import Table
from rich.panel import Panel
from rich import box

from peg_this.utils.ffmpeg_utils import (
    parse_media_tracks, 
    get_codec_options, 
    get_default_codec,
    run_command
)

console = Console()

class TrackAction:
    """Track action constants."""
    REMOVE = "remove"
    KEEP = "keep" 
    CONVERT = "convert"

class InteractiveConverter:
    """Interactive track-based converter with multi-level menus."""
    
    def __init__(self, file_path: str):
        self.file_path = file_path
        self.tracks = []
        self.track_actions = {}  # {track_id: {'action': action, 'codec': codec}}
        self.output_path: Optional[Path] = None

    @staticmethod
    def _clean_codec_choice(selected_option: str) -> str:
        """
        Normalize a UI codec choice into an ffmpeg codec name.

        Examples:
        - "libx264 (H.264)" -> "libx264"
        - "srt (SubRip)" -> "srt"
        - "(SubRip)" -> "srt" (legacy)
        """
        if not selected_option:
            return selected_option

        cleaned = selected_option.strip()
        if cleaned.startswith("(") and cleaned.endswith(")"):
            inner = cleaned[1:-1].strip().lower()
            if "subrip" in inner:
                return "srt"
            if inner == "ass":
                return "ass"
            if "mp4" in inner:
                return "mov_text"
            return inner

        return cleaned.split(" ", 1)[0].strip()

    def extract_tracks(self):
        """Extract all tracks from the media file."""
        console.print("[bold cyan]Analyzing media file...[/bold cyan]")
        self.tracks = parse_media_tracks(self.file_path)
        
        if not self.tracks:
            console.print("[bold red]No tracks found in the media file.[/bold red]")
            return False
            
        # Initialize all tracks with KEEP action by default
        for track_id, _track in enumerate(self.tracks):
            if track_id not in self.track_actions:
                self.track_actions[track_id] = {'action': TrackAction.KEEP}
        
        console.print(f"[bold green]Found {len(self.tracks)} tracks[/bold green]")
        return True
        
    def get_track_display_text(self, track: Dict[str, Any], index: int) -> str:
        """Get formatted display text for a track."""
        track_type_icon = {
            'video': '🎬',
            'audio': '🎵',
            'subtitle': '📝'
        }.get(track['type'], '❓')
        
        basic_info = f"[{index}] {track_type_icon} {track['type'].upper()} - {track['codec']}"
            
        if track['type'] == 'video':
            resolution = f"{track.get('width', 0)}x{track.get('height', 0)}"
            fps = f"{track.get('fps', 0):.2f}" if track.get('fps') else "unknown"
            duration = track.get('duration', 0)
            if duration:
                duration = f"{float(duration):.1f}s"
            return f"{basic_info} | {resolution} | {fps}fps | {duration}"
            
        elif track['type'] == 'audio':
            channels = track.get('channels', 0)
            sample_rate = track.get('sample_rate', 0)
            bit_rate = track.get('bit_rate', 'unknown')
            duration = track.get('duration', 0)
            if duration:
                duration = f"{float(duration):.1f}s"
            language = track.get('language', 'und')
            if language == 'und':
                language = 'unknown'
            return f"{basic_info} | {channels}ch | {sample_rate}Hz | {bit_rate} | {duration} | {language}"
            
        else:  # subtitle
            language = track.get('language', 'und')
            if language == 'und':
                language = 'unknown'
            title = track.get('title', '')
            return f"{basic_info} | {language} | {title}"
            
    def show_track_selection_menu(self):
        """Show the main track selection menu with arrow navigation and keyboard shortcuts."""
        # Track the currently selected track index
        current_selection = 0
        
        while True:
            console.clear()
            self._show_header()
            
            # Show instructions for keyboard shortcuts
            console.print("\n[bold]Keyboard Controls:[/bold]")
            console.print("  [↑][↓]: Navigate between tracks")
            console.print("  [R]emove, [K]eep, [C]onvert: Apply to currently selected track")
            console.print("  [Enter]: Continue with conversion")
            console.print("  [←]: Go back to main menu")
            
            # Show tracks with the currently selected one highlighted
            console.print("\n[bold]Tracks:[/bold]")
            
            for i, track in enumerate(self.tracks):
                display_text = self.get_track_display_text(track, i)
                
                # Add action indicator
                if i in self.track_actions:
                    action = self.track_actions[i]['action']
                    if action == TrackAction.REMOVE:
                        display_text += " [REMOVE]"
                    elif action == TrackAction.KEEP:
                        display_text += " [KEEP]"
                    elif action == TrackAction.CONVERT:
                        codec = self.track_actions[i].get('codec', 'unknown')
                        display_text += f" [CONVERT: {codec}]"
                else:
                    display_text += " [Not set]"
                
                # Highlight the currently selected track
                if i == current_selection:
                    console.print(f"> [bold yellow]{i}[/bold yellow] {display_text}")
                else:
                    console.print(f"  [bold]{i}[/bold] {display_text}")
            
            try:
                # Import here to avoid conflicts with questionary in some environments
                import termios, tty, sys
                fd = sys.stdin.fileno()
                
                # Save current terminal settings
                old_settings = termios.tcgetattr(fd)
                
                try:
                    # Change terminal to raw mode to read single characters
                    tty.setraw(fd)
                    ch = sys.stdin.read(1)
                    
                    # Handle ANSI escape sequences for arrow keys
                    if ord(ch) == 27:  # ESC sequence
                        ch = sys.stdin.read(1)  # Read [
                        if ch == '[':
                            ch = sys.stdin.read(1)  # Read actual key
                            if ch == 'A':  # Up arrow
                                current_selection = max(0, current_selection - 1)
                                continue
                            elif ch == 'B':  # Down arrow
                                current_selection = min(len(self.tracks) - 1, current_selection + 1)
                                continue
                            elif ch == 'C':  # Right arrow - do nothing
                                continue  # Just continue the loop without changing anything
                            elif ch == 'D':  # Left arrow - go back to main menu
                                # Return a special value to indicate going back to main menu
                                return "back_to_main"

                    
                    # Handle regular character inputs
                    if ch.lower() == 'r':
                        # Apply Remove action to currently selected track
                        self.track_actions[current_selection] = {'action': TrackAction.REMOVE}
                        console.print(f"\n[green]Track {current_selection} marked for removal[/green]")
                    elif ch.lower() == 'k':
                        # Apply Keep action to currently selected track
                        self.track_actions[current_selection] = {'action': TrackAction.KEEP}
                        console.print(f"\n[green]Track {current_selection} marked to keep[/green]")
                    elif ch.lower() == 'c':
                        # Apply Convert action to currently selected track
                        self._show_codec_selection_menu(current_selection)
                    elif ord(ch) == 13 or ord(ch) == 10:  # Enter key
                        # Continue with conversion - break the loop normally
                        break
                    else:
                        # Any other key just refreshes the display
                        continue
                        
                finally:
                    # Restore original terminal settings
                    termios.tcsetattr(fd, termios.TCSADRAIN, old_settings)
                    
            except (KeyboardInterrupt, EOFError):
                # Handle Ctrl+C or EOF by propagating the cancellation
                raise  # Re-raise the exception to be handled by the calling function
        
        # If we get here, either Enter was pressed (break) or Ctrl+C (exception)
        # Return None to indicate normal flow should continue
        return None

    def _show_header(self):
        """Show the header panel."""
        file_name = os.path.basename(self.file_path)
        panel = Panel(
            f"[bold]File:[/bold] {file_name}\n",
            title="🎬 Interactive Modify Tracks",
            border_style="cyan"
        )
        console.print(panel)
        
    def _get_menu_choices(self):
        """Get the choices for the main menu."""
        choices = []
        
        # Add track selections
        for i, track in enumerate(self.tracks):
            display_text = self.get_track_display_text(track, i)
            # Add action indicator
            if i in self.track_actions:
                action = self.track_actions[i]['action']
                if action == TrackAction.REMOVE:
                    display_text += " [REMOVE]"
                elif action == TrackAction.KEEP:
                    display_text += " [KEEP]"
                elif action == TrackAction.CONVERT:
                    codec = self.track_actions[i]['codec']
                    display_text += f" [CONVERT: {codec}]"
            else:
                display_text += " [Not set]"
            
            choices.append(display_text)
            
        choices.append(questionary.Separator())
        choices.append("Continue with conversion")
        choices.append("Go back to main menu")
        
        return choices
                
    def _show_codec_selection_menu(self, track_index: int):
        """Show codec selection for a specific track with questionary."""
        track = self.tracks[track_index]
        track_type = track['type']
        
        default_codec = get_default_codec(track_type)
        codec_choices = get_codec_options(track_type) + [questionary.Separator(), "Go back to tracks"]
        
        console.clear()
        self._show_header()

        selected_option = questionary.select(
            f"Select target codec for [bold]Track #{track_index}[/bold] ({track_type}|{track['codec']}):",
            choices=codec_choices,
            use_indicator=True
        ).ask()
        
        if selected_option is None or selected_option == "Go back to tracks":
            return
        
        # Process the selected codec
        selected_codec_clean = self._clean_codec_choice(selected_option)

        self.track_actions[track_index] = {
            'action': TrackAction.CONVERT,
            'codec': selected_codec_clean
        }
        
        return  # Return None after processing
                
    def configure_output_file(self):
        """Configure the output filename."""
        input_path = Path(self.file_path)
        suggested_name = f"{input_path.stem}_modified{input_path.suffix}"
        
        console.clear()
        self._show_header()
        
        console.print("\n[bold]Output File Configuration[/bold]")
        console.print(f"Input file: {input_path.name}")
        console.print(f"Suggested output: {suggested_name}")
        
        output_name = questionary.text(
            "Enter output filename:",
            default=suggested_name
        ).ask()
        
        if not output_name:
            output_name = suggested_name
            
        # Ensure output directory exists
        output_dir = Path.cwd()
        self.output_path = output_dir / output_name
        
        console.print(f"[bold green]Output will be saved as: {self.output_path}[/bold green]")
        return True
        
    def generate_ffmpeg_command(self) -> Optional[Any]:
        """Generate the ffmpeg command based on track actions."""
        try:
            if not self.output_path:
                console.print("[bold red]Error: Output path not configured[/bold red]")
                return None

            input_stream = ffmpeg.input(self.file_path)
            output_args = {'y': None}
            
            # Build streams list and codec parameters for output
            video_streams = []
            audio_streams = []
            subtitle_streams = []
            
            # Counters for stream-specific parameters in the final command
            video_idx_counter = 0
            audio_idx_counter = 0
            subtitle_idx_counter = 0
            
            # Process each track and build streams list with appropriate codec parameters
            for track_id, track in enumerate(self.tracks):
                stream_index = track['index']
                action_info = self.track_actions.get(track_id, {})
                action = action_info.get('action', TrackAction.KEEP)
                
                if action == TrackAction.REMOVE:
                    continue  # Skip this stream
                elif action == TrackAction.KEEP:
                    # For KEEP action, process with copy codec
                    if track['type'] == 'video':
                        stream = self._process_video_stream(input_stream, stream_index, 'copy')
                        if stream:
                            video_streams.append(stream)
                            # Add copy codec parameter for this video stream (by index in output)
                            output_args[f'c:v:{video_idx_counter}'] = 'copy'
                        video_idx_counter += 1
                    elif track['type'] == 'audio':
                        stream = self._process_audio_stream(input_stream, stream_index, 'copy')
                        if stream:
                            audio_streams.append(stream)
                            # Add copy codec parameter for this audio stream (by index in output)
                            output_args[f'c:a:{audio_idx_counter}'] = 'copy'
                        audio_idx_counter += 1
                    elif track['type'] == 'subtitle':
                        stream = self._process_subtitle_stream(input_stream, stream_index, 'copy')
                        if stream:
                            subtitle_streams.append(stream)
                            # Add copy codec parameter for this subtitle stream (by index in output)
                            output_args[f'c:s:{subtitle_idx_counter}'] = 'copy'
                        subtitle_idx_counter += 1
                elif action == TrackAction.CONVERT:
                    # For CONVERT action, set per-stream output codec args
                    codec = self._clean_codec_choice(action_info.get('codec', ''))
                    if track['type'] == 'video':
                        stream = self._process_video_stream(input_stream, stream_index, codec)
                        if stream:
                            video_streams.append(stream)
                            self._set_video_output_args(output_args, video_idx_counter, codec)
                        video_idx_counter += 1
                    elif track['type'] == 'audio':
                        stream = self._process_audio_stream(input_stream, stream_index, codec)
                        if stream:
                            audio_streams.append(stream)
                            self._set_audio_output_args(output_args, audio_idx_counter, codec)
                        audio_idx_counter += 1
                    elif track['type'] == 'subtitle':
                        stream = self._process_subtitle_stream(input_stream, stream_index, codec)
                        if stream:
                            subtitle_streams.append(stream)
                            self._set_subtitle_output_args(output_args, subtitle_idx_counter, codec)
                        subtitle_idx_counter += 1
            
            # Build output stream list - first video, then audio, then subtitles to match index order
            output_streams = []
            output_streams.extend(video_streams)
            output_streams.extend(audio_streams) 
            output_streams.extend(subtitle_streams)
            
            if not output_streams:
                return None
                
            return ffmpeg.output(*output_streams, str(self.output_path), **output_args)
            
        except Exception as e:
            console.print(f"[bold red]Error generating ffmpeg command: {e}[/bold red]")
            return None
            
    @staticmethod
    def _set_video_output_args(output_args: Dict[str, Any], output_index: int, codec: str) -> None:
        codec_l = (codec or "").lower()
        if codec_l in {"copy", ""}:
            output_args[f"c:v:{output_index}"] = "copy"
            return

        output_args[f"c:v:{output_index}"] = codec
        if codec_l == "libx264":
            output_args[f"crf:v:{output_index}"] = 23
            output_args[f"preset:v:{output_index}"] = "medium"
            output_args[f"pix_fmt:v:{output_index}"] = "yuv420p"
        elif codec_l == "libx265":
            output_args[f"crf:v:{output_index}"] = 28
            output_args[f"preset:v:{output_index}"] = "medium"

    @staticmethod
    def _set_audio_output_args(output_args: Dict[str, Any], output_index: int, codec: str) -> None:
        codec_l = (codec or "").lower()
        if codec_l in {"copy", ""}:
            output_args[f"c:a:{output_index}"] = "copy"
            return

        output_args[f"c:a:{output_index}"] = codec
        if codec_l in {"aac", "libmp3lame", "libfdk_aac", "libvorbis"}:
            output_args[f"b:a:{output_index}"] = "192k"
        elif codec_l == "libopus":
            output_args[f"b:a:{output_index}"] = "160k"

    @staticmethod
    def _set_subtitle_output_args(output_args: Dict[str, Any], output_index: int, codec: str) -> None:
        codec_l = (codec or "").lower()
        if codec_l in {"copy", ""}:
            output_args[f"c:s:{output_index}"] = "copy"
            return

        if "subrip" in codec_l:
            codec_l = "srt"
        output_args[f"c:s:{output_index}"] = codec_l

    def _process_video_stream(self, input_stream, track_index: int, codec: str):
        """Process video stream with codec conversion."""
        try:
            return input_stream[str(track_index)]  # Use direct indexing by stream index
        except Exception as e:
            console.print(f"[bold yellow]Warning: Could not process video stream {track_index}: {e}[/bold yellow]")
            return input_stream[str(track_index)]
            
    def _process_audio_stream(self, input_stream, track_index: int, codec: str):
        """Process audio stream with codec conversion."""
        try:
            return input_stream[str(track_index)]  # Use direct indexing by stream index
        except Exception as e:
            console.print(f"[bold yellow]Warning: Could not process audio stream {track_index}: {e}[/bold yellow]")
            return input_stream[str(track_index)]
            
    def _process_subtitle_stream(self, input_stream, track_index: int, codec: str):
        """Process subtitle stream with codec conversion."""
        try:
            return input_stream[str(track_index)]  # Use direct indexing by stream index
        except Exception as e:
            console.print(f"[bold yellow]Warning: Could not process subtitle stream {track_index}: {e}[/bold yellow]")
            return input_stream[str(track_index)]
            
    def convert_file(self):
        """Main conversion process."""
        try:
            if not self.extract_tracks():
                return False
                
            # Interactive track selection
            result = self.show_track_selection_menu()
            
            # Check if user chose to go back to main menu
            if result == "back_to_main":
                console.print("[bold yellow]Returning to main menu.[/bold yellow]")
                return "quit_to_main"  # Return special value to indicate going back to main menu
                
            # Configure output
            if not self.configure_output_file():
                return False
                
            # Generate ffmpeg command
            console.print("\n[bold cyan]Generating ffmpeg command...[/bold cyan]")
            ffmpeg_command = self.generate_ffmpeg_command()
            
            if not ffmpeg_command:
                console.print("[bold red]Failed to generate ffmpeg command[/bold red]")
                questionary.press_any_key_to_continue().ask()
                return False
            
            # Show and get approval for the command before execution
            command_args = ' '.join(ffmpeg_command.get_args())
            full_command = f"ffmpeg {command_args}"
            
            console.print("\n[bold]Generated FFmpeg Command:[/bold]")
            console.print(f"[yellow]{full_command}[/yellow]")
            
            confirm = questionary.confirm(
                "Do you want to execute this command?",
                default=True
            ).ask()
            
            if not confirm:
                console.print("[bold yellow]Command execution cancelled by user.[/bold yellow]")
                return False
                
            # Execute conversion
            result = run_command(ffmpeg_command, f"Converting {os.path.basename(self.file_path)}...", show_progress=True)
            if result is not None:  # Success or user cancelled during execution
                if result:  # Success
                    console.print(f"[bold green]Successfully converted to {self.output_path}[/bold green]")
                else:  # Failed
                    console.print("[bold red]Conversion failed[/bold red]")
                questionary.press_any_key_to_continue().ask()
                return result is not None
            else:  # User cancelled during conversion
                console.print("\n[bold yellow]Operation cancelled by user.[/bold yellow]")
                return False
        except KeyboardInterrupt:
            console.print("\n[bold yellow]Operation cancelled by user.[/bold yellow]")
            questionary.press_any_key_to_continue().ask()
            return False


def convert_file_interactive(file_path: str):
    """Main entry point for interactive conversion."""
    converter = InteractiveConverter(file_path)
    result = converter.convert_file()
    
    # If the result indicates we should quit to main menu, propagate this
    if result == "quit_to_main":
        return "quit_to_main"
    return result
