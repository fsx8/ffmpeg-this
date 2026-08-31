package app

import (
	"path/filepath"
	"strings"
	"testing"

	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
)

func TestResizeWizard_PresetFlowsToOutputStep(t *testing.T) {
	in := "/tmp/clip.mp4"
	m := newResizeWizard(Config{}, in)

	m.Update(keyMsg("down"))
	m.Update(keyMsg("down")) // 2160p -> 1080p -> 720p
	_, cmd := m.Update(keyMsg("enter"))

	if m.step != "probing" {
		t.Fatalf("expected the probing step, got %q", m.step)
	}
	m.Update(hdrProbeMsg{})

	if m.step != "output" {
		t.Fatalf("expected the output step, got %q", m.step)
	}
	if m.outWidth != -2 || m.outHeight != 720 {
		t.Fatalf("preset 720p must set width -2 height 720, got %dx%d", m.outWidth, m.outHeight)
	}
	if got, want := m.out.Value(), ffx.ResizeOutputName(in, "720p"); got != want {
		t.Fatalf("output prefill = %q, want %q", got, want)
	}
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("selecting a preset must not run ffmpeg yet")
	}
}

func TestResizeWizard_CustomWithBothFieldsEmptyShowsError(t *testing.T) {
	m := newResizeWizard(Config{}, "/tmp/clip.mp4")
	m.step = "custom"
	m.width.Focus()

	_, cmd := m.Update(keyMsg("enter"))
	if m.err == "" {
		t.Fatal("expected a validation error when both dimensions are empty")
	}
	if m.step != "custom" {
		t.Fatalf("must stay on the custom step, got %q", m.step)
	}
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("must not proceed without a dimension")
	}

	m.width.SetValue("abc")
	m.Update(keyMsg("enter"))
	if m.err == "" {
		t.Fatal("expected a validation error for a non-numeric width")
	}
	if m.step != "custom" {
		t.Fatalf("must stay on the custom step after invalid width, got %q", m.step)
	}
}

func TestResizeWizard_CustomWidthOnlyKeepsAspect(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	m := newResizeWizard(Config{}, in)
	m.step = "custom"
	m.width.Focus()
	m.width.SetValue("1920")

	m.Update(keyMsg("enter"))
	m.Update(hdrProbeMsg{})
	if m.step != "output" {
		t.Fatalf("expected the output step, got %q", m.step)
	}
	if m.outWidth != 1920 || m.outHeight != -2 {
		t.Fatalf("custom width 1920 must keep height auto, got %dx%d", m.outWidth, m.outHeight)
	}
	if !strings.HasSuffix(m.out.Value(), "_resized.mp4") {
		t.Fatalf("custom output prefill = %q, want a _resized name", m.out.Value())
	}

	_, cmd := m.Update(keyMsg("enter"))
	em := execOfPush(t, cmd)
	if got := flagValue(em.cmd.Args, "-vf"); got != "scale=w=1920:h=-2" {
		t.Fatalf("-vf = %q, want scale=w=1920:h=-2", got)
	}
}

func TestResizeWizard_HappyPathPushesExec(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	m := newResizeWizard(Config{}, in)

	m.Update(keyMsg("down"))
	m.Update(keyMsg("down"))  // -> 720p
	m.Update(keyMsg("enter")) // -> probing
	m.Update(hdrProbeMsg{})
	_, cmd := m.Update(keyMsg("enter"))

	em := execOfPush(t, cmd)
	if got := flagValue(em.cmd.Args, "-vf"); got != "scale=w=-2:h=720" {
		t.Fatalf("-vf = %q, want scale=w=-2:h=720", got)
	}
	if got := flagValue(em.cmd.Args, "-c:a"); got != "copy" {
		t.Fatalf("-c:a = %q, want copy", got)
	}
	if em.title != "Resizing video…" {
		t.Fatalf("exec title = %q, want %q", em.title, "Resizing video…")
	}
	if got, want := em.cmd.Args[len(em.cmd.Args)-1], filepath.Join(dir, "clip_720p.mp4"); got != want {
		t.Fatalf("output path = %q, want %q", got, want)
	}
}

// While probing, only Esc/quit are meaningful; arrow keys must not move
// the hidden preset list's cursor.
func TestResizeWizard_ArrowKeysIgnoredDuringProbing(t *testing.T) {
	m := newResizeWizard(Config{}, "/tmp/clip.mp4")

	m.Update(keyMsg("enter")) // 2160p -> probing
	if m.step != "probing" {
		t.Fatalf("expected the probing step, got %q", m.step)
	}

	before := m.presetList.Index()
	m.Update(keyMsg("up"))
	m.Update(keyMsg("down"))
	if got := m.presetList.Index(); got != before {
		t.Fatalf("probing must not feed the hidden preset list: index %d -> %d", before, got)
	}
}

func TestResizeWizard_EscStepsBackThroughSteps(t *testing.T) {
	m := newResizeWizard(Config{}, "/tmp/clip.mp4")

	_, cmd := m.Update(keyMsg("enter")) // 2160p -> probing
	m.Update(hdrProbeMsg{})
	if m.step != "output" {
		t.Fatalf("expected the output step, got %q", m.step)
	}
	m.Update(keyMsg("esc"))
	if m.step != "preset" {
		t.Fatalf("esc from output must return to the preset step, got %q", m.step)
	}
	if hasMsg[popMsg](msgsOf(cmd)) {
		t.Fatal("esc from the output step must not pop the wizard")
	}

	m.Update(keyMsg("down"))
	m.Update(keyMsg("down"))
	m.Update(keyMsg("down"))
	m.Update(keyMsg("down")) // -> Custom…
	m.Update(keyMsg("enter"))
	if m.step != "custom" {
		t.Fatalf("expected the custom step, got %q", m.step)
	}
	m.Update(keyMsg("esc"))
	if m.step != "preset" {
		t.Fatalf("esc from custom must return to the preset step, got %q", m.step)
	}

	_, cmd = m.Update(keyMsg("esc"))
	if !hasMsg[popMsg](msgsOf(cmd)) {
		t.Fatal("esc on the first step should pop the wizard")
	}
}
