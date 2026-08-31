package app

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
)

func TestScreenshotWizard_FormatPickReachesTimestamp(t *testing.T) {
	m := newScreenshotWizard(Config{}, "/tmp/clip.mp4")
	if m.step != "format" {
		t.Fatalf("expected the format step first, got %q", m.step)
	}

	m.Update(keyMsg("enter")) // png
	if m.step != "timestamp" {
		t.Fatalf("expected the timestamp step, got %q", m.step)
	}
	if m.format != "png" {
		t.Fatalf("format = %q, want png", m.format)
	}
}

func TestScreenshotWizard_InvalidTimestampShowsError(t *testing.T) {
	m := newScreenshotWizard(Config{}, "/tmp/clip.mp4")
	m.Update(keyMsg("enter")) // -> timestamp
	m.timestamp.SetValue("abc")

	_, cmd := m.Update(keyMsg("enter"))
	if m.err == "" {
		t.Fatal("expected a validation error for an invalid timestamp")
	}
	if m.step != "timestamp" {
		t.Fatalf("must stay on the timestamp step, got %q", m.step)
	}
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("must not proceed to the exec screen on an invalid timestamp")
	}
}

func TestScreenshotWizard_ValidFlowPrefillsOutput(t *testing.T) {
	in := "/tmp/clip.mp4"
	m := newScreenshotWizard(Config{}, in)
	m.Update(keyMsg("enter")) // png -> timestamp
	m.timestamp.SetValue("00:05:00")

	_, cmd := m.Update(keyMsg("enter"))
	if m.step != "probing" {
		t.Fatalf("expected the probing step, got %q", m.step)
	}
	m.Update(screenshotDurMsg{dur: 601}) // longer than the 5 minute mark
	if m.err != "" {
		t.Fatalf("valid timestamp rejected: %s", m.err)
	}
	if m.step != "output" {
		t.Fatalf("expected the output step, got %q", m.step)
	}
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("timestamp step must not push the exec screen yet")
	}
	if got, want := m.out.Value(), ffx.ScreenshotOutputName(in, "00:05:00", "png"); got != want {
		t.Fatalf("output prefill = %q, want %q", got, want)
	}
}

func TestScreenshotWizard_HappyPathPushesExec(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	m := newScreenshotWizard(Config{}, in)
	m.Update(keyMsg("enter")) // png -> timestamp
	m.timestamp.SetValue("00:00:05")
	m.Update(keyMsg("enter")) // -> probing
	m.Update(screenshotDurMsg{dur: 20})
	_, cmd := m.Update(keyMsg("enter"))

	em := execOfPush(t, cmd)
	if got := flagValue(em.cmd.Args, "-ss"); got != "00:00:05" {
		t.Fatalf("-ss = %q, want 00:00:05", got)
	}
	if got := flagValue(em.cmd.Args, "-frames:v"); got != "1" {
		t.Fatalf("-frames:v = %q, want 1", got)
	}
	if got, want := em.cmd.Args[len(em.cmd.Args)-1], filepath.Join(dir, "clip_frame_00-00-05.png"); got != want {
		t.Fatalf("output path = %q, want %q", got, want)
	}
	if em.title != "Taking screenshot…" {
		t.Fatalf("exec title = %q, want %q", em.title, "Taking screenshot…")
	}
}

func TestScreenshotWizard_OverwriteNeedsSecondEnter(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	writeFile(t, dir, "clip_frame_00-00-05.png")

	m := newScreenshotWizard(Config{}, in)
	m.Update(keyMsg("enter")) // png -> timestamp
	m.timestamp.SetValue("00:00:05")
	m.Update(keyMsg("enter")) // -> probing
	m.Update(screenshotDurMsg{dur: 20})

	_, cmd := m.Update(keyMsg("enter"))
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("first Enter must not overwrite an existing file")
	}
	if m.guard.armedFor == "" {
		t.Fatal("expected an overwrite warning")
	}

	_, cmd = m.Update(keyMsg("enter"))
	if !hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("second Enter should confirm the overwrite and run")
	}
}

func TestScreenshotWizard_EscStepsBackThroughSteps(t *testing.T) {
	m := newScreenshotWizard(Config{}, "/tmp/clip.mp4")

	m.Update(keyMsg("enter")) // -> timestamp
	m.timestamp.SetValue("1")
	m.Update(keyMsg("enter")) // -> probing
	m.Update(screenshotDurMsg{dur: 20})
	_, cmd := m.Update(keyMsg("esc"))
	if m.step != "timestamp" {
		t.Fatalf("esc from output must return to the timestamp step, got %q", m.step)
	}
	if hasMsg[popMsg](msgsOf(cmd)) {
		t.Fatal("esc from the output step must not pop the whole wizard")
	}

	m.Update(keyMsg("esc"))
	if m.step != "format" {
		t.Fatalf("esc from timestamp must return to the format step, got %q", m.step)
	}

	_, cmd = m.Update(keyMsg("esc"))
	if !hasMsg[popMsg](msgsOf(cmd)) {
		t.Fatal("esc on the format step should pop the wizard")
	}
}

// While probing, only Esc/quit are meaningful; arrow keys must not move
// the hidden format list's cursor.
func TestScreenshotWizard_ArrowKeysIgnoredDuringProbing(t *testing.T) {
	m := newScreenshotWizard(Config{}, "/tmp/clip.mp4")

	m.Update(keyMsg("enter")) // png -> timestamp
	m.timestamp.SetValue("00:00:05")
	m.Update(keyMsg("enter")) // -> probing
	if m.step != "probing" {
		t.Fatalf("expected the probing step, got %q", m.step)
	}

	before := m.formatList.Index()
	m.Update(keyMsg("up"))
	m.Update(keyMsg("down"))
	if got := m.formatList.Index(); got != before {
		t.Fatalf("probing must not feed the hidden format list: index %d -> %d", before, got)
	}
}

func TestScreenshotWizard_TypingQReachesTimestampInput(t *testing.T) {
	m := newScreenshotWizard(Config{}, "/tmp/clip.mp4")
	m.Update(keyMsg("enter")) // -> timestamp
	m.timestamp.Focus()

	model, cmd := m.Update(keyMsg("q"))
	sw, ok := model.(*screenshotWizard)
	if !ok {
		t.Fatalf("unexpected model %T", model)
	}
	if sw.timestamp.Value() != "q" {
		t.Fatalf("'q' must reach the timestamp input, got %q", sw.timestamp.Value())
	}
	if hasMsg[tea.QuitMsg](msgsOf(cmd)) {
		t.Fatal("typing 'q' must not quit the app")
	}
}
