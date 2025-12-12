import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "src"))

from peg_this.features.interactive_convert import InteractiveConverter, TrackAction  # noqa: E402


def _maps(args):
    return [args[i + 1] for i, arg in enumerate(args) if arg == "-map" and i + 1 < len(args)]

def _flag_value(args, flag):
    for i, arg in enumerate(args):
        if arg == flag and i + 1 < len(args):
            return args[i + 1]
    return None


class TestInteractiveConvertCommand(unittest.TestCase):
    def _converter(self, tracks=None):
        converter = InteractiveConverter("input.mkv")
        converter.output_path = Path("output.mkv")
        converter.tracks = tracks or [
            {"index": 0, "type": "video", "codec": "h264"},
            {"index": 2, "type": "audio", "codec": "aac"},
            {"index": 5, "type": "subtitle", "codec": "subrip"},
        ]
        converter.track_actions = {}
        return converter

    def test_keep_all_tracks_with_gapped_indexes(self):
        converter = self._converter()
        cmd = converter.generate_ffmpeg_command()
        self.assertIsNotNone(cmd)

        args = cmd.get_args()
        self.assertEqual(_maps(args), ["0:0", "0:2", "0:5"])
        self.assertIn("-c:v:0", args)
        self.assertIn("copy", args)
        self.assertIn("-c:a:0", args)
        self.assertIn("-c:s:0", args)

    def test_all_tracks_removed_returns_none(self):
        converter = self._converter()
        converter.track_actions = {
            0: {"action": TrackAction.REMOVE},
            1: {"action": TrackAction.REMOVE},
            2: {"action": TrackAction.REMOVE},
        }
        self.assertIsNone(converter.generate_ffmpeg_command())

    def test_remove_video_only_outputs_audio_and_subs(self):
        converter = self._converter()
        converter.track_actions = {0: {"action": TrackAction.REMOVE}}
        cmd = converter.generate_ffmpeg_command()
        self.assertIsNotNone(cmd)

        args = cmd.get_args()
        self.assertEqual(_maps(args), ["0:2", "0:5"])
        self.assertNotIn("-c:v:0", args)
        self.assertEqual(_flag_value(args, "-c:a:0"), "copy")
        self.assertEqual(_flag_value(args, "-c:s:0"), "copy")

    def test_remove_audio_track_by_track_id(self):
        converter = self._converter()
        converter.track_actions = {1: {"action": TrackAction.REMOVE}}
        cmd = converter.generate_ffmpeg_command()
        self.assertIsNotNone(cmd)

        args = cmd.get_args()
        self.assertEqual(_maps(args), ["0:0", "0:5"])
        self.assertNotIn("-c:a:0", args)

    def test_convert_audio_sets_codec_and_bitrate(self):
        converter = self._converter()
        converter.track_actions = {1: {"action": TrackAction.CONVERT, "codec": "aac"}}
        cmd = converter.generate_ffmpeg_command()
        self.assertIsNotNone(cmd)

        args = cmd.get_args()
        self.assertIn("-c:a:0", args)
        self.assertIn("aac", args)
        self.assertIn("-b:a:0", args)
        self.assertIn("192k", args)

    def test_convert_audio_from_ui_choice_is_normalized(self):
        converter = self._converter()
        converter.track_actions = {1: {"action": TrackAction.CONVERT, "codec": "libmp3lame (MP3)"}}
        cmd = converter.generate_ffmpeg_command()
        self.assertIsNotNone(cmd)

        args = cmd.get_args()
        self.assertEqual(_flag_value(args, "-c:a:0"), "libmp3lame")
        self.assertEqual(_flag_value(args, "-b:a:0"), "192k")

    def test_multiple_audio_keep_and_convert_indexes_compact(self):
        tracks = [
            {"index": 0, "type": "video", "codec": "h264"},
            {"index": 1, "type": "audio", "codec": "aac"},
            {"index": 4, "type": "audio", "codec": "dts"},
            {"index": 6, "type": "subtitle", "codec": "subrip"},
        ]
        converter = self._converter(tracks=tracks)
        converter.track_actions = {2: {"action": TrackAction.CONVERT, "codec": "libopus (Opus)"}}
        cmd = converter.generate_ffmpeg_command()
        self.assertIsNotNone(cmd)

        args = cmd.get_args()
        self.assertEqual(_maps(args), ["0:0", "0:1", "0:4", "0:6"])
        self.assertEqual(_flag_value(args, "-c:a:0"), "copy")
        self.assertEqual(_flag_value(args, "-c:a:1"), "libopus")
        self.assertEqual(_flag_value(args, "-b:a:1"), "160k")

    def test_convert_subtitle_choice_is_normalized(self):
        converter = self._converter()
        converter.track_actions = {2: {"action": TrackAction.CONVERT, "codec": "srt (SubRip)"}}
        cmd = converter.generate_ffmpeg_command()
        self.assertIsNotNone(cmd)

        args = cmd.get_args()
        self.assertIn("-c:s:0", args)
        self.assertIn("srt", args)

    def test_convert_subtitle_mov_text_choice_is_normalized(self):
        converter = self._converter()
        converter.track_actions = {2: {"action": TrackAction.CONVERT, "codec": "mov_text (MP4)"}}
        cmd = converter.generate_ffmpeg_command()
        self.assertIsNotNone(cmd)

        args = cmd.get_args()
        self.assertEqual(_flag_value(args, "-c:s:0"), "mov_text")

    def test_convert_subtitle_legacy_choice_is_normalized(self):
        converter = self._converter()
        converter.track_actions = {2: {"action": TrackAction.CONVERT, "codec": "(SubRip)"}}
        cmd = converter.generate_ffmpeg_command()
        self.assertIsNotNone(cmd)

        args = cmd.get_args()
        self.assertIn("-c:s:0", args)
        self.assertIn("srt", args)

    def test_multiple_subtitles_remove_then_convert_indexes_compact(self):
        tracks = [
            {"index": 0, "type": "video", "codec": "h264"},
            {"index": 2, "type": "audio", "codec": "aac"},
            {"index": 5, "type": "subtitle", "codec": "subrip"},
            {"index": 7, "type": "subtitle", "codec": "ass"},
        ]
        converter = self._converter(tracks=tracks)
        converter.track_actions = {
            2: {"action": TrackAction.REMOVE},
            3: {"action": TrackAction.CONVERT, "codec": "srt (SubRip)"},
        }
        cmd = converter.generate_ffmpeg_command()
        self.assertIsNotNone(cmd)

        args = cmd.get_args()
        self.assertEqual(_maps(args), ["0:0", "0:2", "0:7"])
        self.assertNotIn("-c:s:1", args)
        self.assertEqual(_flag_value(args, "-c:s:0"), "srt")

    def test_map_order_is_video_then_audio_then_subtitle(self):
        tracks = [
            {"index": 2, "type": "audio", "codec": "aac"},
            {"index": 5, "type": "subtitle", "codec": "subrip"},
            {"index": 0, "type": "video", "codec": "h264"},
        ]
        converter = self._converter(tracks=tracks)
        cmd = converter.generate_ffmpeg_command()
        self.assertIsNotNone(cmd)

        args = cmd.get_args()
        self.assertEqual(_maps(args), ["0:0", "0:2", "0:5"])

    def test_convert_video_sets_expected_args(self):
        converter = self._converter()
        converter.track_actions = {0: {"action": TrackAction.CONVERT, "codec": "libx265 (H.265/HEVC)"}}
        cmd = converter.generate_ffmpeg_command()
        self.assertIsNotNone(cmd)

        args = cmd.get_args()
        self.assertEqual(_flag_value(args, "-c:v:0"), "libx265")
        self.assertEqual(_flag_value(args, "-crf:v:0"), "28")
        self.assertEqual(_flag_value(args, "-preset:v:0"), "medium")


if __name__ == "__main__":
    unittest.main()
