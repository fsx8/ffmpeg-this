package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fsx8/ffwiz/internal/execx"
	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
	ffprobe "github.com/fsx8/ffwiz/internal/ffprobe"
)

type execLineMsg struct{ line string }

type execProgMsg struct{ line string }

type execProbeMsg struct{ total time.Duration }

type execDoneMsg struct {
	exitCode int
	err      error
}

type execTickMsg struct{}

type execModel struct {
	cfg   Config
	title string
	cmd   execx.Cmd

	ctx    context.Context
	cancel context.CancelFunc

	spin spinner.Model
	vp   viewport.Model

	lines    []string
	maxLines int

	lineCh  chan string
	progCh  chan string
	probeCh chan time.Duration
	doneCh  chan execDoneMsg

	tracker   ffx.ProgressTracker
	dur       time.Duration
	probing   bool
	startedAt time.Time

	running bool
	done    *execDoneMsg
	style   lipgloss.Style
}

func newExecScreen(cfg Config, title string, cmd execx.Cmd) *execModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	ctx, cancel := context.WithCancel(context.Background())
	return &execModel{
		cfg:       cfg,
		title:     title,
		cmd:       cmd,
		ctx:       ctx,
		cancel:    cancel,
		spin:      sp,
		maxLines:  200,
		probing:   true,
		startedAt: time.Now(),
		style:     lipgloss.NewStyle().Padding(1, 2),
	}
}

func (m *execModel) Init() tea.Cmd {
	m.running = true
	m.lineCh = make(chan string, 256)
	m.progCh = make(chan string, 64)
	m.probeCh = make(chan time.Duration, 1)
	m.doneCh = make(chan execDoneMsg, 1)

	cmd := ffx.AddProgressArgs(m.cmd.Args)
	streamCmd := execx.Cmd{Name: m.cmd.Name, Args: cmd}
	go func() {
		m.probeCh <- execTotalDuration(m.ctx, m.cfg.Prober, streamCmd.Args)
	}()

	go func() {
		exitCode, err := m.cfg.Runner.RunStreaming(m.ctx, streamCmd,
			func(line string) {
				select {
				case m.progCh <- line:
				default:
				}
			},
			func(line string) {
				select {
				case m.lineCh <- line:
				default:
					// Drop if UI can't keep up.
				}
				if m.cfg.Logger != nil {
					m.cfg.Logger.Printf("%s", line)
				}
			})
		m.doneCh <- execDoneMsg{exitCode: exitCode, err: err}
	}()

	return tea.Batch(m.spin.Tick, pollExec(m))
}

func pollExec(m *execModel) tea.Cmd {
	return func() tea.Msg {
		select {
		case line := <-m.lineCh:
			return execLineMsg{line: line}
		case line := <-m.progCh:
			return execProgMsg{line: line}
		case dur := <-m.probeCh:
			return execProbeMsg{total: dur}
		case done := <-m.doneCh:
			return done
		case <-time.After(100 * time.Millisecond):
			return execTickMsg{}
		}
	}
}

