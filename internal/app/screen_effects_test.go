package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
	"github.com/fsx8/ffwiz/internal/ffprobe"
)

func effectsProbeOf(t *testing.T, cmd tea.Cmd) effectsAudioMsg {
	t.Helper()
	for _, msg := range msgsOf(cmd) {
		if p, ok := msg.(effectsAudioMsg); ok {
			return p
		}
	}
	t.Fatal("no effectsAudioMsg produced")
	return effectsAudioMsg{}
}

// multiAudioResult builds a probe result with one video and n audio tracks.
func multiAudioResult(n int) *ffprobe.ProbeResult {
	res := videoWithAudio("10")
	for i := 1; i < n; i++ {
		res.Streams = append(res.Streams, ffprobe.Stream{
			Index: i + 1, CodecType: "audio", CodecName: "aac", SampleRate: "48000",
		})
	}
	return res
}

func selectEffectsFactor(t *testing.T, m *effectsWizard, index int) tea.Cmd {
	t.Helper()
	for i := 0; i < index; i++ {
		m.Update(keyMsg("down"))
	}
	_, cmd := m.Update(keyMsg("enter"))
	return cmd
}

func TestEffectsWizard_SpeedPresetProbesThenReachesOutput(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: videoWithAudio("10"),
	}}}
	m := newEffectsWizard(cfg, in)

	m.Update(keyMsg("enter")) // Change Speed… -> factor list
	if m.step != "factor" {
		t.Fatalf("expected the factor step, got %q", m.step)
	}

	cmd := selectEffectsFactor(t, m, 5) // 2x -> probing
	if m.step != "probing" || !m.probing {
		t.Fatalf("expected the probing step, got step=%q probing=%v", m.step, m.probing)
	}
	probe := effectsProbeOf(t, cmd)
	if probe.err != nil {
		t.Fatalf("probe failed: %v", probe.err)
	}
	if !probe.hasAudio {
		t.Fatal("fixture is expected to have audio")
	}

	m.Update(probe)
	if m.step != "output" {
		t.Fatalf("expected the output step after the probe, got %q", m.step)
	}
	if got, want := m.out.Value(), ffx.SpeedOutputName(in, 2); got != want {
		t.Fatalf("output prefill = %q, want %q", got, want)
	}
}

func TestEffectsWizard_CustomFactorValidation(t *testing.T) {
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		"/tmp/clip.mp4": videoWithAudio("10"),
	}}}
	m := newEffectsWizard(cfg, "/tmp/clip.mp4")

	m.Update(keyMsg("enter")) // -> factor
	selectEffectsFactor(t, m, 6)
	if m.step != "custom" {
		t.Fatalf("expected the custom step, got %q", m.step)
	}

	_, cmd := m.Update(keyMsg("enter"))
	if m.err == "" {
		t.Fatal("expected an error for an empty factor")
	}
	if m.step != "custom" {
		t.Fatalf("must stay on the custom step, got %q", m.step)
	}
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("must not proceed on an invalid factor")
	}

	m.factorInput.SetValue("5")
	m.Update(keyMsg("enter"))
	if m.err == "" {
		t.Fatal("expected an error for a factor above 4.0")
	}

	m.factorInput.SetValue("0.1")
	m.Update(keyMsg("enter"))
	if m.err == "" {
		t.Fatal("expected an error for a factor below 0.25")
	}

	m.factorInput.SetValue("abc")
	m.Update(keyMsg("enter"))
	if m.err == "" {
		t.Fatal("expected an error for a non-numeric factor")
	}

	m.factorInput.SetValue("1.25")
	_, cmd = m.Update(keyMsg("enter"))
	if m.step != "probing" {
		t.Fatalf("a valid factor must start the audio probe, got %q", m.step)
	}
	probe := effectsProbeOf(t, cmd)
	m.Update(probe)
	if got, want := m.out.Value(), ffx.SpeedOutputName("/tmp/clip.mp4", 1.25); got != want {
		t.Fatalf("output prefill = %q, want %q", got, want)
	}
}

