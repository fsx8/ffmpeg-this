package app

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
	"github.com/fsx8/ffwiz/internal/ffprobe"
)

func TestTransformWizard_RotateReachesOutputStep(t *testing.T) {
	in := "/tmp/clip.mp4"
	m := newTransformWizard(Config{}, in)

	_, cmd := m.Update(keyMsg("enter")) // first entry: Rotate 90° -> probing

	if m.step != "probing" {
		t.Fatalf("expected the probing step, got %q", m.step)
	}
	m.Update(transformProbeMsg{width: 640, height: 360})

	if m.step != "output" {
		t.Fatalf("expected the output step, got %q", m.step)
	}
	if m.mode != "rotate90" {
		t.Fatalf("mode = %q, want rotate90", m.mode)
	}
	if got, want := m.out.Value(), ffx.RotateOutputName(in, 90); got != want {
		t.Fatalf("output prefill = %q, want %q", got, want)
	}
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("selecting a mode must not run ffmpeg yet")
	}
}

func TestTransformWizard_FlipReachesOutputStep(t *testing.T) {
	in := "/tmp/clip.mp4"
	m := newTransformWizard(Config{}, in)

	for i := 0; i < 3; i++ {
		m.Update(keyMsg("down"))
	}
	m.Update(keyMsg("enter")) // Flip Horizontal -> probing
	m.Update(transformProbeMsg{width: 640, height: 360})

	if m.step != "output" || m.mode != "fliph" {
		t.Fatalf("expected the output step for fliph, got step=%q mode=%q", m.step, m.mode)
	}
	if got, want := m.out.Value(), ffx.FlipOutputName(in, "h"); got != want {
		t.Fatalf("output prefill = %q, want %q", got, want)
	}
}

func TestTransformWizard_CropStepProbesAndPrefills(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: videoWithAudio("10"),
	}}}
	m := newTransformWizard(cfg, in)

	for i := 0; i < 5; i++ {
		m.Update(keyMsg("down"))
	}
	_, cmd := m.Update(keyMsg("enter")) // Crop…

	if m.step != "probing" {
		t.Fatalf("expected the probing step, got %q", m.step)
	}
	if m.width.Value() != "" {
		t.Fatalf("fields must stay empty while probing, got width %q", m.width.Value())
	}

	var probe transformProbeMsg
	found := false
	for _, msg := range msgsOf(cmd) {
		if p, ok := msg.(transformProbeMsg); ok {
			probe = p
			found = true
		}
	}
	if !found {
		t.Fatal("expected a probe command on entering the crop step")
	}
	if probe.err != nil {
		t.Fatalf("probe failed: %v", probe.err)
	}

	m.Update(probe)
	if m.probing {
		t.Fatal("probing must end once the result arrives")
	}
	if got, want := m.width.Value(), "320"; got != want {
		t.Fatalf("width = %q, want %q (half of 640)", got, want)
	}
	if got, want := m.height.Value(), "180"; got != want {
		t.Fatalf("height = %q, want %q (half of 360)", got, want)
	}
	if got, want := m.x.Value(), "160"; got != want {
		t.Fatalf("x = %q, want %q (centered)", got, want)
	}
	if got, want := m.y.Value(), "90"; got != want {
		t.Fatalf("y = %q, want %q (centered)", got, want)
	}
}

func TestTransformWizard_CropProbeFailureLeavesFieldsEmpty(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	cfg := Config{Prober: &fakeProber{err: map[string]error{in: errors.New("ffprobe failed")}}}
	m := newTransformWizard(cfg, in)
	m.mode = "crop"
	m.step = "probing"
	m.probing = true

	m.Update(transformProbeMsg{err: errors.New("ffprobe failed")})

	if m.probing {
		t.Fatal("probing must end after a failure")
	}
	if m.step != "crop" {
		t.Fatalf("a failed crop probe must land on the crop step, got %q", m.step)
	}
	if m.width.Value() != "" || m.height.Value() != "" || m.x.Value() != "" || m.y.Value() != "" {
		t.Fatalf("a failed probe must leave the fields empty, got %q/%q/%q/%q",
			m.width.Value(), m.height.Value(), m.x.Value(), m.y.Value())
	}

	m.Update(keyMsg("enter"))
	if m.err == "" {
		t.Fatal("empty fields must fail validation after a probe failure")
	}
}

