package app

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
	"github.com/fsx8/ffwiz/internal/ffprobe"
)

func metadataProbeOf(t *testing.T, cmd tea.Cmd) metadataProbeMsg {
	t.Helper()
	for _, msg := range msgsOf(cmd) {
		if p, ok := msg.(metadataProbeMsg); ok {
			return p
		}
	}
	t.Fatal("no metadataProbeMsg produced")
	return metadataProbeMsg{}
}

func selectMetadataOp(t *testing.T, m *metadataWizard, index int) tea.Cmd {
	t.Helper()
	m.opList.Select(index)
	_, cmd := m.Update(keyMsg("enter"))
	return cmd
}

func argIndex(args []string, arg string) int {
	for i, a := range args {
		if a == arg {
			return i
		}
	}
	return -1
}

func taggedResult(tags map[string]string) *ffprobe.ProbeResult {
	res := videoWithAudio("10")
	res.Format.Tags = tags
	return res
}

func TestMetadataWizard_EditFlowProbesAndPrefillsTags(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: taggedResult(map[string]string{"title": "Old Title", "artist": "Old Artist"}),
	}}}
	m := newMetadataWizard(cfg, in)

	cmd := selectMetadataOp(t, m, 0) // Edit Tags… -> probing
	if m.step != "probing" || !m.probing {
		t.Fatalf("expected the probing step, got step=%q probing=%v", m.step, m.probing)
	}
	probe := metadataProbeOf(t, cmd)
	if probe.err != nil {
		t.Fatalf("probe failed: %v", probe.err)
	}

	m.Update(probe)
	if m.step != "tags" {
		t.Fatalf("expected the tags step after the probe, got %q", m.step)
	}
	if got := m.title.Value(); got != "Old Title" {
		t.Fatalf("title prefill = %q, want %q", got, "Old Title")
	}
	if got := m.artist.Value(); got != "Old Artist" {
		t.Fatalf("artist prefill = %q, want %q", got, "Old Artist")
	}
	if got := m.comment.Value(); got != "" {
		t.Fatalf("comment prefill = %q, want empty (tag absent)", got)
	}
}

func TestMetadataWizard_EditEmptyFieldsShowsError(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: taggedResult(nil),
	}}}
	m := newMetadataWizard(cfg, in)

	m.Update(metadataProbeOf(t, selectMetadataOp(t, m, 0)))
	_, cmd := m.Update(keyMsg("enter"))
	if m.err == "" {
		t.Fatal("expected an error when every field is empty")
	}
	if !strings.Contains(m.err, "nothing to change") {
		t.Fatalf("error = %q, want it to mention nothing to change", m.err)
	}
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("must not proceed to the exec screen with nothing to set")
	}
}

func TestMetadataWizard_EditHappyPathPushesExec(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: taggedResult(map[string]string{"title": "Old"}),
	}}}
	m := newMetadataWizard(cfg, in)

	m.Update(metadataProbeOf(t, selectMetadataOp(t, m, 0)))
	m.title.SetValue("New")
	m.comment.SetValue("A comment")
	_, cmd := m.Update(keyMsg("enter"))
	if m.step != "output" {
		t.Fatalf("expected the editable output step after the tags step, got %q", m.step)
	}
	if got, want := m.out.Value(), ffx.SetMetadataOutputName(in); got != want {
		t.Fatalf("output prefill = %q, want %q", got, want)
	}

	_, cmd = m.Update(keyMsg("enter"))
	em := execOfPush(t, cmd)
	idxTitle := argIndex(em.cmd.Args, "title=New")
	idxComment := argIndex(em.cmd.Args, "comment=A comment")
	if idxTitle < 0 || idxComment < 0 {
		t.Fatalf("expected both tags in args: %#v", em.cmd.Args)
	}
	if idxComment > idxTitle {
		t.Fatalf("tags must be sorted by key (comment before title): %#v", em.cmd.Args)
	}
	if got := flagValue(em.cmd.Args, "-c"); got != "copy" {
		t.Fatalf("remux must be lossless, -c = %q", got)
	}
	if got, want := em.cmd.Args[len(em.cmd.Args)-1], filepath.Join(dir, "clip_tagged.mp4"); got != want {
		t.Fatalf("output path = %q, want %q", got, want)
	}
	if em.title != "Updating metadata…" {
		t.Fatalf("exec title = %q, want %q", em.title, "Updating metadata…")
	}
}

