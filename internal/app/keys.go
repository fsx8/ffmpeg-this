package app

import (
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// probeTimeout bounds every one-shot ffprobe call issued from a screen.
const probeTimeout = 30 * time.Second

// focusStep moves a field cursor by delta (-1 or 1) around a ring of n
// fields, wrapping in both directions.
func focusStep(cur, n, delta int) int {
	return ((cur+delta)%n + n) % n
}

// textInputFocused reports whether any of the given text inputs has focus,
// meaning ordinary typing (e.g. "q") must reach the input instead of being
// interpreted as an app keybinding.
func textInputFocused(inputs ...textinput.Model) bool {
	for _, in := range inputs {
		if in.Focused() {
			return true
		}
	}
	return false
}

// filterActive reports whether a list's filter is being typed into. While
// typing, keys like q, space, enter and esc must be forwarded to the list.
func filterActive(l list.Model) bool {
	return l.FilterState() == list.Filtering
}

// filterApplied reports whether a confirmed filter is limiting the visible
// items. While applied, screen keys (space, enter, q) work normally again;
// only Esc is special: it clears the filter instead of leaving the screen.
func filterApplied(l list.Model) bool {
	return l.FilterState() == list.FilterApplied
}

// outputExists reports whether path already exists (used for overwrite
// confirmations; commands run with -y).
func outputExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// overwriteGuard arms an explicit overwrite confirmation: the first Enter
// on an existing output arms the warning, the second confirms it. Commands
// run with -y, so overwriting must be a deliberate act.
type overwriteGuard struct {
	armedFor string
}

// shouldWarn returns true when outPath exists but has not been confirmed
// yet, arming the guard in that case. A confirmed or non-existing path
// clears it, so renaming the output rearms the warning.
func (g *overwriteGuard) shouldWarn(outPath string) bool {
	if outputExists(outPath) && g.armedFor != outPath {
		g.armedFor = outPath
		return true
	}
	g.armedFor = ""
	return false
}

// warningText renders the overwrite warning shown while the guard is armed.
func (g *overwriteGuard) warningText(outPath string) string {
	return "Output file exists: " + outPath + "\nPress Enter again to overwrite, or edit the name."
}

// newPathInput returns a text input for file paths, with the same width and
// character limit across all wizards.
func newPathInput() textinput.Model {
	in := textinput.New()
	in.Prompt = "> "
	in.CharLimit = 4096
	in.Width = 50
	return in
}

// resolveOutputPath anchors a relative output name in baseDir; absolute
// names pass through unchanged.
func resolveOutputPath(baseDir, outName string) string {
	if filepath.IsAbs(outName) {
		return outName
	}
	return filepath.Join(baseDir, outName)
}

// renderErrLine renders a wizard error line in red; empty when msg is
// empty, so views can append it unconditionally.
func renderErrLine(msg string) string {
	if msg == "" {
		return ""
	}
	return "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(msg)
}

// renderWarnLine renders an armed overwrite-guard warning in orange;
// empty when no output path is armed.
func renderWarnLine(g overwriteGuard) string {
	if g.armedFor == "" {
		return ""
	}
	return "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(g.warningText(g.armedFor))
}

// newShortInput returns a short single-line text input (time specs, CRF,
// crop fields, metadata tags, …) with the same width and character limit
// across all wizards.
func newShortInput(placeholder string) textinput.Model {
	in := textinput.New()
	in.Placeholder = placeholder
	in.Prompt = "> "
	in.CharLimit = 32
	in.Width = 30
	return in
}

// simpleItem is a plain single-column list entry.
type simpleItem struct {
	value string
}

func (i simpleItem) Title() string       { return i.value }
func (i simpleItem) Description() string { return "" }
func (i simpleItem) FilterValue() string { return i.value }

// dirItem is a navigable directory entry in a file list; name ".." marks
// the parent directory.
type dirItem struct {
	name string
	path string // absolute directory path
}

func (i dirItem) Title() string       { return i.name + "/" }
func (i dirItem) Description() string { return "" }
func (i dirItem) FilterValue() string { return i.name }