func TestTransformWizard_CropValidationRejectsEmptyAndNegative(t *testing.T) {
	m := newTransformWizard(Config{}, "/tmp/clip.mp4")
	m.step = "crop"
	m.focus = 0
	m.width.Focus()

	_, cmd := m.Update(keyMsg("enter"))
	if m.err == "" {
		t.Fatal("expected a validation error for empty fields")
	}
	if m.step != "crop" {
		t.Fatalf("must stay on the crop step, got %q", m.step)
	}
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("must not proceed on invalid crop values")
	}

	m.width.SetValue("320")
	m.height.SetValue("240")
	m.x.SetValue("-1")
	m.y.SetValue("10")
	m.Update(keyMsg("enter"))
	if m.err == "" {
		t.Fatal("expected a validation error for a negative x")
	}

	m.x.SetValue("160")
	m.width.SetValue("abc")
	m.Update(keyMsg("enter"))
	if m.err == "" {
		t.Fatal("expected a validation error for a non-numeric width")
	}
}

func TestTransformWizard_RotateHappyPathPushesExec(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	m := newTransformWizard(Config{}, in)

	m.Update(keyMsg("enter")) // Rotate 90° -> probing
	m.Update(transformProbeMsg{width: 640, height: 360})
	_, cmd := m.Update(keyMsg("enter"))

	em := execOfPush(t, cmd)
	if got := flagValue(em.cmd.Args, "-vf"); got != "transpose=1" {
		t.Fatalf("-vf = %q, want transpose=1", got)
	}
	if em.title != "Rotating video…" {
		t.Fatalf("exec title = %q, want %q", em.title, "Rotating video…")
	}
	if got, want := em.cmd.Args[len(em.cmd.Args)-1], filepath.Join(dir, "clip_rot90.mp4"); got != want {
		t.Fatalf("output path = %q, want %q", got, want)
	}
}

func TestTransformWizard_FlipHappyPathPushesExec(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	m := newTransformWizard(Config{}, in)

	for i := 0; i < 4; i++ {
		m.Update(keyMsg("down"))
	}
	m.Update(keyMsg("enter")) // Flip Vertical -> probing
	m.Update(transformProbeMsg{width: 640, height: 360})
	_, cmd := m.Update(keyMsg("enter"))

	em := execOfPush(t, cmd)
	if got := flagValue(em.cmd.Args, "-vf"); got != "vflip" {
		t.Fatalf("-vf = %q, want vflip", got)
	}
	if em.title != "Flipping video…" {
		t.Fatalf("exec title = %q, want %q", em.title, "Flipping video…")
	}
	if got, want := em.cmd.Args[len(em.cmd.Args)-1], filepath.Join(dir, "clip_flipv.mp4"); got != want {
		t.Fatalf("output path = %q, want %q", got, want)
	}
}

func TestTransformWizard_CropHappyPathPushesExec(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: videoWithAudio("10"),
	}}}
	m := newTransformWizard(cfg, in)

	for i := 0; i < 5; i++ {
		m.Update(keyMsg("down"))
	}
	m.Update(keyMsg("enter")) // Crop… -> probing
	m.Update(transformProbeMsg{width: 640, height: 360})
	if m.step != "crop" || m.probing {
		t.Fatalf("expected the prefilled crop step, got step=%q probing=%v", m.step, m.probing)
	}
	m.Update(keyMsg("enter")) // -> output
	if m.step != "output" {
		t.Fatalf("expected the output step, got %q", m.step)
	}
	if got, want := m.out.Value(), ffx.CropOutputName(in); got != want {
		t.Fatalf("output prefill = %q, want %q", got, want)
	}
	_, cmd := m.Update(keyMsg("enter"))

	em := execOfPush(t, cmd)
	if got := flagValue(em.cmd.Args, "-vf"); got != "crop=w=320:h=180:x=160:y=90" {
		t.Fatalf("-vf = %q, want crop=w=320:h=180:x=160:y=90", got)
	}
	if em.title != "Cropping video…" {
		t.Fatalf("exec title = %q, want %q", em.title, "Cropping video…")
	}
}

