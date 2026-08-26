package ffmpeg

import (
	"testing"
)

func maps(args []string) []string {
	var out []string
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-map" {
			out = append(out, args[i+1])
		}
	}
	return out
}

func flagValue(args []string, flag string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

func TestInteractiveConvert_KeepAllTracksWithGappedIndexes(t *testing.T) {
	tracks := []Track{
		{Index: 0, Type: TrackVideo, Codec: "h264"},
		{Index: 2, Type: TrackAudio, Codec: "aac"},
		{Index: 5, Type: TrackSubtitle, Codec: "subrip"},
	}
	cmd := BuildInteractiveConvertCmd("input.mkv", "output.mkv", tracks, map[int]TrackActionInfo{})
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	if got, want := maps(cmd.Args), []string{"0:0", "0:2", "0:5"}; !equalStrings(got, want) {
		t.Fatalf("maps mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
	if got := flagValue(cmd.Args, "-c:v:0"); got != "copy" {
		t.Fatalf("expected -c:v:0 copy, got %q", got)
	}
	if got := flagValue(cmd.Args, "-c:a:0"); got != "copy" {
		t.Fatalf("expected -c:a:0 copy, got %q", got)
	}
	if got := flagValue(cmd.Args, "-c:s:0"); got != "copy" {
		t.Fatalf("expected -c:s:0 copy, got %q", got)
	}
}

func TestInteractiveConvert_AllTracksRemovedReturnsNil(t *testing.T) {
	tracks := []Track{
		{Index: 0, Type: TrackVideo, Codec: "h264"},
		{Index: 2, Type: TrackAudio, Codec: "aac"},
		{Index: 5, Type: TrackSubtitle, Codec: "subrip"},
	}
	actions := map[int]TrackActionInfo{
		0: {Action: ActionRemove},
		1: {Action: ActionRemove},
		2: {Action: ActionRemove},
	}
	if cmd := BuildInteractiveConvertCmd("input.mkv", "output.mkv", tracks, actions); cmd != nil {
		t.Fatalf("expected nil cmd, got %+v", cmd)
	}
}

func TestInteractiveConvert_RemoveVideoOnlyOutputsAudioAndSubs(t *testing.T) {
	tracks := []Track{
		{Index: 0, Type: TrackVideo, Codec: "h264"},
		{Index: 2, Type: TrackAudio, Codec: "aac"},
		{Index: 5, Type: TrackSubtitle, Codec: "subrip"},
	}
	actions := map[int]TrackActionInfo{0: {Action: ActionRemove}}
	cmd := BuildInteractiveConvertCmd("input.mkv", "output.mkv", tracks, actions)
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	if got, want := maps(cmd.Args), []string{"0:2", "0:5"}; !equalStrings(got, want) {
		t.Fatalf("maps mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
	if got := flagValue(cmd.Args, "-c:v:0"); got != "" {
		t.Fatalf("expected no -c:v:0, got %q", got)
	}
	if got := flagValue(cmd.Args, "-c:a:0"); got != "copy" {
		t.Fatalf("expected -c:a:0 copy, got %q", got)
	}
	if got := flagValue(cmd.Args, "-c:s:0"); got != "copy" {
		t.Fatalf("expected -c:s:0 copy, got %q", got)
	}
}

func TestInteractiveConvert_RemoveAudioTrackByTrackID(t *testing.T) {
	tracks := []Track{
		{Index: 0, Type: TrackVideo, Codec: "h264"},
		{Index: 2, Type: TrackAudio, Codec: "aac"},
		{Index: 5, Type: TrackSubtitle, Codec: "subrip"},
	}
	actions := map[int]TrackActionInfo{1: {Action: ActionRemove}}
	cmd := BuildInteractiveConvertCmd("input.mkv", "output.mkv", tracks, actions)
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	if got, want := maps(cmd.Args), []string{"0:0", "0:5"}; !equalStrings(got, want) {
		t.Fatalf("maps mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
	if got := flagValue(cmd.Args, "-c:a:0"); got != "" {
		t.Fatalf("expected no -c:a:0, got %q", got)
	}
}

func TestInteractiveConvert_ConvertAudioSetsCodecAndBitrate(t *testing.T) {
	tracks := []Track{
		{Index: 0, Type: TrackVideo, Codec: "h264"},
		{Index: 2, Type: TrackAudio, Codec: "aac"},
		{Index: 5, Type: TrackSubtitle, Codec: "subrip"},
	}
	actions := map[int]TrackActionInfo{1: {Action: ActionConvert, Codec: "aac"}}
	cmd := BuildInteractiveConvertCmd("input.mkv", "output.mkv", tracks, actions)
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	if got := flagValue(cmd.Args, "-c:a:0"); got != "aac" {
		t.Fatalf("expected -c:a:0 aac, got %q", got)
	}
	if got := flagValue(cmd.Args, "-b:a:0"); got != "192k" {
		t.Fatalf("expected -b:a:0 192k, got %q", got)
	}
}

func TestInteractiveConvert_ConvertAudioFromUIChoiceIsNormalized(t *testing.T) {
	tracks := []Track{
		{Index: 0, Type: TrackVideo, Codec: "h264"},
		{Index: 2, Type: TrackAudio, Codec: "aac"},
		{Index: 5, Type: TrackSubtitle, Codec: "subrip"},
	}
	actions := map[int]TrackActionInfo{1: {Action: ActionConvert, Codec: "libmp3lame (MP3)"}}
	cmd := BuildInteractiveConvertCmd("input.mkv", "output.mkv", tracks, actions)
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	if got := flagValue(cmd.Args, "-c:a:0"); got != "libmp3lame" {
		t.Fatalf("expected libmp3lame, got %q", got)
	}
	if got := flagValue(cmd.Args, "-b:a:0"); got != "192k" {
		t.Fatalf("expected 192k, got %q", got)
	}
}

func TestInteractiveConvert_MultipleAudioKeepAndConvertIndexesCompact(t *testing.T) {
	tracks := []Track{
		{Index: 0, Type: TrackVideo, Codec: "h264"},
		{Index: 1, Type: TrackAudio, Codec: "aac"},
		{Index: 4, Type: TrackAudio, Codec: "dts"},
		{Index: 6, Type: TrackSubtitle, Codec: "subrip"},
	}
	actions := map[int]TrackActionInfo{2: {Action: ActionConvert, Codec: "libopus (Opus)"}}
	cmd := BuildInteractiveConvertCmd("input.mkv", "output.mkv", tracks, actions)
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	if got, want := maps(cmd.Args), []string{"0:0", "0:1", "0:4", "0:6"}; !equalStrings(got, want) {
		t.Fatalf("maps mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
	if got := flagValue(cmd.Args, "-c:a:0"); got != "copy" {
		t.Fatalf("expected audio0 copy, got %q", got)
	}
	if got := flagValue(cmd.Args, "-c:a:1"); got != "libopus" {
		t.Fatalf("expected audio1 libopus, got %q", got)
	}
	if got := flagValue(cmd.Args, "-b:a:1"); got != "160k" {
		t.Fatalf("expected audio1 160k, got %q", got)
	}
}

func TestInteractiveConvert_ConvertSubtitleChoiceIsNormalized(t *testing.T) {
	tracks := []Track{
		{Index: 0, Type: TrackVideo, Codec: "h264"},
		{Index: 2, Type: TrackAudio, Codec: "aac"},
		{Index: 5, Type: TrackSubtitle, Codec: "subrip"},
	}
	actions := map[int]TrackActionInfo{2: {Action: ActionConvert, Codec: "srt (SubRip)"}}
	cmd := BuildInteractiveConvertCmd("input.mkv", "output.mkv", tracks, actions)
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	if got := flagValue(cmd.Args, "-c:s:0"); got != "srt" {
		t.Fatalf("expected srt, got %q", got)
	}
}

func TestInteractiveConvert_ConvertSubtitleMovTextChoiceIsNormalized(t *testing.T) {
	tracks := []Track{
		{Index: 0, Type: TrackVideo, Codec: "h264"},
		{Index: 2, Type: TrackAudio, Codec: "aac"},
		{Index: 5, Type: TrackSubtitle, Codec: "subrip"},
	}
	actions := map[int]TrackActionInfo{2: {Action: ActionConvert, Codec: "mov_text (MP4)"}}
	cmd := BuildInteractiveConvertCmd("input.mkv", "output.mkv", tracks, actions)
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	if got := flagValue(cmd.Args, "-c:s:0"); got != "mov_text" {
		t.Fatalf("expected mov_text, got %q", got)
	}
}

func TestInteractiveConvert_ConvertSubtitleLegacyChoiceIsNormalized(t *testing.T) {
	tracks := []Track{
		{Index: 0, Type: TrackVideo, Codec: "h264"},
		{Index: 2, Type: TrackAudio, Codec: "aac"},
		{Index: 5, Type: TrackSubtitle, Codec: "subrip"},
	}
	actions := map[int]TrackActionInfo{2: {Action: ActionConvert, Codec: "(SubRip)"}}
	cmd := BuildInteractiveConvertCmd("input.mkv", "output.mkv", tracks, actions)
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	if got := flagValue(cmd.Args, "-c:s:0"); got != "srt" {
		t.Fatalf("expected srt, got %q", got)
	}
}

func TestInteractiveConvert_MultipleSubtitlesRemoveThenConvertIndexesCompact(t *testing.T) {
	tracks := []Track{
		{Index: 0, Type: TrackVideo, Codec: "h264"},
		{Index: 2, Type: TrackAudio, Codec: "aac"},
		{Index: 5, Type: TrackSubtitle, Codec: "subrip"},
		{Index: 7, Type: TrackSubtitle, Codec: "ass"},
	}
	actions := map[int]TrackActionInfo{
		2: {Action: ActionRemove},
		3: {Action: ActionConvert, Codec: "srt (SubRip)"},
	}
	cmd := BuildInteractiveConvertCmd("input.mkv", "output.mkv", tracks, actions)
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	if got, want := maps(cmd.Args), []string{"0:0", "0:2", "0:7"}; !equalStrings(got, want) {
		t.Fatalf("maps mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
	if got := flagValue(cmd.Args, "-c:s:0"); got != "srt" {
		t.Fatalf("expected srt, got %q", got)
	}
	if got := flagValue(cmd.Args, "-c:s:1"); got != "" {
		t.Fatalf("expected no -c:s:1, got %q", got)
	}
}

func TestInteractiveConvert_MapOrderIsVideoThenAudioThenSubtitle(t *testing.T) {
	tracks := []Track{
		{Index: 2, Type: TrackAudio, Codec: "aac"},
		{Index: 5, Type: TrackSubtitle, Codec: "subrip"},
		{Index: 0, Type: TrackVideo, Codec: "h264"},
	}
	cmd := BuildInteractiveConvertCmd("input.mkv", "output.mkv", tracks, map[int]TrackActionInfo{})
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	if got, want := maps(cmd.Args), []string{"0:0", "0:2", "0:5"}; !equalStrings(got, want) {
		t.Fatalf("maps mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestInteractiveConvert_ConvertVideoSetsExpectedArgs(t *testing.T) {
	tracks := []Track{
		{Index: 0, Type: TrackVideo, Codec: "h264"},
		{Index: 2, Type: TrackAudio, Codec: "aac"},
		{Index: 5, Type: TrackSubtitle, Codec: "subrip"},
	}
	actions := map[int]TrackActionInfo{0: {Action: ActionConvert, Codec: "libx265 (H.265/HEVC)"}}
	cmd := BuildInteractiveConvertCmd("input.mkv", "output.mkv", tracks, actions)
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	if got := flagValue(cmd.Args, "-c:v:0"); got != "libx265" {
		t.Fatalf("expected libx265, got %q", got)
	}
	if got := flagValue(cmd.Args, "-crf:v:0"); got != "28" {
		t.Fatalf("expected crf 28, got %q", got)
	}
	if got := flagValue(cmd.Args, "-preset:v:0"); got != "medium" {
		t.Fatalf("expected preset medium, got %q", got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
