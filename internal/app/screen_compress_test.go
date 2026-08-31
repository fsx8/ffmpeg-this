package app

import (
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
)

func TestCompressWizard_PresetFlowsToSpeedStep(t *testing.T) {
	m := newCompressWizard(Config{}, "/tmp/clip.mp4")

	m.Update(keyMsg("enter")) // High (CRF 18) -> speed

	if m.step != "speed" {
		t.Fatalf("expected the speed step, got %q", m.step)
	}
	if m.crfV != 18 {
		t.Fatalf("preset High must set crf 18, got %d", m.crfV)
	}
	if cur := m.speedList.SelectedItem().(simpleItem).value; cur != "medium" {
		t.Fatalf("speed cursor = %q, want medium preselected", cur)
	}
}

func TestCompressWizard_SpeedFlowsToOutputStep(t *testing.T) {
	in := "/tmp/clip.mp4"
	m := newCompressWizard(Config{}, in)

	m.Update(keyMsg("enter"))           // High (CRF 18) -> speed
	_, cmd := m.Update(keyMsg("enter")) // preselected medium -> probing

	if m.step != "probing" {
		t.Fatalf("expected the probing step, got %q", m.step)
	}
	m.Update(hdrProbeMsg{})

	if m.step != "output" {
		t.Fatalf("expected the output step, got %q", m.step)
	}
	if m.crfV != 18 || m.preset != "medium" {
		t.Fatalf("crf=%d preset=%q, want 18/medium", m.crfV, m.preset)
	}
	if got, want := m.out.Value(), ffx.CompressOutputName(in, 18); got != want {
		t.Fatalf("output prefill = %q, want %q", got, want)
	}
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("selecting a speed must not run ffmpeg yet")
	}
}

func TestCompressWizard_CustomCRFInvalidShowsError(t *testing.T) {
	m := newCompressWizard(Config{}, "/tmp/clip.mp4")
	m.step = "custom"
	m.crf.Focus()

	for _, bad := range []string{"", "abc", "60", "-1"} {
		m.crf.SetValue(bad)
		_, cmd := m.Update(keyMsg("enter"))
		if m.err == "" {
			t.Fatalf("expected a validation error for CRF %q", bad)
		}
		if m.step != "custom" {
			t.Fatalf("must stay on the custom step for CRF %q, got %q", bad, m.step)
		}
		if hasMsg[pushMsg](msgsOf(cmd)) {
			t.Fatalf("must not proceed on invalid CRF %q", bad)
		}
	}
}

func TestCompressWizard_CustomCRF30FlowsToSpeed(t *testing.T) {
	m := newCompressWizard(Config{}, "/tmp/clip.mp4")
	m.step = "custom"
	m.crf.Focus()
	m.crf.SetValue("30")

	m.Update(keyMsg("enter"))

	if m.err != "" {
		t.Fatalf("valid CRF 30 rejected: %s", m.err)
	}
	if m.step != "speed" {
		t.Fatalf("expected the speed step, got %q", m.step)
	}
	if m.crfV != 30 {
		t.Fatalf("custom crf = %d, want 30", m.crfV)
	}
}

func TestCompressWizard_HappyPathPushesExec(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	m := newCompressWizard(Config{}, in)

	for i := 0; i < 5; i++ {
		m.Update(keyMsg("down"))
	}
	m.Update(keyMsg("enter")) // Custom CRF… -> custom
	m.crf.SetValue("30")
	m.Update(keyMsg("enter")) // -> speed
	m.Update(keyMsg("enter")) // medium -> probing
	m.Update(hdrProbeMsg{})
	if got, want := m.out.Value(), ffx.CompressOutputName(in, 30); got != want {
		t.Fatalf("output prefill = %q, want %q", got, want)
	}
	_, cmd := m.Update(keyMsg("enter"))

	em := execOfPush(t, cmd)
	if got := flagValue(em.cmd.Args, "-crf"); got != "30" {
		t.Fatalf("-crf = %q, want 30", got)
	}
	if got := flagValue(em.cmd.Args, "-preset"); got != "medium" {
		t.Fatalf("-preset = %q, want medium", got)
	}
	if got := flagValue(em.cmd.Args, "-c:v"); got != "libx264" {
		t.Fatalf("-c:v = %q, want libx264", got)
	}
	if got := flagValue(em.cmd.Args, "-c:a"); got != "copy" {
		t.Fatalf("-c:a = %q, want copy", got)
	}
	if em.title != "Compressing video…" {
		t.Fatalf("exec title = %q, want %q", em.title, "Compressing video…")
	}
	if got, want := em.cmd.Args[len(em.cmd.Args)-1], filepath.Join(dir, "clip_crf30.mp4"); got != want {
		t.Fatalf("output path = %q, want %q", got, want)
	}
}

