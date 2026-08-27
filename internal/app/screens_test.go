package app

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fsx8/ffwiz/internal/execx"
	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
	"github.com/fsx8/ffwiz/internal/ffprobe"
)

func flagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// --- test doubles ---

type fakeProber struct {
	results   map[string]*ffprobe.ProbeResult
	err       map[string]error
	keyframes map[string][]float64
}

func (f *fakeProber) Probe(_ context.Context, path string) (*ffprobe.ProbeResult, error) {
	if f.err != nil {
		if err, ok := f.err[path]; ok {
			return nil, err
		}
	}
	if res, ok := f.results[path]; ok {
		return res, nil
	}
	return &ffprobe.ProbeResult{}, nil
}

func (f *fakeProber) HasAudio(ctx context.Context, path string) (bool, error) {
	res, err := f.Probe(ctx, path)
	if err != nil {
		return false, err
	}
	for _, s := range res.Streams {
		if s.CodecType == "audio" {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeProber) Keyframes(_ context.Context, path string) ([]float64, error) {
	if kf, ok := f.keyframes[path]; ok {
		return kf, nil
	}
	return nil, nil
}

type fakeRunner struct {
	err error
}

func (f fakeRunner) Run(_ context.Context, _ execx.Cmd) (string, string, error) {
	if f.err != nil {
		return "", "boom stderr", f.err
	}
	return "", "", nil
}

func (f fakeRunner) RunStreaming(_ context.Context, _ execx.Cmd, _, onStderr func(string)) (int, error) {
	if f.err != nil && onStderr != nil {
		onStderr("boom stderr")
		return 1, f.err
	}
	if onStderr != nil {
		onStderr("fake stderr line")
	}
	return 0, nil
}

func videoWithAudio(duration string) *ffprobe.ProbeResult {
	return &ffprobe.ProbeResult{
		Streams: []ffprobe.Stream{
			{Index: 0, CodecType: "video", CodecName: "h264", Width: 640, Height: 360, SampleAspectRatio: "1:1"},
			{Index: 1, CodecType: "audio", CodecName: "aac", SampleRate: "48000"},
		},
		Format: ffprobe.Format{Duration: duration},
	}
}

func audioOnly() *ffprobe.ProbeResult {
	return &ffprobe.ProbeResult{
		Streams: []ffprobe.Stream{{Index: 0, CodecType: "audio", CodecName: "mp3", SampleRate: "44100"}},
		Format:  ffprobe.Format{Duration: "60"},
	}
}

// --- helpers ---

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// msgsOf runs a command (resolving batched commands) to inspect its messages.
func msgsOf(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	switch m := cmd().(type) {
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range m {
			out = append(out, msgsOf(c)...)
		}
		return out
	default:
		return []tea.Msg{m}
	}
}

func hasMsg[T any](msgs []tea.Msg) bool {
	for _, m := range msgs {
		if _, ok := m.(T); ok {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- key routing ---

func TestTrimWizard_TypingQReachesInput(t *testing.T) {
	m := newTrimWizard(Config{}, "/tmp/x.mp4")
	model, cmd := m.Update(keyMsg("q"))
	tw, ok := model.(*trimWizard)
	if !ok {
		t.Fatalf("unexpected model %T", model)
	}
	if tw.start.Value() != "q" {
		t.Fatalf("'q' must reach the focused input, got %q", tw.start.Value())
	}
	if hasMsg[tea.QuitMsg](msgsOf(cmd)) {
		t.Fatal("typing 'q' must not quit the app")
	}
}

func TestTrimWizard_EnterValidatesTimes(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	m := newTrimWizard(Config{}, in)
	m.start.SetValue("10")
	m.end.SetValue("5")

	_, cmd := m.Update(keyMsg("enter"))
	if m.err == "" {
		t.Fatal("expected a validation error for start >= end")
	}
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("must not proceed to the exec screen on invalid times")
	}

	m.end.SetValue("20")
	_, cmd = m.Update(keyMsg("enter"))
	if m.err != "" {
		t.Fatalf("valid range rejected: %s", m.err)
	}
	if m.step != "snapping" {
		t.Fatalf("valid range must enter the keyframe-snapping step, got %q", m.step)
	}
	_, cmd = m.Update(trimKeyframesMsg{})
	if !hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("expected the exec screen to be pushed after keyframe probing")
	}
}

func TestTrimWizard_OverwriteNeedsSecondEnter(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	writeFile(t, dir, "clip.mp4")
	writeFile(t, dir, "clip_trimmed.mp4") // existing output

	m := newTrimWizard(Config{}, in)
	m.start.SetValue("0")
	m.end.SetValue("5")

	_, cmd := m.Update(keyMsg("enter"))
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("first Enter must not overwrite an existing file")
	}
	if m.warnPath == "" {
		t.Fatal("expected an overwrite warning")
	}

	_, cmd = m.Update(keyMsg("enter"))
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("second Enter starts keyframe probing, not the exec screen yet")
	}
	_, cmd = m.Update(trimKeyframesMsg{})
	if !hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("second Enter should confirm the overwrite and run")
	}
}

