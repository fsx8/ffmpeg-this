package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/fsx8/ffwiz/internal/ffprobe"
)

func pushedOf(t *testing.T, cmd tea.Cmd) tea.Model {
	t.Helper()
	for _, msg := range msgsOf(cmd) {
		if p, ok := msg.(pushMsg); ok {
			return p.m
		}
	}
	t.Fatal("no pushMsg produced")
	return nil
}

// --- main menu ---

func TestMainMenu_EnterNavigatesPerItem(t *testing.T) {
	cases := []struct {
		name  string
		index int
		want  tea.Model // expected pushed screen; nil when the entry quits
		quit  bool
	}{
		{"single-file entry opens the file picker", 0, newFilePicker(Config{}, "."), false},
		{"join entry opens the join wizard", 1, newJoinWizard(Config{}, "."), false},
		{"batch entry opens the batch wizard", 2, newBatchWizard(Config{}, "."), false},
		{"exit entry quits", 3, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newMainMenu(Config{})
			m.list.Select(c.index)

			_, cmd := m.Update(keyMsg("enter"))
			msgs := msgsOf(cmd)
			if c.quit {
				if !hasMsg[tea.QuitMsg](msgs) {
					t.Fatalf("enter on %q must quit the app", "exit")
				}
				return
			}
			pushed := pushedOf(t, cmd)
			if reflect.TypeOf(pushed) != reflect.TypeOf(c.want) {
				t.Fatalf("pushed %T, want %T", pushed, c.want)
			}
		})
	}
}

func TestMainMenu_QQuits(t *testing.T) {
	m := newMainMenu(Config{})
	_, cmd := m.Update(keyMsg("q"))
	if !hasMsg[tea.QuitMsg](msgsOf(cmd)) {
		t.Fatal("q must quit the app from the main menu")
	}
}

func TestMainMenu_ViewListsEntries(t *testing.T) {
	m := newMainMenu(Config{})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	view := m.View()
	for _, want := range []string{
		"ffwiz — ffmpeg wizard",
		"Process a Single Media File",
		"Join Multiple Videos",
		"Batch Convert Directory",
		"Exit",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("main menu view must list %q:\n%s", want, view)
		}
	}
}

// --- action menu ---

func TestActionMenu_PushesWizardPerAction(t *testing.T) {
	const file = "movie.mkv"
	cases := []struct {
		name  string
		index int
		want  tea.Model
		pops  bool
	}{
		{"inspect opens the inspect screen", 0, newInspectScreen(Config{}, file), false},
		{"trim opens the trim wizard", 2, newTrimWizard(Config{}, file), false},
		{"extract-audio opens its wizard", 3, newExtractAudioWizard(Config{}, file), false},
		{"metadata opens its wizard", 9, newMetadataWizard(Config{}, file), false},
		{"back pops the action menu", 10, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newActionMenu(Config{}, file)
			m.list.Select(c.index)

			_, cmd := m.Update(keyMsg("enter"))
			msgs := msgsOf(cmd)
			if c.pops {
				if !hasMsg[popMsg](msgs) {
					t.Fatal("enter on back must pop the action menu")
				}
				return
			}
			pushed := pushedOf(t, cmd)
			if reflect.TypeOf(pushed) != reflect.TypeOf(c.want) {
				t.Fatalf("pushed %T, want %T", pushed, c.want)
			}
		})
	}
}

func TestActionMenu_EscPops(t *testing.T) {
	m := newActionMenu(Config{}, "movie.mkv")
	_, cmd := m.Update(keyMsg("esc"))
	if !hasMsg[popMsg](msgsOf(cmd)) {
		t.Fatal("esc must pop the action menu")
	}
}

// --- inspect screen ---

func TestInspectScreen_ShowsProbeResults(t *testing.T) {
	const file = "movie.mkv"
	res := &ffprobe.ProbeResult{
		Streams: []ffprobe.Stream{
			{Index: 0, CodecType: "video", CodecName: "h264", Width: 640, Height: 360, RFrameRate: "24/1", PixFmt: "yuv420p"},
			{Index: 1, CodecType: "audio", CodecName: "aac", SampleRate: "48000", Channels: 2, Tags: map[string]string{"language": "eng"}},
			{Index: 2, CodecType: "subtitle", CodecName: "subrip"},
		},
		Format: ffprobe.Format{Filename: file, Duration: "20", Size: "10485760", BitRate: "500000"},
	}
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{file: res}}}
	m := newInspectScreen(cfg, file)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	var probe inspectDoneMsg
	for _, msg := range msgsOf(m.Init()) {
		if p, ok := msg.(inspectDoneMsg); ok {
			probe = p
		}
	}
	if probe.err != nil {
		t.Fatalf("probe failed: %v", probe.err)
	}
	m.Update(probe)

	view := m.View()
	for _, want := range []string{
		"File Information",
		"movie.mkv",
		"10.00 MB",
		"20.00 seconds",
		"500 kb/s",
		"#0  h264",
		"640x360",
		"yuv420p",
		"#1  aac",
		"48000 Hz",
		"2ch",
		"eng",
		"#2  subrip",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("inspect view must contain %q:\n%s", want, view)
		}
	}
}

// --- start-screen routing by initial path ---

func TestRoot_NewRoutesInitialPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "clip.mp4")
	file := filepath.Join(dir, "clip.mp4")
	missing := filepath.Join(dir, "missing.mp4")

	cases := []struct {
		name string
		path string
		want tea.Model
	}{
		{"no path opens the main menu", "", newMainMenu(Config{})},
		{"a file opens the action menu", file, newActionMenu(Config{}, file)},
		{"a directory opens the join wizard", dir, newJoinWizard(Config{}, dir)},
		{"a missing path falls back to the main menu", missing, newMainMenu(Config{})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, ok := New(Config{InitialPath: c.path}).(*rootModel)
			if !ok {
				t.Fatalf("unexpected root model %T", r)
			}
			if len(r.stack) != 1 {
				t.Fatalf("expected a single start screen, got %d", len(r.stack))
			}
			if reflect.TypeOf(r.stack[0]) != reflect.TypeOf(c.want) {
				t.Fatalf("start screen is %T, want %T", r.stack[0], c.want)
			}
		})
	}
}