func TestEffectsWizard_ReverseProbesThenReachesOutput(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: videoWithAudio("10"),
	}}}
	m := newEffectsWizard(cfg, in)

	m.Update(keyMsg("down"))
	_, cmd := m.Update(keyMsg("enter")) // Reverse Video -> probing
	if m.step != "probing" || !m.probing {
		t.Fatalf("expected the probing step, got step=%q probing=%v", m.step, m.probing)
	}
	probe := effectsProbeOf(t, cmd)
	if probe.err != nil {
		t.Fatalf("probe failed: %v", probe.err)
	}

	m.Update(probe)
	if m.step != "output" {
		t.Fatalf("expected the output step, got %q", m.step)
	}
	if got, want := m.out.Value(), ffx.ReverseOutputName(in); got != want {
		t.Fatalf("output prefill = %q, want %q", got, want)
	}
}

func TestEffectsWizard_MuteSkipsProbe(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	m := newEffectsWizard(Config{}, in)

	for i := 0; i < 2; i++ {
		m.Update(keyMsg("down"))
	}
	_, cmd := m.Update(keyMsg("enter")) // Mute Audio -> output
	if m.step != "output" {
		t.Fatalf("expected the output step without probing, got %q", m.step)
	}
	if hasMsg[effectsAudioMsg](msgsOf(cmd)) {
		t.Fatal("mute must not probe for audio")
	}
	if got, want := m.out.Value(), ffx.MuteOutputName(in); got != want {
		t.Fatalf("output prefill = %q, want %q", got, want)
	}
}

func TestEffectsWizard_SpeedHappyPathPushesExec(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: videoWithAudio("10"),
	}}}
	m := newEffectsWizard(cfg, in)

	m.Update(keyMsg("enter"))           // -> factor
	cmd := selectEffectsFactor(t, m, 5) // 2x -> probing
	m.Update(effectsProbeOf(t, cmd))    // -> output
	_, cmd = m.Update(keyMsg("enter"))

	em := execOfPush(t, cmd)
	if got, want := flagValue(em.cmd.Args, "-filter_complex"), "[0:v]setpts=PTS/2[v];[0:a:0]atempo=2.0[a0]"; got != want {
		t.Fatalf("filter_complex = %q, want %q", got, want)
	}
	if got := flagValue(em.cmd.Args, "-c:a"); got != "aac" {
		t.Fatalf("-c:a = %q, want aac", got)
	}
	if em.title != "Changing speed…" {
		t.Fatalf("exec title = %q, want %q", em.title, "Changing speed…")
	}
	if got, want := em.cmd.Args[len(em.cmd.Args)-1], filepath.Join(dir, "clip_2x.mp4"); got != want {
		t.Fatalf("output path = %q, want %q", got, want)
	}
}

// The probed audio track count must reach the command generator so every
// track of a multi-audio file is filtered and mapped.
func TestEffectsWizard_MultiAudioCountReachesCommand(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mkv")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: multiAudioResult(3),
	}}}
	m := newEffectsWizard(cfg, in)

	m.Update(keyMsg("enter"))           // -> factor
	cmd := selectEffectsFactor(t, m, 5) // 2x -> probing
	probe := effectsProbeOf(t, cmd)
	if probe.audioStreams != 3 {
		t.Fatalf("probe must carry the audio track count, got %d", probe.audioStreams)
	}
	m.Update(probe) // -> output
	_, cmd = m.Update(keyMsg("enter"))

	em := execOfPush(t, cmd)
	if got, want := flagValue(em.cmd.Args, "-filter_complex"),
		"[0:v]setpts=PTS/2[v];[0:a:0]atempo=2.0[a0];[0:a:1]atempo=2.0[a1];[0:a:2]atempo=2.0[a2]"; got != want {
		t.Fatalf("filter_complex = %q, want %q", got, want)
	}
	for _, label := range []string{"[a0]", "[a1]", "[a2]"} {
		found := false
		for i, a := range em.cmd.Args {
			if a == "-map" && em.cmd.Args[i+1] == label {
				found = true
			}
		}
		if !found {
			t.Fatalf("every audio track must be mapped, missing %s: %#v", label, em.cmd.Args)
		}
	}
}