func execOfPush(t *testing.T, cmd tea.Cmd) *execModel {
	t.Helper()
	for _, msg := range msgsOf(cmd) {
		if p, ok := msg.(pushMsg); ok {
			em, ok := p.m.(*execModel)
			if !ok {
				t.Fatalf("pushed model is %T, want *execModel", p.m)
			}
			return em
		}
	}
	t.Fatal("no pushMsg produced")
	return nil
}

func TestTrimWizard_SnapsStartToPreviousKeyframe(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	kf := []float64{0, 2, 4, 6, 8, 10}
	cfg := Config{Prober: &fakeProber{keyframes: map[string][]float64{in: kf}}}

	m := newTrimWizard(cfg, in)
	m.start.SetValue("5")
	m.end.SetValue("9")

	_, cmd := m.Update(keyMsg("enter"))
	if m.step != "snapping" {
		t.Fatalf("expected snapping step, got %q", m.step)
	}
	_, cmd = m.Update(trimKeyframesMsg{keyframes: kf})
	em := execOfPush(t, cmd)
	if got := flagValue(em.cmd.Args, "-ss"); got != "00:00:04" {
		t.Fatalf("snapped -ss = %q, want 00:00:04 (previous keyframe)", got)
	}
	if got := flagValue(em.cmd.Args, "-to"); got != "00:00:09" {
		t.Fatalf("-to = %q, want 00:00:09 (end unchanged)", got)
	}
	if !strings.Contains(em.title, "00:00:04") {
		t.Fatalf("exec title should surface the snapped start, got %q", em.title)
	}
}

func TestTrimWizard_WithoutKeyframesTrimsUnsnapped(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	m := newTrimWizard(Config{}, in) // nil Prober: immediate empty keyframes
	m.start.SetValue("5")
	m.end.SetValue("9")

	_, cmd := m.Update(keyMsg("enter"))
	_, cmd = m.Update(trimKeyframesMsg{})
	em := execOfPush(t, cmd)
	if got := flagValue(em.cmd.Args, "-ss"); got != "00:00:05" {
		t.Fatalf("-ss = %q, want the unsnapped 00:00:05", got)
	}
	if strings.Contains(em.title, "lossless cut") {
		t.Fatalf("title should not claim snapping, got %q", em.title)
	}
}

func TestFilePicker_ManualModeQTypes(t *testing.T) {
	m := newFilePicker(Config{}, t.TempDir())
	m.mode = "manual"
	m.input.Focus()

	model, cmd := m.Update(keyMsg("q"))
	fp, ok := model.(*filePickerModel)
	if !ok {
		t.Fatalf("unexpected model %T", model)
	}
	if fp.mode != "manual" || fp.input.Value() != "q" {
		t.Fatalf("'q' must reach the path input (mode=%q value=%q)", fp.mode, fp.input.Value())
	}
	if hasMsg[tea.QuitMsg](msgsOf(cmd)) {
		t.Fatal("typing 'q' must not quit the app")
	}
}

func TestFilePicker_EscWhileFilteringClearsFilter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.mp4")
	writeFile(t, dir, "b.mp4")
	m := newFilePicker(Config{}, dir)

	m.Update(keyMsg("/"))
	if m.list.FilterState() == 0 { // list.Unfiltered
		t.Fatal("expected filter to be active after '/'")
	}

	_, cmd := m.Update(keyMsg("esc"))
	if m.list.FilterState() != 0 {
		t.Fatal("esc should clear the filter, not pop the screen")
	}
	if hasMsg[popMsg](msgsOf(cmd)) {
		t.Fatal("esc must not pop the screen while filtering")
	}

	// A second esc (no filter) pops.
	_, cmd = m.Update(keyMsg("esc"))
	if !hasMsg[popMsg](msgsOf(cmd)) {
		t.Fatal("esc without filter should pop the screen")
	}
}

func TestJoinWizard_SpaceWhileFilteringGoesToFilter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a part1.mp4")
	writeFile(t, dir, "b part2.mp4")
	m := newJoinWizard(Config{}, dir)

	m.Update(keyMsg("/"))
	if m.list.FilterState() == 0 {
		t.Fatal("expected filter to be active after '/'")
	}

	m.Update(keyMsg(" "))
	for _, it := range m.list.Items() {
		if ji, ok := it.(*joinItem); ok && ji.selected {
			t.Fatal("space must type into the filter, not toggle selection")
		}
	}
}