func TestTransformWizard_OverwriteNeedsSecondEnter(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	writeFile(t, dir, "clip_rot90.mp4")

	m := newTransformWizard(Config{}, in)
	m.Update(keyMsg("enter")) // Rotate 90° -> probing
	m.Update(transformProbeMsg{width: 640, height: 360})

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

func TestTransformWizard_EscStepsBackThroughSteps(t *testing.T) {
	m := newTransformWizard(Config{}, "/tmp/clip.mp4")

	m.Update(keyMsg("enter")) // Rotate 90° -> probing
	m.Update(transformProbeMsg{width: 640, height: 360})
	m.Update(keyMsg("esc"))
	if m.step != "mode" {
		t.Fatalf("esc from output must return to the mode step, got %q", m.step)
	}
	_, cmd := m.Update(keyMsg("esc"))
	if !hasMsg[popMsg](msgsOf(cmd)) {
		t.Fatal("esc on the first step should pop the wizard")
	}

	for i := 0; i < 5; i++ {
		m.Update(keyMsg("down"))
	}
	m.Update(keyMsg("enter")) // Crop… -> probing
	if m.step != "probing" {
		t.Fatalf("expected the probing step, got %q", m.step)
	}
	m.Update(transformProbeMsg{width: 640, height: 360})
	if m.step != "crop" {
		t.Fatalf("expected the crop step, got %q", m.step)
	}
	m.Update(transformProbeMsg{width: 640, height: 360})
	m.Update(keyMsg("enter")) // -> output
	m.Update(keyMsg("esc"))
	if m.step != "crop" {
		t.Fatalf("esc from output must return to the crop step, got %q", m.step)
	}
	m.Update(keyMsg("esc"))
	if m.step != "mode" {
		t.Fatalf("esc from crop must return to the mode step, got %q", m.step)
	}
}

// Esc from probing must clear the probing flag, matching the effects and
// metadata wizards, so stale probe state cannot linger on the model.
func TestTransformWizard_EscDuringProbingClearsProbingFlag(t *testing.T) {
	m := newTransformWizard(Config{}, "/tmp/clip.mp4")

	m.Update(keyMsg("enter")) // Rotate 90° -> probing
	if m.step != "probing" || !m.probing {
		t.Fatalf("expected the probing step, got step=%q probing=%v", m.step, m.probing)
	}

	m.Update(keyMsg("esc"))
	if m.step != "mode" || m.probing {
		t.Fatalf("esc during probing must clear the flag, got step=%q probing=%v", m.step, m.probing)
	}
	m.Update(transformProbeMsg{width: 640, height: 360})
	if m.step != "mode" {
		t.Fatalf("a stale probe result after esc must be dropped, got step %q", m.step)
	}
}

// Esc during probing must abort the in-flight dimensions/color probe
// instead of letting it run to completion behind the backed-out step.
func TestTransformWizard_EscDuringProbingCancelsProbe(t *testing.T) {
	p := &blockingProbeProber{started: make(chan struct{}), cancelled: make(chan struct{})}
	m := newTransformWizard(Config{Prober: p}, "/tmp/clip.mp4")

	_, cmd := m.Update(keyMsg("enter")) // Rotate 90° -> probing
	done := make(chan struct{})
	go func() {
		defer close(done)
		msgsOf(cmd) // runs the batch, including the blocking probe
	}()
	<-p.started

	m.Update(keyMsg("esc"))
	select {
	case <-p.cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("esc during probing must cancel the in-flight probe")
	}
	<-done
}

func TestTransformWizard_TypingQReachesOutputInput(t *testing.T) {
	m := newTransformWizard(Config{}, "/tmp/clip.mp4")
	m.step = "output"
	m.mode = "rotate90"
	m.out.Focus()

	model, cmd := m.Update(keyMsg("q"))
	tw, ok := model.(*transformWizard)
	if !ok {
		t.Fatalf("unexpected model %T", model)
	}
	if tw.out.Value() != "q" {
		t.Fatalf("'q' must reach the output input, got %q", tw.out.Value())
	}
	if hasMsg[tea.QuitMsg](msgsOf(cmd)) {
		t.Fatal("typing 'q' must not quit the app")
	}
}

// Re-encoding modes must surface the HDR/10-bit warning from the probe.
func TestTransformWizard_RotateShowsHDRWarning(t *testing.T) {
	m := newTransformWizard(Config{}, "/tmp/clip.mp4")

	m.Update(keyMsg("enter")) // Rotate 90° -> probing
	m.Update(transformProbeMsg{width: 3840, height: 2160, stream: &ffprobe.Stream{
		CodecType:     "video",
		PixFmt:        "yuv420p10le",
		ColorTransfer: "smpte2084",
	}})

	if m.step != "output" {
		t.Fatalf("expected the output step, got %q", m.step)
	}
	if m.hdr.note == "" || !strings.Contains(m.hdr.note, "HDR10") {
		t.Fatalf("expected an HDR10 warning, got %q", m.hdr.note)
	}
}

func TestTransformWizard_NoHDRWarningForSDR(t *testing.T) {
	m := newTransformWizard(Config{}, "/tmp/clip.mp4")

	m.Update(keyMsg("enter"))
	m.Update(transformProbeMsg{width: 640, height: 360, stream: &ffprobe.Stream{
		CodecType: "video",
		PixFmt:    "yuv420p",
	}})

	if m.hdr.note != "" {
		t.Fatalf("plain 8-bit SDR must not warn, got %q", m.hdr.note)
	}
}

// An oversized crop window is rejected up front instead of failing inside
// ffmpeg; offsets running past the frame are clamped back into it.
func TestTransformWizard_CropValidatesAgainstSourceSize(t *testing.T) {
	m := newTransformWizard(Config{}, "/tmp/clip.mp4")
	m.mode = "crop"
	m.step = "probing"
	m.probing = true
	m.Update(transformProbeMsg{width: 640, height: 360})

	m.width.SetValue("800")
	m.height.SetValue("180")
	m.x.SetValue("0")
	m.y.SetValue("0")
	m.Update(keyMsg("enter"))
	if m.err == "" || m.step != "crop" {
		t.Fatalf("a crop wider than the source must be rejected, err=%q step=%q", m.err, m.step)
	}

	m.width.SetValue("400")
	m.height.SetValue("300")
	m.x.SetValue("500") // 500+400 > 640 -> clamped to 240
	m.y.SetValue("100") // 100+300 > 360 -> clamped to 60
	m.Update(keyMsg("enter"))
	if m.err != "" {
		t.Fatalf("clamped crop must be accepted, got %q", m.err)
	}
	if m.step != "output" {
		t.Fatalf("expected the output step, got %q", m.step)
	}
	if m.cropX != 240 || m.cropY != 60 {
		t.Fatalf("crop offset = %d,%d, want clamped 240,60", m.cropX, m.cropY)
	}
	if m.x.Value() != "240" || m.y.Value() != "60" {
		t.Fatalf("fields must show the clamped values, got %q/%q", m.x.Value(), m.y.Value())
	}
}

// Cropping from the top-left origin is the most common anchor and must be
// accepted end to end.
func TestTransformWizard_CropAtOriginRuns(t *testing.T) {
	m := newTransformWizard(Config{}, "/tmp/clip.mp4")
	m.mode = "crop"
	m.step = "crop"
	m.focus = 0
	m.width.SetValue("320")
	m.height.SetValue("180")
	m.x.SetValue("0")
	m.y.SetValue("0")

	m.Update(keyMsg("enter"))
	if m.err != "" || m.step != "output" {
		t.Fatalf("crop at origin must proceed, err=%q step=%q", m.err, m.step)
	}
	_, cmd := m.Update(keyMsg("enter"))

	em := execOfPush(t, cmd)
	if got := flagValue(em.cmd.Args, "-vf"); got != "crop=w=320:h=180:x=0:y=0" {
		t.Fatalf("-vf = %q, want crop=w=320:h=180:x=0:y=0", got)
	}
}