func TestEffectsWizard_ReverseHappyPathPushesExec(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: videoWithAudio("10"),
	}}}
	m := newEffectsWizard(cfg, in)

	m.Update(keyMsg("down"))
	_, cmd := m.Update(keyMsg("enter")) // Reverse -> probing
	m.Update(effectsProbeOf(t, cmd))    // -> output
	_, cmd = m.Update(keyMsg("enter"))

	em := execOfPush(t, cmd)
	if got, want := flagValue(em.cmd.Args, "-filter_complex"), "[0:v]reverse[v];[0:a:0]areverse[a0]"; got != want {
		t.Fatalf("filter_complex = %q, want %q", got, want)
	}
	if em.title != "Reversing video…" {
		t.Fatalf("exec title = %q, want %q", em.title, "Reversing video…")
	}
	if got, want := em.cmd.Args[len(em.cmd.Args)-1], filepath.Join(dir, "clip_reversed.mp4"); got != want {
		t.Fatalf("output path = %q, want %q", got, want)
	}
}

func TestEffectsWizard_MuteHappyPathPushesExec(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	m := newEffectsWizard(Config{}, in)

	for i := 0; i < 2; i++ {
		m.Update(keyMsg("down"))
	}
	m.Update(keyMsg("enter")) // Mute -> output
	_, cmd := m.Update(keyMsg("enter"))

	em := execOfPush(t, cmd)
	foundAn := false
	for _, a := range em.cmd.Args {
		if a == "-an" {
			foundAn = true
		}
		if a == "libx264" {
			t.Fatalf("mute must stay lossless: %#v", em.cmd.Args)
		}
	}
	if !foundAn {
		t.Fatalf("expected -an, got %#v", em.cmd.Args)
	}
	if em.title != "Muting audio…" {
		t.Fatalf("exec title = %q, want %q", em.title, "Muting audio…")
	}
}

func TestEffectsWizard_OverwriteNeedsSecondEnter(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	writeFile(t, dir, "clip_2x.mp4")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: videoWithAudio("10"),
	}}}
	m := newEffectsWizard(cfg, in)

	m.Update(keyMsg("enter"))           // -> factor
	cmd := selectEffectsFactor(t, m, 5) // 2x -> probing
	m.Update(effectsProbeOf(t, cmd))    // -> output

	_, cmd = m.Update(keyMsg("enter"))
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

func TestEffectsWizard_EscStepsBackThroughSteps(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: videoWithAudio("10"),
	}}}
	m := newEffectsWizard(cfg, in)

	m.Update(keyMsg("enter"))           // -> factor
	cmd := selectEffectsFactor(t, m, 0) // 0.25x -> probing
	m.Update(effectsProbeOf(t, cmd))    // -> output
	m.Update(keyMsg("esc"))
	if m.step != "factor" {
		t.Fatalf("esc from output must return to the factor step, got %q", m.step)
	}
	m.Update(keyMsg("esc"))
	if m.step != "op" {
		t.Fatalf("esc from factor must return to the op step, got %q", m.step)
	}
	_, cmd = m.Update(keyMsg("esc"))
	if !hasMsg[popMsg](msgsOf(cmd)) {
		t.Fatal("esc on the first step should pop the wizard")
	}

	m.Update(keyMsg("down"))
	m.Update(keyMsg("enter")) // Reverse -> probing
	m.Update(keyMsg("esc"))
	if m.step != "op" || m.probing {
		t.Fatalf("esc during probing must return to the op step, got step=%q probing=%v", m.step, m.probing)
	}

	m.Update(keyMsg("up"))
	m.Update(keyMsg("enter")) // Change Speed… -> factor
	for i := 0; i < 6; i++ {
		m.Update(keyMsg("down"))
	}
	m.Update(keyMsg("enter")) // Custom… -> custom
	if m.step != "custom" {
		t.Fatalf("expected the custom step, got %q", m.step)
	}
	m.factorInput.SetValue("0.75")
	_, cmd = m.Update(keyMsg("enter")) // -> probing
	m.Update(effectsProbeOf(t, cmd))   // -> output
	m.Update(keyMsg("esc"))
	if m.step != "custom" {
		t.Fatalf("esc from output must return to the custom step, got %q", m.step)
	}
	m.Update(keyMsg("esc"))
	if m.step != "factor" {
		t.Fatalf("esc from custom must return to the factor step, got %q", m.step)
	}
}