func TestTracksWizard_TypingQReachesOutputInput(t *testing.T) {
	m := newTracksWizard(Config{}, "x.mp4")
	m.loading = false
	m.tracks = []trackView{{Track: ffx.Track{Index: 0, Type: ffx.TrackVideo}}}
	m.actions[0] = ffx.TrackActionInfo{Action: ffx.ActionKeep}
	m.step = "output"
	m.out.Focus()

	model, cmd := m.Update(keyMsg("q"))
	tw, ok := model.(*tracksWizard)
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

func TestExtractAudio_EscFromOutputReturnsToFormat(t *testing.T) {
	m := newExtractAudioWizard(Config{}, "x.mp4")
	m.loading = false
	m.hasAudio = true
	m.mode = "output"

	_, cmd := m.Update(keyMsg("esc"))
	if m.mode != "format" {
		t.Fatalf("esc from the output step must return to the format step, got %q", m.mode)
	}
	if hasMsg[popMsg](msgsOf(cmd)) {
		t.Fatal("esc from the output step must not pop the whole wizard")
	}
}

// --- join wizard flow ---

func TestJoinWizard_FullFlowToConfirmAndRun(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.mp4")
	writeFile(t, dir, "b.mp4")
	a := filepath.Join(dir, "a.mp4")
	b := filepath.Join(dir, "b.mp4")

	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		a: videoWithAudio("10"),
		b: videoWithAudio("12"),
	}}}
	m := newJoinWizard(cfg, dir)

	// Select both files (in list order) and continue.
	m.Update(keyMsg(" "))
	m.Update(keyMsg("down"))
	m.Update(keyMsg(" "))
	m.Update(keyMsg("enter"))
	if m.step != "output" {
		t.Fatalf("expected output step, got %q", m.step)
	}

	// Enter output name -> probing starts.
	_, cmd := m.Update(keyMsg("enter"))
	if m.step != "probing" {
		t.Fatalf("expected probing step, got %q", m.step)
	}
	var probe joinProbeMsg
	for _, msg := range msgsOf(cmd) {
		if p, ok := msg.(joinProbeMsg); ok {
			probe = p
		}
	}
	if probe.err != nil {
		t.Fatalf("probe failed: %v", probe.err)
	}
	if len(probe.inputs) != 2 || !probe.inputs[0].HasAudio || probe.inputs[1].DurationSec != 12 {
		t.Fatalf("unexpected probe result: %+v", probe.inputs)
	}
	if probe.target.Width != 640 || probe.target.SampleRate != "48000" {
		t.Fatalf("unexpected target: %+v", probe.target)
	}

	// Probe done -> confirm screen with command preview.
	m.Update(probe)
	if m.step != "confirm" {
		t.Fatalf("expected confirm step, got %q", m.step)
	}

	// Enter runs the join.
	_, cmd = m.Update(keyMsg("enter"))
	if !hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("expected the exec screen to be pushed from confirm")
	}
}

func TestJoinWizard_ProbeErrorReturnsToSelect(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.mp4")
	writeFile(t, dir, "broken.mp4")
	cfg := Config{Prober: &fakeProber{
		err: map[string]error{filepath.Join(dir, "broken.mp4"): errors.New("ffprobe failed")},
	}}
	m := newJoinWizard(cfg, dir)

	m.Update(keyMsg(" "))
	m.Update(keyMsg("down"))
	m.Update(keyMsg(" "))
	m.Update(keyMsg("enter"))
	_, cmd := m.Update(keyMsg("enter")) // -> probing
	var probe joinProbeMsg
	for _, msg := range msgsOf(cmd) {
		if p, ok := msg.(joinProbeMsg); ok {
			probe = p
		}
	}
	if probe.err == nil {
		t.Fatal("expected probe error for unreadable file")
	}
	m.Update(probe)
	if m.step != "select" || m.err == "" {
		t.Fatalf("expected error surfaced on select step, got step=%q err=%q", m.step, m.err)
	}
}

func TestJoinWizard_JoinOrderShownInListOrder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "part2.mp4")
	writeFile(t, dir, "part10.mp4")
	writeFile(t, dir, "part1.mp4")
	m := newJoinWizard(Config{}, dir)

	// Cursor starts at part1 (natural order); select parts in visual order.
	m.Update(keyMsg(" ")) // part1
	m.Update(keyMsg("down"))
	m.Update(keyMsg(" ")) // part2
	m.Update(keyMsg("down"))
	m.Update(keyMsg(" ")) // part10

	got := m.selectedPaths()
	want := []string{
		filepath.Join(dir, "part1.mp4"),
		filepath.Join(dir, "part2.mp4"),
		filepath.Join(dir, "part10.mp4"),
	}
	if len(got) != len(want) {
		t.Fatalf("selected: %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("join order must follow natural list order:\ngot  %#v\nwant %#v", got, want)
		}
	}
	// Order labels: part1 -> 1, part2 -> 2, part10 -> 3.
	items := m.list.Items()
	if ji := items[2].(*joinItem); ji.order != 3 {
		t.Fatalf("part10 should join at position 3, got %d", ji.order)
	}
}

