package app

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type rootModel struct {
	cfg   Config
	stack []tea.Model
	w, h  int
}

func New(cfg Config) tea.Model {
	r := &rootModel{cfg: cfg}

	start := tea.Model(newMainMenu(cfg))
	if cfg.InitialPath != "" {
		if fi, err := os.Stat(cfg.InitialPath); err == nil {
			if fi.IsDir() {
				start = tea.Model(newJoinWizard(cfg, cfg.InitialPath))
			} else {
				start = tea.Model(newActionMenu(cfg, cfg.InitialPath))
			}
		}
	}

	r.stack = []tea.Model{start}
	return r
}

func (m *rootModel) Init() tea.Cmd {
	if len(m.stack) == 0 {
		return tea.Quit
	}
	return m.stack[len(m.stack)-1].Init()
}

func (m *rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			// Let the active screen cancel in-flight work (e.g. a running
			// ffmpeg) before quitting, so no child process is orphaned.
			if len(m.stack) > 0 {
				updated, _ := m.stack[len(m.stack)-1].Update(msg)
				m.stack[len(m.stack)-1] = updated
			}
			return m, tea.Quit
		}
	case pushMsg:
		if m.w > 0 && m.h > 0 {
			updated, _ := msg.m.Update(tea.WindowSizeMsg{Width: m.w, Height: m.h})
			msg.m = updated
		}
		m.stack = append(m.stack, msg.m)
		return m, msg.m.Init()
	case popMsg:
		if len(m.stack) > 1 {
			m.stack = m.stack[:len(m.stack)-1]
		} else {
			return m, tea.Quit
		}
		return m, nil
	}

	cur := m.stack[len(m.stack)-1]
	next, cmd := cur.Update(msg)
	m.stack[len(m.stack)-1] = next
	return m, cmd
}

func (m *rootModel) View() string {
	if len(m.stack) == 0 {
		return ""
	}
	return m.stack[len(m.stack)-1].View()
}
