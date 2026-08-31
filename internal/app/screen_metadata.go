package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fsx8/ffwiz/internal/execx"
	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
)

type metadataProbeMsg struct {
	tags  map[string]string
	count int
	err   error
}

type metadataWizard struct {
	cfg      Config
	filePath string

	opList  list.Model
	title   textinput.Model
	artist  textinput.Model
	comment textinput.Model
	out     textinput.Model
	guard   overwriteGuard

	op          string // "edit" | "strip"
	step        string // "op" | "probing" | "tags" | "confirm" | "output"
	focus       int
	probing     bool
	confirmArm  bool
	streamCount int
	orig        map[string]string // tags as found on the file (for clear detection)
	spin        spinner.Model

	probeCancel context.CancelFunc // cancels the in-flight probe on ctrl+c

	err   string
	style lipgloss.Style
}

var metadataOps = []list.Item{
	simpleItem{value: "Edit Tags…"},
	simpleItem{value: "Strip All Metadata…"},
}

func newMetadataWizard(cfg Config, filePath string) *metadataWizard {
	ol := list.New(metadataOps, list.NewDefaultDelegate(), 0, 0)
	ol.Title = "Edit or strip metadata"
	ol.SetFilteringEnabled(false)
	ol.DisableQuitKeybindings()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return &metadataWizard{
		cfg:      cfg,
		filePath: filePath,
		opList:   ol,
		title:    newShortInput(""),
		artist:   newShortInput(""),
		comment:  newShortInput(""),
		out:      newPathInput(),
		step:     "op",
		spin:     sp,
		style:    lipgloss.NewStyle().Padding(1, 2),
	}
}

func (m *metadataWizard) Init() tea.Cmd { return nil }

