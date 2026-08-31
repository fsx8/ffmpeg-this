package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"path/filepath"
	"strings"
	"testing"

	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
	"github.com/fsx8/ffwiz/internal/ffprobe"
)

// sameArgs compares two argument slices element-wise.
func sameArgs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func audioCheckOf(t *testing.T, cmd tea.Cmd) audioCheckDoneMsg {
	t.Helper()
	for _, msg := range msgsOf(cmd) {
		if c, ok := msg.(audioCheckDoneMsg); ok {
			return c
		}
	}
	t.Fatal("no audioCheckDoneMsg produced")
	return audioCheckDoneMsg{}
}

func TestExtractAudio_HappyPathRunsExtractCmd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "clip.mkv")
	in := filepath.Join(dir, "clip.mkv")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{in: videoWithAudio("10")}}}
	m := newExtractAudioWizard(cfg, in)

	m.Update(audioCheckOf(t, m.Init()))
	if m.loading {
		t.Fatal("the audio check must clear the loading state")
	}
	if !m.hasAudio {
		t.Fatal("expected the audio check to find audio")
	}

	m.Update(keyMsg("enter")) // format -> output
	if m.mode != "output" {
		t.Fatalf("enter on the format step must advance to the output step, got %q", m.mode)
	}
	if want := ffx.ExtractAudioOutputName(in, "mp3"); m.out.Value() != want {
		t.Fatalf("default output = %q, want %q", m.out.Value(), want)
	}

	_, cmd := m.Update(keyMsg("enter")) // output -> exec
	em := execOfPush(t, cmd)
	wantCmd := ffx.BuildExtractAudioCmd(in, "mp3", filepath.Join(dir, "clip_audio.mp3"))
	if !sameArgs(em.cmd.Args, wantCmd.Args) {
		t.Fatalf("exec args:\ngot  %#v\nwant %#v", em.cmd.Args, wantCmd.Args)
	}
	if got := em.cmd.Name; got != "ffmpeg" {
		t.Fatalf("exec command = %q, want ffmpeg", got)
	}
}

func TestExtractAudio_NoAudioSurfacesWarningAndBlocks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "silent.mp4")
	in := filepath.Join(dir, "silent.mp4")
	silent := &ffprobe.ProbeResult{
		Streams: []ffprobe.Stream{{Index: 0, CodecType: "video", CodecName: "h264"}},
	}
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{in: silent}}}
	m := newExtractAudioWizard(cfg, in)

	m.Update(audioCheckOf(t, m.Init()))
	if m.loading {
		t.Fatal("the audio check must clear the loading state")
	}
	if m.hasAudio {
		t.Fatal("a video-only file must report no audio")
	}
	if m.err == "" {
		t.Fatal("the no-audio finding must surface as a warning")
	}
	if !strings.Contains(m.View(), "no audio stream found") {
		t.Fatalf("the view must explain the missing audio:\n%s", m.View())
	}

	_, cmd := m.Update(keyMsg("enter"))
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("enter must not run an extract on a file without audio")
	}
}
