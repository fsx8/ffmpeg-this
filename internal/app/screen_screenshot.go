package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fsx8/ffwiz/internal/execx"
	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
)

type screenshotDurMsg struct {
	dur float64 // container duration in seconds; 0 when unknown
	err error
}

type screenshotWizard struct {
	cfg      Config
	filePath string

	formatList list.Model
	timestamp  textinput.Model
	out        textinput.Model
	guard      overwriteGuard

	step   string // "format" | "timestamp" | "probing" | "output"
	format string
	ts     string
	spin   spinner.Model

	probeCancel context.CancelFunc // cancels the in-flight duration probe on ctrl+c

	err   string
	style lipgloss.Style
}

func newScreenshotWizard(cfg Config, filePath string) *screenshotWizard {
	items := []list.Item{simpleItem{value: "png"}, simpleItem{value: "jpg"}}
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select image format"
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()

	ts := newShortInput("HH:MM:SS or seconds")
	out := newPathInput()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return &screenshotWizard{
		cfg:        cfg,
		filePath:   filePath,
		formatList: l,
		timestamp:  ts,
		out:        out,
		step:       "format",
		spin:       sp,
		style:      lipgloss.NewStyle().Padding(1, 2),
	}
}

func (m *screenshotWizard) Init() tea.Cmd { return nil }

func (m *screenshotWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.formatList.SetSize(dim(msg.Width, 4), dim(msg.Height, 6))
	case spinner.TickMsg:
		if m.step == "probing" {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
	case screenshotDurMsg:
		return m.applyDur(msg)
	case tea.KeyMsg:
		typing := (m.step == "timestamp" && textInputFocused(m.timestamp)) ||
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
				m.step = "timestamp"
				m.timestamp.Focus()
				return m, textinput.Blink
			case "probing":
				m.err = ""
				m.step = "timestamp"
				m.timestamp.Focus()
				return m, textinput.Blink
			case "timestamp":
				m.err = ""
				m.timestamp.Blur()
				m.step = "format"
				return m, nil
			default:
				m.guard.armedFor = ""
				m.err = ""
				return m, pop()
			}
		case "enter":
			switch m.step {
			case "format":
				return m.selectFormat()
			case "timestamp":
				return m.applyTimestamp()
			case "output":
				return m.run()
			}
		}
		if m.step == "probing" {
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.step {
	case "timestamp":
		m.timestamp, cmd = m.timestamp.Update(msg)
	case "output":
		m.out, cmd = m.out.Update(msg)
	default:
		m.formatList, cmd = m.formatList.Update(msg)
	}
	return m, cmd
}

func (m *screenshotWizard) selectFormat() (tea.Model, tea.Cmd) {
	fi, ok := m.formatList.SelectedItem().(simpleItem)
	if !ok {
		return m, nil
	}
	m.format = fi.value
	m.step = "timestamp"
	m.timestamp.Focus()
	return m, textinput.Blink
}

func (m *screenshotWizard) applyTimestamp() (tea.Model, tea.Cmd) {
	ts := strings.TrimSpace(m.timestamp.Value())
	if _, err := ffx.ParseTimeSpec(ts); err != nil {
		m.err = "timestamp: " + err.Error()
		return m, nil
	}
	m.err = ""
	m.ts = ts
	// Probe the duration so a timestamp beyond EOF can be rejected here
	// instead of failing inside ffmpeg with an empty-output error.
	m.step = "probing"
	m.timestamp.Blur()
	return m, tea.Batch(m.spin.Tick, m.probeCmd())
}

func (m *screenshotWizard) probeCmd() tea.Cmd {
	prober := m.cfg.Prober
	path := m.filePath
	if prober == nil {
		return func() tea.Msg { return screenshotDurMsg{} }
	}
	// The parent context is created before the command is returned and
	// stored as m.probeCancel, so ctrl+c aborts the probe instead of
	// letting it run to completion behind a quitting screen.
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	m.probeCancel = cancel
	return func() tea.Msg {
		defer cancel()
		res, err := prober.Probe(ctx, path)
		if err != nil {
			return screenshotDurMsg{err: err}
		}
		dur, _ := res.Duration()
		return screenshotDurMsg{dur: dur}
	}
}

func (m *screenshotWizard) applyDur(msg screenshotDurMsg) (tea.Model, tea.Cmd) {
	if m.step != "probing" {
		return m, nil
	}
	// An unknown duration (probe failed) must not block the user.
	if msg.err == nil && msg.dur > 0 {
		sec, _ := ffx.ParseTimeSpec(m.ts)
		if sec >= msg.dur-0.05 {
			m.err = fmt.Sprintf("timestamp %s is at or after the end of the file (%s)",
				m.ts, formatDur(time.Duration(msg.dur*float64(time.Second))))
			m.step = "timestamp"
			m.timestamp.Focus()
			return m, textinput.Blink
		}
	}
	m.out.SetValue(ffx.ScreenshotOutputName(m.filePath, m.ts, m.format))
	m.step = "output"
	m.out.Focus()
	return m, textinput.Blink
}

func (m *screenshotWizard) run() (tea.Model, tea.Cmd) {
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
	cmd := ffx.BuildScreenshotCmd(m.filePath, m.ts, outPath)
	return m, push(newExecScreen(m.cfg, "Taking screenshot…", execx.Cmd{Name: "ffmpeg", Args: cmd.Args}))
}

func (m *screenshotWizard) View() string {
	errLine := renderErrLine(m.err)
	switch m.step {
	case "probing":
		return m.style.Render("Take Screenshot\n\n" +
			m.spin.View() + " Checking file duration…\n\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Esc to go back"))
	case "timestamp":
		return m.style.Render("Take Screenshot\n\n" +
			"Timestamp:\n" + m.timestamp.View() + "\n" +
			errLine + "\n\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
				"Enter to continue • Esc to go back • q to quit"))
	case "output":
		warnLine := renderWarnLine(m.guard)
		return m.style.Render("Output file:\n\n" + m.out.View() + errLine + warnLine + "\n\nEnter to run • Esc to go back • q to quit")
	default:
		return m.style.Render(m.formatList.View() + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
			"Enter to select format • Esc to go back • q to quit"))
	}
}
