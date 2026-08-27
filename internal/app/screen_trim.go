package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fsx8/ffwiz/internal/execx"
	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
)

type trimKeyframesMsg struct {
	keyframes []float64
	err       error
}

type trimPending struct {
	startSec float64
	endSec   float64
	outPath  string
}

type trimWizard struct {
	cfg      Config
	filePath string

	start textinput.Model
	end   textinput.Model
	out   textinput.Model

	focus    int
	err      string
	warnPath string // output path the overwrite warning was armed for

	step    string // "form" | "snapping"
	spin    spinner.Model
	pending trimPending

	style lipgloss.Style
}

func newTrimWizard(cfg Config, filePath string) *trimWizard {
	makeInput := func(ph string) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = ph
		ti.Prompt = "> "
		ti.CharLimit = 64
		ti.Width = 30
		return ti
	}
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	start := makeInput("HH:MM:SS or seconds")
	end := makeInput("HH:MM:SS or seconds")
	out := makeInput(ffx.TrimOutputName(filePath))
	out.SetValue(ffx.TrimOutputName(filePath))
	start.Focus()
	return &trimWizard{
		cfg:      cfg,
		filePath: filePath,
		start:    start,
		end:      end,
		out:      out,
		focus:    0,
		step:     "form",
		spin:     sp,
		style:    lipgloss.NewStyle().Padding(1, 2),
	}
}

func (m *trimWizard) Init() tea.Cmd { return textinput.Blink }

func (m *trimWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case trimKeyframesMsg:
		return m.startTrim(msg)
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.step == "snapping" {
				m.step = "form"
				return m, textinput.Blink
			}
			m.warnPath = ""
			m.err = ""
			return m, pop()
		}
		if m.step != "form" {
			return m, nil
		}
		switch msg.String() {
		case "tab", "down":
			m.focus = (m.focus + 1) % 3
			m.updateFocus()
			return m, textinput.Blink
		case "shift+tab", "up":
			m.focus = (m.focus + 2) % 3
			m.updateFocus()
			return m, textinput.Blink
		case "enter":
			start := strings.TrimSpace(m.start.Value())
			end := strings.TrimSpace(m.end.Value())
			outName := strings.TrimSpace(m.out.Value())
			if start == "" || end == "" || outName == "" {
				m.err = "start, end, and output are required"
				return m, nil
			}
			if err := ffx.ValidateTrim(start, end); err != nil {
				m.err = err.Error()
				m.warnPath = ""
				return m, nil
			}
			m.err = ""
			outPath := outName
			if !filepath.IsAbs(outPath) {
				outPath = filepath.Join(filepath.Dir(m.filePath), outName)
			}
			// -y is passed to ffmpeg; make overwriting explicit.
			if outputExists(outPath) && m.warnPath != outPath {
				m.warnPath = outPath
				return m, nil
			}
			m.warnPath = ""
			startSec, _ := ffx.ParseTimeSpec(start)
			endSec, _ := ffx.ParseTimeSpec(end)
			m.pending = trimPending{startSec: startSec, endSec: endSec, outPath: outPath}
			m.step = "snapping"
			return m, tea.Batch(m.spin.Tick, m.probeKeyframesCmd())
		}
	case spinner.TickMsg:
		if m.step == "snapping" {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
	}

	if m.step != "form" {
		return m, nil
	}
	var cmd tea.Cmd
	switch m.focus {
	case 0:
		m.start, cmd = m.start.Update(msg)
	case 1:
		m.end, cmd = m.end.Update(msg)
	case 2:
		m.out, cmd = m.out.Update(msg)
	}
	return m, cmd
}

func (m *trimWizard) probeKeyframesCmd() tea.Cmd {
	prober := m.cfg.Prober
	path := m.filePath
	if prober == nil {
		return func() tea.Msg { return trimKeyframesMsg{} }
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		kf, err := prober.Keyframes(ctx, path)
		return trimKeyframesMsg{keyframes: kf, err: err}
	}
}

// startTrim snaps the cut start to a keyframe and launches the exec screen.
// Without keyframes (probe failed or unavailable) the trim runs unsnapped —
// the legacy behavior — rather than blocking the user.
func (m *trimWizard) startTrim(msg trimKeyframesMsg) (tea.Model, tea.Cmd) {
	p := m.pending
	snapped := ffx.SnapToKeyframe(p.startSec, msg.keyframes)
	title := "Trimming video…"
	if snapped < p.startSec-0.05 {
		title = fmt.Sprintf("Trimming video… (lossless cut starts at %s)", ffx.FormatTimeSpec(snapped))
		if m.cfg.Logger != nil {
			m.cfg.Logger.Printf("trim: start %s snapped to keyframe %s",
				ffx.FormatTimeSpec(p.startSec), ffx.FormatTimeSpec(snapped))
		}
	}
	cmd := ffx.BuildTrimCmd(m.filePath, ffx.FormatTimeSpec(snapped), ffx.FormatTimeSpec(p.endSec), p.outPath)
	return m, push(newExecScreen(m.cfg, title, execx.Cmd{Name: "ffmpeg", Args: cmd.Args}))
}

func (m *trimWizard) View() string {
	if m.step == "snapping" {
		return m.style.Render("Trim Video (lossless)\n\n" +
			m.spin.View() + " Finding keyframes for a lossless cut…\n\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Esc to go back"))
	}
	errLine := ""
	if m.err != "" {
		errLine = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.err)
	}
	warnLine := ""
	if m.warnPath != "" {
		warnLine = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(
			"Output file exists: "+m.warnPath+"\nPress Enter again to overwrite, or edit the name.",
		)
	}
	s := "Trim Video (lossless)\n\n" +
		"Start time:\n" + m.start.View() + "\n\n" +
		"End time:\n" + m.end.View() + "\n\n" +
		"Output file:\n" + m.out.View() + "\n" +
		errLine + warnLine + "\n\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
			"Tab to switch fields • Enter to run • Esc to go back\nNote: with -c copy the start is snapped to the previous keyframe, so the\ncut may begin slightly earlier than requested (lossless, audio in sync).")
	return m.style.Render(s)
}

func (m *trimWizard) updateFocus() {
	m.start.Blur()
	m.end.Blur()
	m.out.Blur()
	switch m.focus {
	case 0:
		m.start.Focus()
	case 1:
		m.end.Focus()
	case 2:
		m.out.Focus()
	}
}