func TestMetadataWizard_EditOverwriteNeedsSecondEnter(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	writeFile(t, dir, "clip_tagged.mp4")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: taggedResult(nil),
	}}}
	m := newMetadataWizard(cfg, in)

	m.Update(metadataProbeOf(t, selectMetadataOp(t, m, 0)))
	m.title.SetValue("New")
	m.Update(keyMsg("enter")) // tags -> output
	if m.step != "output" {
		t.Fatalf("expected the output step, got %q", m.step)
	}

	_, cmd := m.Update(keyMsg("enter"))
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("first Enter at the output step must not overwrite an existing file")
	}
	if m.guard.armedFor == "" {
		t.Fatal("expected an overwrite warning")
	}

	_, cmd = m.Update(keyMsg("enter"))
	if !hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("second Enter should confirm the overwrite and run")
	}
}

// The Edit Tags output step is editable: a renamed output must be used
// (resolved next to the source) by the final command.
func TestMetadataWizard_EditUsesEditedOutputName(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: taggedResult(map[string]string{"title": "Old"}),
	}}}
	m := newMetadataWizard(cfg, in)

	m.Update(metadataProbeOf(t, selectMetadataOp(t, m, 0)))
	m.title.SetValue("New")
	m.Update(keyMsg("enter")) // tags -> output
	m.out.SetValue("renamed.mp4")
	_, cmd := m.Update(keyMsg("enter"))

	em := execOfPush(t, cmd)
	if got, want := em.cmd.Args[len(em.cmd.Args)-1], filepath.Join(dir, "renamed.mp4"); got != want {
		t.Fatalf("output path = %q, want the edited name %q", got, want)
	}
}

func TestMetadataWizard_StripFlowProbesStreamCount(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "movie.mkv")
	res := videoWithAudio("10")
	res.Streams = append(res.Streams, ffprobe.Stream{Index: 2, CodecType: "subtitle"})
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{in: res}}}
	m := newMetadataWizard(cfg, in)

	cmd := selectMetadataOp(t, m, 1) // Strip All Metadata… -> probing
	if m.step != "probing" || !m.probing {
		t.Fatalf("expected the probing step, got step=%q probing=%v", m.step, m.probing)
	}
	probe := metadataProbeOf(t, cmd)
	m.Update(probe)
	if m.step != "confirm" {
		t.Fatalf("expected the confirm step after the probe, got %q", m.step)
	}

	_, cmd = m.Update(keyMsg("enter"))
	if m.step != "confirm" {
		t.Fatalf("first Enter must only arm the strip confirmation, got step %q", m.step)
	}
	if hasMsg[pushMsg](msgsOf(cmd)) {
		t.Fatal("first Enter at the confirm step must not run anything")
	}

	m.Update(keyMsg("enter"))
	if m.step != "output" {
		t.Fatalf("second Enter must move to the output step, got %q", m.step)
	}
	if got, want := m.out.Value(), ffx.StripMetadataOutputName(in); got != want {
		t.Fatalf("output prefill = %q, want %q", got, want)
	}

	_, cmd = m.Update(keyMsg("enter"))
	em := execOfPush(t, cmd)
	joined := strings.Join(em.cmd.Args, "\x00")
	for _, want := range []string{"-map_metadata\x00-1", "-map_chapters\x00-1", "-metadata:s:2\x00title="} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in args: %#v", want, em.cmd.Args)
		}
	}
	if em.title != "Stripping metadata…" {
		t.Fatalf("exec title = %q, want %q", em.title, "Stripping metadata…")
	}
	if got, want := em.cmd.Args[len(em.cmd.Args)-1], filepath.Join(dir, "movie_stripped.mkv"); got != want {
		t.Fatalf("output path = %q, want %q", got, want)
	}
}

func TestMetadataWizard_EscStepsBackThroughSteps(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: taggedResult(nil),
	}}}
	m := newMetadataWizard(cfg, in)

	m.Update(metadataProbeOf(t, selectMetadataOp(t, m, 0))) // edit -> probing -> tags
	if m.step != "tags" {
		t.Fatalf("expected the tags step, got %q", m.step)
	}
	m.Update(keyMsg("esc"))
	if m.step != "op" {
		t.Fatalf("esc from tags must return to the op step, got %q", m.step)
	}
	_, cmd := m.Update(keyMsg("esc"))
	if !hasMsg[popMsg](msgsOf(cmd)) {
		t.Fatal("esc on the first step should pop the wizard")
	}

	// strip flow: esc during probing returns to op and drops the late result
	cmd = selectMetadataOp(t, m, 1)
	m.Update(keyMsg("esc"))
	if m.step != "op" || m.probing {
		t.Fatalf("esc during probing must return to the op step, got step=%q probing=%v", m.step, m.probing)
	}
	m.Update(metadataProbeOf(t, cmd))
	if m.step != "op" {
		t.Fatalf("a probe result arriving after esc must be dropped, got step %q", m.step)
	}

	m.Update(metadataProbeOf(t, selectMetadataOp(t, m, 1)))
	if m.step != "confirm" {
		t.Fatalf("expected the confirm step, got %q", m.step)
	}
	m.Update(keyMsg("esc"))
	if m.step != "op" {
		t.Fatalf("esc from confirm must return to the op step, got %q", m.step)
	}

	m.Update(metadataProbeOf(t, selectMetadataOp(t, m, 1)))
	m.Update(keyMsg("enter")) // arm
	m.Update(keyMsg("enter")) // -> output
	m.Update(keyMsg("esc"))
	if m.step != "confirm" {
		t.Fatalf("esc from output must return to the confirm step, got %q", m.step)
	}
}