// Esc during probing must abort the in-flight audio/color probe instead
// of letting it run to completion behind the backed-out step.
func TestEffectsWizard_EscDuringProbingCancelsProbe(t *testing.T) {
	p := &blockingProbeProber{started: make(chan struct{}), cancelled: make(chan struct{})}
	m := newEffectsWizard(Config{Prober: p}, "/tmp/clip.mp4")

	m.Update(keyMsg("enter"))           // Change Speed… -> factor
	_, cmd := m.Update(keyMsg("enter")) // 0.25x -> probing
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

func TestEffectsWizard_TypingQReachesInputs(t *testing.T) {
	m := newEffectsWizard(Config{}, "/tmp/clip.mp4")
	m.step = "output"
	m.op = "mute"
	m.out.Focus()

	model, cmd := m.Update(keyMsg("q"))
	ew, ok := model.(*effectsWizard)
	if !ok {
		t.Fatalf("unexpected model %T", model)
	}
	if ew.out.Value() != "q" {
		t.Fatalf("'q' must reach the output input, got %q", ew.out.Value())
	}
	if hasMsg[tea.QuitMsg](msgsOf(cmd)) {
		t.Fatal("typing 'q' must not quit the app")
	}

	m.step = "custom"
	m.out.Blur()
	m.factorInput.Focus()
	model, cmd = m.Update(keyMsg("q"))
	ew, ok = model.(*effectsWizard)
	if !ok {
		t.Fatalf("unexpected model %T", model)
	}
	if ew.factorInput.Value() != "q" {
		t.Fatalf("'q' must reach the factor input, got %q", ew.factorInput.Value())
	}
	if hasMsg[tea.QuitMsg](msgsOf(cmd)) {
		t.Fatal("typing 'q' must not quit the app")
	}
}

// Speed must surface the HDR/10-bit warning delivered by the probe.
func TestEffectsWizard_SpeedShowsHDRNote(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	res := videoWithAudio("10")
	res.Streams[0].PixFmt = "yuv420p10le"
	res.Streams[0].ColorTransfer = "smpte2084"
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{in: res}}}
	m := newEffectsWizard(cfg, in)

	m.Update(keyMsg("enter")) // Change Speed… -> factor
	if m.step != "factor" {
		t.Fatalf("expected the factor step, got %q", m.step)
	}
	_, cmd := m.Update(keyMsg("enter")) // 0.25x -> probing
	var probe effectsAudioMsg
	for _, msg := range msgsOf(cmd) {
		if p, ok := msg.(effectsAudioMsg); ok {
			probe = p
		}
	}
	m.Update(probe)

	if m.step != "output" {
		t.Fatalf("expected the output step, got %q", m.step)
	}
	if m.hdrNote == "" || !strings.Contains(m.hdrNote, "HDR10") {
		t.Fatalf("expected an HDR10 note on the output step, got %q", m.hdrNote)
	}
	if !strings.Contains(m.View(), "HDR10") {
		t.Fatal("the output view should render the HDR note")
	}
}
