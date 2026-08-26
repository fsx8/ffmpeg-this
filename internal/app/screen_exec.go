package app

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fsx8/ffwiz/internal/execx"
)

type execLineMsg struct{ line string }

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

	lineCh chan string
	doneCh chan execDoneMsg

	running bool
	done    *execDoneMsg
	style   lipgloss.Style
}

func newExecScreen(cfg Config, title string, cmd execx.Cmd) *execModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	ctx, cancel := context.WithCancel(context.Background())
	return &execModel{
		cfg:      cfg,
		title:    title,
		cmd:      cmd,
		ctx:      ctx,
		cancel:   cancel,
		spin:     sp,
		maxLines: 200,
		style:    lipgloss.NewStyle().Padding(1, 2),
	}
}

func (m *execModel) Init() tea.Cmd {
	m.running = true
	m.lineCh = make(chan string, 256)
	m.doneCh = make(chan execDoneMsg, 1)

	go func() {
		exitCode, err := m.cfg.Runner.RunStreaming(m.ctx, m.cmd, func(line string) {
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

	return tea.Batch(m.spin.Tick, pollExec(m.lineCh, m.doneCh))
}

func pollExec(lineCh <-chan string, doneCh <-chan execDoneMsg) tea.Cmd {
	return func() tea.Msg {
		select {
		case line := <-lineCh:
			return execLineMsg{line: line}
		case done := <-doneCh:
			return done
		case <-time.After(100 * time.Millisecond):
			return execTickMsg{}
		}
	}
}

func (m *execModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.vp = viewport.New(msg.Width-4, msg.Height-8)
		m.vp.SetContent(m.renderLog())
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
			m.vp.SetContent(m.renderLog())
			m.vp.GotoBottom()
		}
		return m, pollExec(m.lineCh, m.doneCh)
	case execTickMsg:
		if m.running {
			return m, pollExec(m.lineCh, m.doneCh)
		}
	case execDoneMsg:
		m.running = false
		m.done = &msg
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

	footer := ""
	if m.running {
		footer = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(m.spin.View()+" Running… (Esc cancels)")
	} else if m.done != nil && m.done.err != nil {
		footer = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Failed: "+m.done.err.Error()+"\nEnter to go back")
	} else {
		footer = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("Done.\nEnter to go back")
	}

	return m.style.Render(header + m.vp.View() + footer)
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