func (m *metadataWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.opList.SetSize(dim(msg.Width, 4), dim(msg.Height, 6))
	case metadataProbeMsg:
		return m.applyProbe(msg)
	case spinner.TickMsg:
		if m.step == "probing" && m.probing {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
	case tea.KeyMsg:
		typing := (m.step == "tags" && textInputFocused(m.title, m.artist, m.comment)) ||
			(m.step == "output" && textInputFocused(m.out))
		switch msg.String() {
		case "q":
			if typing {
				break
			}
			return m, tea.Quit
		case "ctrl+c":
			if m.probeCancel != nil {
				m.probeCancel()
				m.probeCancel = nil
			}
			return m, tea.Quit
		case "esc":
			switch m.step {
			case "output":
				m.guard.armedFor = ""
				m.err = ""
				m.out.Blur()
				if m.op == "strip" {
					m.confirmArm = false
					m.step = "confirm"
					return m, nil
				}
				m.step = "tags"
				m.refocusTagInput()
				return m, textinput.Blink
			case "tags":
				m.guard.armedFor = ""
				m.err = ""
				m.blurTagInputs()
				m.step = "op"
				return m, nil
			case "confirm":
				m.confirmArm = false
				m.err = ""
				m.step = "op"
				return m, nil
			case "probing":
				m.probing = false
				m.err = ""
				m.step = "op"
				return m, nil
			default:
				m.guard.armedFor = ""
				m.err = ""
				return m, pop()
			}
		case "tab", "down":
			if m.step == "tags" {
				m.focus = focusStep(m.focus, 3, 1)
				m.refocusTagInput()
				return m, textinput.Blink
			}
		case "shift+tab", "up":
			if m.step == "tags" {
				m.focus = focusStep(m.focus, 3, -1)
				m.refocusTagInput()
				return m, textinput.Blink
			}
		case "enter":
			if m.step == "probing" {
				return m, nil
			}
			switch m.step {
			case "op":
				return m.selectOp()
			case "tags":
				return m.applyTags()
			case "confirm":
				return m.confirmStrip()
			case "output":
				if m.op == "edit" {
					return m.runTags()
				}
				return m.runStrip()
			}
		}
		if m.step == "probing" {
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.step {
	case "tags":
		switch m.focus {
		case 0:
			m.title, cmd = m.title.Update(msg)
		case 1:
			m.artist, cmd = m.artist.Update(msg)
		case 2:
			m.comment, cmd = m.comment.Update(msg)
		}
	case "output":
		m.out, cmd = m.out.Update(msg)
	default:
		m.opList, cmd = m.opList.Update(msg)
	}
	return m, cmd
}

func (m *metadataWizard) selectOp() (tea.Model, tea.Cmd) {
	fi, ok := m.opList.SelectedItem().(simpleItem)
	if !ok {
		return m, nil
	}
	switch fi.value {
	case "Edit Tags…":
		m.op = "edit"
	default:
		m.op = "strip"
	}
	m.step = "probing"
	m.probing = true
	return m, tea.Batch(m.spin.Tick, m.probeCmd())
}

func (m *metadataWizard) probeCmd() tea.Cmd {
	prober := m.cfg.Prober
	path := m.filePath
	if prober == nil {
		return func() tea.Msg { return metadataProbeMsg{err: errors.New("no prober available")} }
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	m.probeCancel = cancel
	return func() tea.Msg {
		defer cancel()
		res, err := prober.Probe(ctx, path)
		if err != nil {
			return metadataProbeMsg{err: err}
		}
		var tags map[string]string
		if res.Format.Tags != nil {
			tags = res.Format.Tags
		} else {
			tags = map[string]string{}
		}
		return metadataProbeMsg{tags: tags, count: len(res.Streams)}
	}
}

func (m *metadataWizard) applyProbe(msg metadataProbeMsg) (tea.Model, tea.Cmd) {
	if m.step != "probing" || !m.probing {
		return m, nil
	}
	m.probing = false
	if msg.err != nil {
		m.err = msg.err.Error()
		m.step = "op"
		return m, nil
	}
	if m.op == "edit" {
		m.orig = msg.tags
		m.title.SetValue(msg.tags["title"])
		m.artist.SetValue(msg.tags["artist"])
		m.comment.SetValue(msg.tags["comment"])
		m.focus = 0
		m.refocusTagInput()
		m.step = "tags"
		return m, textinput.Blink
	}
	m.streamCount = msg.count
	m.confirmArm = false
	m.step = "confirm"
	return m, nil
}

// collectedTags diffs the three fields against the tags found on the file:
// a non-empty field sets the tag, and a field that the user cleared (it was
// prefilled with the existing value) deletes it via an empty -metadata
// value. Tags not shown here are left untouched.
func (m *metadataWizard) collectedTags() map[string]string {
	tags := map[string]string{}
	for key, in := range map[string]*textinput.Model{
		"title":   &m.title,
		"artist":  &m.artist,
		"comment": &m.comment,
	} {
		v := strings.TrimSpace(in.Value())
		switch {
		case v != "":
			tags[key] = v
		case m.orig[key] != "":
			tags[key] = "" // deletion
		}
	}
	return tags
}

// applyTags validates the collected tags and moves to the editable output
// step, mirroring the strip flow (tags -> output -> run).
func (m *metadataWizard) applyTags() (tea.Model, tea.Cmd) {
	if len(m.collectedTags()) == 0 {
		m.err = "nothing to change; edit a field or leave via Esc"
		return m, nil
	}
	m.err = ""
	m.out.SetValue(ffx.SetMetadataOutputName(m.filePath))
	m.step = "output"
	m.out.Focus()
	return m, textinput.Blink
}

// runTags launches the lossless tag update against the (possibly edited)
// output path.
func (m *metadataWizard) runTags() (tea.Model, tea.Cmd) {
	outName := strings.TrimSpace(m.out.Value())
	if outName == "" {
		m.err = "output file is required"
		return m, nil
	}
	m.err = ""
	outPath := resolveOutputPath(filepath.Dir(m.filePath), outName)
	// -y is passed to ffmpeg; make overwriting explicit.
	if m.guard.shouldWarn(outPath) {
		return m, nil
	}
	cmd := ffx.BuildSetMetadataCmd(m.filePath, m.collectedTags(), outPath)
	return m, push(newExecScreen(m.cfg, "Updating metadata…", execx.Cmd{Name: "ffmpeg", Args: cmd.Args}))
}

func (m *metadataWizard) confirmStrip() (tea.Model, tea.Cmd) {
	if !m.confirmArm {
		m.confirmArm = true
		return m, nil
	}
	m.confirmArm = false
	m.err = ""
	m.out.SetValue(ffx.StripMetadataOutputName(m.filePath))
	m.step = "output"
	m.out.Focus()
	return m, textinput.Blink
}

func (m *metadataWizard) runStrip() (tea.Model, tea.Cmd) {
	outName := strings.TrimSpace(m.out.Value())
	if outName == "" {
		m.err = "output file is required"
		return m, nil
	}
	m.err = ""
	outPath := resolveOutputPath(filepath.Dir(m.filePath), outName)
	if m.guard.shouldWarn(outPath) {
		return m, nil
	}
	cmd := ffx.BuildStripMetadataCmd(m.filePath, m.streamCount, outPath)
	return m, push(newExecScreen(m.cfg, "Stripping metadata…", execx.Cmd{Name: "ffmpeg", Args: cmd.Args}))
}

func (m *metadataWizard) refocusTagInput() {
	m.blurTagInputs()
	switch m.focus {
	case 0:
		m.title.Focus()
	case 1:
		m.artist.Focus()
	case 2:
		m.comment.Focus()
	}
}

func (m *metadataWizard) blurTagInputs() {
	m.title.Blur()
	m.artist.Blur()
	m.comment.Blur()
}

func (m *metadataWizard) View() string {
	errLine := renderErrLine(m.err)
	switch m.step {
	case "probing":
		return m.style.Render(m.spin.View() + " Reading metadata…\n\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Esc to go back"))
	case "tags":
		warnLine := renderWarnLine(m.guard)
		return m.style.Render("Edit Tags\n\n" +
			"Title:\n" + m.title.View() + "\n\n" +
			"Artist:\n" + m.artist.View() + "\n\n" +
			"Comment:\n" + m.comment.View() + "\n" +
			errLine + warnLine + "\n\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
				"Tab to switch fields • Enter to continue • Esc to go back • q to quit\nFields are prefilled; clear a field to delete that tag.\nExisting tags not shown here are kept."))
	case "confirm":
		return m.style.Render("Strip All Metadata\n\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(
				"This removes global tags and per-stream language/title tags. Enter again to confirm.") +
			errLine + "\n\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
				"Enter to confirm • Esc to go back • q to quit"))
	case "output":
		warnLine := renderWarnLine(m.guard)
		return m.style.Render("Output file:\n\n" + m.out.View() + errLine + warnLine + "\n\nEnter to run • Esc to go back • q to quit")
	default:
		return m.style.Render(m.opList.View() + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
			"Enter to select • Esc to go back • q to quit"))
	}
}