func TestMetadataWizard_TypingQReachesInputs(t *testing.T) {
	m := newMetadataWizard(Config{}, "/tmp/clip.mp4")
	m.step = "tags"
	m.focus = 0
	m.refocusTagInput()

	model, cmd := m.Update(keyMsg("q"))
	mw, ok := model.(*metadataWizard)
	if !ok {
		t.Fatalf("unexpected model %T", model)
	}
	if mw.title.Value() != "q" {
		t.Fatalf("'q' must reach the title input, got %q", mw.title.Value())
	}
	if hasMsg[tea.QuitMsg](msgsOf(cmd)) {
		t.Fatal("typing 'q' must not quit the app")
	}

	m.step = "output"
	m.op = "strip"
	m.blurTagInputs()
	m.out.Focus()
	model, cmd = m.Update(keyMsg("q"))
	mw, ok = model.(*metadataWizard)
	if !ok {
		t.Fatalf("unexpected model %T", model)
	}
	if mw.out.Value() != "q" {
		t.Fatalf("'q' must reach the output input, got %q", mw.out.Value())
	}
	if hasMsg[tea.QuitMsg](msgsOf(cmd)) {
		t.Fatal("typing 'q' must not quit the app")
	}
}

// A field that was prefilled with an existing tag and then cleared must
// delete the tag (empty -metadata value) instead of silently keeping it.
func TestMetadataWizard_ClearingPrefilledTagDeletesIt(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: taggedResult(map[string]string{"title": "Old Title"}),
	}}}
	m := newMetadataWizard(cfg, in)

	m.Update(metadataProbeOf(t, selectMetadataOp(t, m, 0)))
	m.title.SetValue("")      // clear the prefilled title
	m.Update(keyMsg("enter")) // tags -> output
	_, cmd := m.Update(keyMsg("enter"))

	em := execOfPush(t, cmd)
	args := em.cmd.Args
	idx := argIndex(args, "-metadata")
	if idx < 0 || args[idx+1] != "title=" {
		t.Fatalf("expected a deleting -metadata title= argument, got %#v", args)
	}
	for _, a := range args {
		if a == "title=Old Title" {
			t.Fatal("the old title must not be re-written")
		}
	}
}

// Esc from the tags step must disarm an armed overwrite warning so a later
// Enter cannot run on stale confirmation state.
func TestMetadataWizard_EscFromTagsDisarmsOverwriteGuard(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	writeFile(t, dir, "clip_tagged.mp4") // existing output
	cfg := Config{Prober: &fakeProber{results: map[string]*ffprobe.ProbeResult{
		in: taggedResult(nil),
	}}}
	m := newMetadataWizard(cfg, in)

	m.Update(metadataProbeOf(t, selectMetadataOp(t, m, 0)))
	m.title.SetValue("New")
	m.Update(keyMsg("enter")) // tags -> output
	m.Update(keyMsg("enter")) // arms the guard (output exists)
	if m.guard.armedFor == "" {
		t.Fatal("expected the guard to be armed")
	}
	m.Update(keyMsg("esc")) // back to tags
	if m.step != "tags" {
		t.Fatalf("esc from the output step must return to tags, got %q", m.step)
	}
	if m.guard.armedFor != "" {
		t.Fatalf("esc must disarm the guard, armedFor=%q", m.guard.armedFor)
	}
	m.Update(keyMsg("esc")) // back to op
	m.Update(metadataProbeOf(t, selectMetadataOp(t, m, 0)))
	if m.guard.armedFor != "" {
		t.Fatalf("a fresh flow must start disarmed, armedFor=%q", m.guard.armedFor)
	}
}