func (m *execModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Keep the scroll position across resizes instead of snapping.
		atBottom := m.vp.AtBottom()
		offset := m.vp.YOffset
		m.vp = viewport.New(msg.Width-4, msg.Height-9)
		m.vp.SetContent(m.renderLog())
		if atBottom {
			m.vp.GotoBottom()
		} else {
			m.vp.SetYOffset(offset)
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			if m.running && m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		case "ctrl+c":
			if m.running && m.cancel != nil {
				m.cancel()
			}
			return m, nil
		case "esc":
			if m.running && m.cancel != nil {
				m.cancel()
				return m, nil
			}
			return m, pop()
		case "enter":
			if m.done != nil {
				return m, pop()
			}
		}
	case spinner.TickMsg:
		if m.running {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
	case execLineMsg:
		if msg.line != "" {
			m.lines = append(m.lines, msg.line)
			if len(m.lines) > m.maxLines {
				m.lines = m.lines[len(m.lines)-m.maxLines:]
			}
			// Only follow the output if the user is already at the bottom,
			// so scrolling back through the log is not fought by new lines.
			atBottom := m.vp.AtBottom()
			m.vp.SetContent(m.renderLog())
			if atBottom {
				m.vp.GotoBottom()
			}
		}
		return m, pollExec(m)
	case execProgMsg:
		m.tracker.Observe(msg.line)
		return m, pollExec(m)
	case execProbeMsg:
		m.dur = msg.total
		m.probing = false
		return m, pollExec(m)
	case execTickMsg:
		if m.running {
			return m, pollExec(m)
		}
	case execDoneMsg:
		m.running = false
		m.done = &msg
		m.probing = false
		if msg.err == nil && m.dur > 0 {
			m.tracker.Complete(m.dur)
		}
		m.vp.SetContent(m.renderLog())
		return m, nil
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m *execModel) View() string {
	header := lipgloss.NewStyle().Bold(true).Render(m.title) + "\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(m.cmd.String()) + "\n\n"

	if prog := m.progressLine(); m.running && prog != "" {
		header += prog + "\n\n"
	}

	footer := ""
	switch {
	case m.running:
		footer = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(m.spin.View()+" Running… (Esc cancels • ↑/↓ scroll)")
	case m.done == nil:
		footer = ""
	case m.done.err != nil && m.wasCancelled():
		footer = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("Cancelled.\nEnter to go back")
	case m.done.err != nil:
		footer = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Failed: "+m.done.err.Error()+"\nEnter to go back")
	default:
		footer = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("Done.\nEnter to go back")
	}

	return m.style.Render(header + m.vp.View() + footer)
}

// progressLine renders the live percentage bar during execution; empty when
// duration info is unavailable or nothing has been processed yet.
func (m *execModel) progressLine() string {
	if m.probing || m.dur <= 0 {
		if s := m.tracker.Sample(); s.OutTime > 0 {
			return fmt.Sprintf("%s processed",
				lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(formatDur(s.OutTime)))
		}
		return ""
	}
	s := m.tracker.Sample()
	if s.OutTime <= 0 {
		return ""
	}
	pct := s.Percent(m.dur)
	bar := renderBar(progressWidth(m.vp.Width), pct)
	info := fmt.Sprintf(" %3.0f%%  %s / %s", pct*100, formatDur(s.OutTime), formatDur(m.dur))
	if elapsed := time.Since(m.startedAt); pct > 0.03 && elapsed > 2*time.Second {
		eta := time.Duration(float64(elapsed)/pct*(1-pct)) / time.Second * time.Second
		info += "  ETA " + formatDur(eta)
	}
	if s.Speed > 0 {
		info += fmt.Sprintf("  %.1fx", s.Speed)
	}
	info = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(info)
	return lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(bar) + info
}

func progressWidth(vpWidth int) int {
	w := vpWidth - 24
	if w > 60 {
		w = 60
	}
	if w < 20 {
		w = 20
	}
	return w
}

// wasCancelled distinguishes a user cancellation (context cancel kills the
// process with a negative exit code) from a genuine command failure.
func (m *execModel) wasCancelled() bool {
	return errors.Is(m.done.err, context.Canceled) || m.done.exitCode < 0
}

func (m *execModel) renderLog() string {
	if len(m.lines) == 0 {
		return ""
	}
	out := ""
	for _, l := range m.lines {
		out += l + "\n"
	}
	return out
}

// execTotalDuration sums the probed durations of every `-i` input; concat-style
// outputs (join) then match exactly and single-input commands trivially. Zero
// means unknown — the UI falls back to an indeterminate indicator.
func execTotalDuration(ctx context.Context, prober ffprobe.Prober, args []string) time.Duration {
	var total time.Duration
	for i, a := range args {
		if a != "-i" || i+1 >= len(args) {
			continue
		}
		res, err := prober.Probe(ctx, args[i+1])
		if err != nil || res == nil {
			continue
		}
		if d, ok := parseProbeSeconds(res.Format.Duration); ok {
			total += d
		}
	}
	return total
}