// --- batch ---

func TestBatchRun_SkipsAudioOnlyFilesForVideoTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "video.mp4")
	writeFile(t, dir, "song.mp3")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		filepath.Join(dir, "video.mp4"): videoWithAudio("10"),
		filepath.Join(dir, "song.mp3"):  audioOnly(),
	}}, Runner: fakeRunner{}}

	m := newBatchRun(cfg, dir, "mp4", ffx.QualityMedium)
	msg := m.runBatchCmd()()
	st, ok := msg.(batchStatusMsg)
	if !ok {
		t.Fatalf("unexpected msg %T", msg)
	}
	if st.ok != 1 || st.skipped != 1 || st.fail != 0 {
		t.Fatalf("ok=%d skipped=%d fail=%d; %+v", st.ok, st.skipped, st.fail, st)
	}
}

func TestBatchRun_ConvertsAllForAudioTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "video.mp4")
	writeFile(t, dir, "song.mp3")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		filepath.Join(dir, "video.mp4"): videoWithAudio("10"),
		filepath.Join(dir, "song.mp3"):  audioOnly(),
	}}, Runner: fakeRunner{}}

	m := newBatchRun(cfg, dir, "mp3", ffx.QualityMedium)
	st, ok := m.runBatchCmd()().(batchStatusMsg)
	if !ok {
		t.Fatal("unexpected msg")
	}
	if st.ok != 2 || st.skipped != 0 || st.fail != 0 {
		t.Fatalf("ok=%d skipped=%d fail=%d", st.ok, st.skipped, st.fail)
	}
}

func TestBatchRun_FailuresAreLoggedWithStderr(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "video.mp4")
	var buf bytes.Buffer
	cfg := Config{
		Logger: log.New(&buf, "", 0),
		Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{filepath.Join(dir, "video.mp4"): videoWithAudio("10")}},
		Runner: fakeRunner{err: errors.New("exit status 3")},
	}

	m := newBatchRun(cfg, dir, "mp4", ffx.QualityMedium)
	st, ok := m.runBatchCmd()().(batchStatusMsg)
	if !ok {
		t.Fatal("unexpected msg")
	}
	if st.fail != 1 || st.ok != 0 {
		t.Fatalf("fail=%d ok=%d", st.fail, st.ok)
	}
	if !bytes.Contains(buf.Bytes(), []byte("boom stderr")) {
		t.Fatalf("ffmpeg stderr should be logged for diagnosis, log: %q", buf.String())
	}
}

// --- navigation ---

func TestRootPushPopNavigation(t *testing.T) {
	r, ok := New(Config{}).(*rootModel)
	if !ok {
		t.Fatalf("unexpected model %T", r)
	}
	if len(r.stack) != 1 {
		t.Fatalf("expected a single start screen, got %d", len(r.stack))
	}

	_, cmd := r.Update(pushMsg{m: newMainMenu(Config{})})
	if len(r.stack) != 2 {
		t.Fatalf("push must grow the stack, got %d", len(r.stack))
	}
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("push handler should not re-emit push")
	}

	r.Update(popMsg{})
	if len(r.stack) != 1 {
		t.Fatalf("pop must shrink the stack, got %d", len(r.stack))
	}

	_, cmd = r.Update(popMsg{})
	if !hasMsg[tea.QuitMsg](msgsOf(cmd)) {
		t.Fatal("popping the last screen must quit")
	}
}

func TestRoot_CtrlCQuits(t *testing.T) {
	r, _ := New(Config{}).(*rootModel)
	_, cmd := r.Update(keyMsg("ctrl+c"))
	if !hasMsg[tea.QuitMsg](msgsOf(cmd)) {
		t.Fatal("ctrl+c must quit the program")
	}
}

// --- exec result classification ---

func TestExecScreen_CancelledVersusFailed(t *testing.T) {
	m := newExecScreen(Config{}, "test", execx.Cmd{Name: "true"})

	m.done = &execDoneMsg{exitCode: -1, err: errors.New("signal: killed")}
	if !m.wasCancelled() {
		t.Fatal("killed process (negative exit code) should read as cancelled")
	}

	m.done = &execDoneMsg{exitCode: 1, err: context.Canceled}
	if !m.wasCancelled() {
		t.Fatal("context.Canceled should read as cancelled")
	}

	m.done = &execDoneMsg{exitCode: 3, err: errors.New("command failed (exit 3)")}
	if m.wasCancelled() {
		t.Fatal("a real non-zero exit must not read as cancelled")
	}
}