func TestCompressWizard_OverwriteNeedsSecondEnter(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	writeFile(t, dir, "clip_crf23.mp4")

	m := newCompressWizard(Config{}, in)
	m.Update(keyMsg("down")) // Medium (CRF 23)
	m.Update(keyMsg("enter"))
	m.Update(keyMsg("enter")) // medium -> probing
	m.Update(hdrProbeMsg{})

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

// While probing, only Esc/quit are meaningful; arrow keys must not move
// the hidden quality list's cursor.
func TestCompressWizard_ArrowKeysIgnoredDuringProbing(t *testing.T) {
	m := newCompressWizard(Config{}, "/tmp/clip.mp4")

	m.Update(keyMsg("down"))  // Medium (CRF 23)
	m.Update(keyMsg("enter")) // -> speed
	m.Update(keyMsg("enter")) // -> probing
	if m.step != "probing" {
		t.Fatalf("expected the probing step, got %q", m.step)
	}

	before := m.qualityList.Index()
	m.Update(keyMsg("up"))
	m.Update(keyMsg("down"))
	if got := m.qualityList.Index(); got != before {
		t.Fatalf("probing must not feed the hidden quality list: index %d -> %d", before, got)
	}
}

// ctrl+c during probing must cancel the in-flight color-format probe
// before the quit, not leave ffprobe running behind a dead UI.
func TestCompressWizard_CtrlCDuringProbingCancelsProbe(t *testing.T) {
	p := &blockingProbeProber{started: make(chan struct{}), cancelled: make(chan struct{})}
	m := newCompressWizard(Config{Prober: p}, "/tmp/clip.mp4")

	m.Update(keyMsg("enter"))           // High (CRF 18) -> speed
	_, cmd := m.Update(keyMsg("enter")) // medium -> probing
	done := make(chan struct{})
	go func() {
		defer close(done)
		msgsOf(cmd) // runs the batch, including the blocking probe
	}()
	<-p.started

	_, cmd = m.Update(keyMsg("ctrl+c"))
	if !hasMsg[tea.QuitMsg](msgsOf(cmd)) {
		t.Fatal("ctrl+c during probing must quit")
	}
	select {
	case <-p.cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("ctrl+c during probing must cancel the in-flight probe")
	}
	<-done
}

func TestCompressWizard_EscStepsBackThroughSteps(t *testing.T) {
	m := newCompressWizard(Config{}, "/tmp/clip.mp4")

	m.Update(keyMsg("enter")) // High (CRF 18) -> speed
	if m.step != "speed" {
		t.Fatalf("expected the speed step, got %q", m.step)
	}
	m.Update(keyMsg("enter")) // medium -> probing
	m.Update(hdrProbeMsg{})
	_, cmd := m.Update(keyMsg("esc"))
	if m.step != "speed" {
		t.Fatalf("esc from output must return to the speed step, got %q", m.step)
	}
	if hasMsg[popMsg](msgsOf(cmd)) {
		t.Fatal("esc from the output step must not pop the wizard")
	}
	m.Update(keyMsg("esc"))
	if m.step != "quality" {
		t.Fatalf("esc from speed must return to the quality step, got %q", m.step)
	}

	for i := 0; i < 5; i++ {
		m.Update(keyMsg("down"))
	}
	m.Update(keyMsg("enter")) // Custom CRF… -> custom
	if m.step != "custom" {
		t.Fatalf("expected the custom step, got %q", m.step)
	}
	m.Update(keyMsg("esc"))
	if m.step != "quality" {
		t.Fatalf("esc from custom must return to the quality step, got %q", m.step)
	}

	_, cmd = m.Update(keyMsg("esc"))
	if !hasMsg[popMsg](msgsOf(cmd)) {
		t.Fatal("esc on the first step should pop the wizard")
	}
}

// Esc from the output step must restore the custom-CRF context the same
// way resize/effects restore their custom fields.
func TestCompressWizard_EscFromOutputRestoresCustomCRF(t *testing.T) {
	m := newCompressWizard(Config{}, "/tmp/clip.mp4")

	for i := 0; i < 5; i++ {
		m.Update(keyMsg("down"))
	}
	m.Update(keyMsg("enter")) // Custom CRF… -> custom
	m.crf.SetValue("30")
	m.Update(keyMsg("enter")) // -> speed
	m.Update(keyMsg("enter")) // -> probing
	m.Update(hdrProbeMsg{})   // -> output
	if m.step != "output" {
		t.Fatalf("expected the output step, got %q", m.step)
	}

	m.Update(keyMsg("esc"))
	if m.step != "custom" {
		t.Fatalf("esc from output must restore the custom step, got %q", m.step)
	}
	if !m.crf.Focused() {
		t.Fatal("the CRF field must be focused again after esc")
	}

	// A preset CRF loses its meaning on the custom field: esc from output
	// returns to the speed step instead.
	m2 := newCompressWizard(Config{}, "/tmp/clip.mp4")
	m2.Update(keyMsg("enter")) // High (CRF 18) -> speed
	m2.Update(keyMsg("enter")) // -> probing
	m2.Update(hdrProbeMsg{})   // -> output
	m2.Update(keyMsg("esc"))
	if m2.step != "speed" || m2.fromCustom {
		t.Fatalf("preset esc from output must return to speed, got step=%q fromCustom=%v", m2.step, m2.fromCustom)
	}
}
