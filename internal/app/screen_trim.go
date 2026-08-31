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
	dur       float64 // container duration in seconds; 0 when unknown
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

	focus int
	err   string
	guard overwriteGuard

	step        string // "form" | "snapping"
	spin        spinner.Model
	pending     trimPending
	probeCancel context.CancelFunc // cancels the in-flight keyframe probe on Esc

	style lipgloss.Style
}

func newTrimWizard(cfg Config, filePath string) *trimWizard {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	start := newShortInput("HH:MM:SS or seconds")
	end := newShortInput("HH:MM:SS or seconds")
	out := newPathInput()
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
		// The probe kept running while the user backed out; its result is
		// stale and must not launch a cancelled trim (or attach to pending
		// values the user has since changed).
		if m.step != "snapping" {
			return m, nil
		}
		return m.startTrim(msg)
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.step == "snapping" {
				if m.probeCancel != nil {
					m.probeCancel()
					m.probeCancel = nil
				}
				m.step = "form"
				return m, textinput.Blink
			}
			m.guard.armedFor = ""
			m.err = ""
			return m, pop()
		case "ctrl+c":
			if m.probeCancel != nil {
				m.probeCancel()
				m.probeCancel = nil
			}
			return m, tea.Quit
		}
		if m.step != "form" {
			return m, nil
		}
		switch msg.String() {
		case "tab", "down":
			m.focus = focusStep(m.focus, 3, 1)
			m.updateFocus()
			return m, textinput.Blink
		case "shift+tab", "up":
			m.focus = focusStep(m.focus, 3, -1)
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
				m.guard.armedFor = ""
				return m, nil
			}
			m.err = ""
			outPath := resolveOutputPath(filepath.Dir(m.filePath), outName)
			// -y is passed to ffmpeg; make overwriting explicit.
			if m.guard.shouldWarn(outPath) {
				return m, nil
			}
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

// probeKeyframesCmd fetches the keyframe list (for lossless snapping) and,
// independently, the container duration so a start beyond EOF can be
// rejected before ffmpeg runs. A keyframe failure must not disable the
// duration check, so both results travel in one message. The parent
// context is created before the command is returned and stored as
// m.probeCancel, so Esc aborts both probes instead of letting them run to
// completion behind a backed-out screen.
func (m *trimWizard) probeKeyframesCmd() tea.Cmd {
	prober := m.cfg.Prober
	path := m.filePath
	if prober == nil {
		return func() tea.Msg { return trimKeyframesMsg{} }
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.probeCancel = cancel
	return func() tea.Msg {
		defer cancel()
		msg := trimKeyframesMsg{}
		kfCtx, kfCancel := context.WithTimeout(ctx, probeTimeout)
		kf, err := prober.Keyframes(kfCtx, path)
		kfCancel()
		msg.keyframes, msg.err = kf, err

		probeCtx, probeCancel := context.WithTimeout(ctx, probeTimeout)
		defer probeCancel()
		if res, err := prober.Probe(probeCtx, path); err == nil && res != nil {
			msg.dur, _ = res.Duration()
		}
		return msg
	}
}

// startTrim snaps the cut start to a keyframe and launches the exec screen.
// Without keyframes (probe failed or unavailable) the trim runs unsnapped —
// the legacy behavior — rather than blocking the user.
func (m *trimWizard) startTrim(msg trimKeyframesMsg) (tea.Model, tea.Cmd) {
	p := m.pending
	if msg.dur > 0 && p.startSec >= msg.dur-0.05 {
		m.step = "form"
		m.err = fmt.Sprintf("start %s is at or after the end of the file (%s) — nothing to cut",
			ffx.FormatTimeSpec(p.startSec), formatDur(time.Duration(msg.dur*float64(time.Second))))
		return m, nil
	}
	snapped := ffx.SnapToKeyframe(p.startSec, msg.keyframes)
	// Snapping can move the start forward (when it precedes the first
	// keyframe). Refuse cuts that would end up empty instead of handing
	// ffmpeg a start beyond its end.
	if snapped >= p.endSec {
		m.step = "form"
		m.err = fmt.Sprintf("keyframe snapping moves the cut start to %s, which is not before the end %s — choose a later end time",
			ffx.FormatTimeSpec(snapped), ffx.FormatTimeSpec(p.endSec))
		return m, nil
	}
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
	errLine := renderErrLine(m.err)
	warnLine := renderWarnLine(m.guard)
	s := "Trim Video (lossless)\n\n" +
		"Start time:\n" + m.start.View() + "\n\n" +
		"End time:\n" + m.end.View() + "\n\n" +
		"Output file:\n" + m.out.View() + "\n" +
		errLine + warnLine + "\n\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
			"Tab to switch fields • Enter to run • Esc to go back • Ctrl+C to quit\nNote: with -c copy the start is snapped to the previous keyframe, so the\ncut may begin slightly earlier than requested (lossless, audio in sync).")
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
