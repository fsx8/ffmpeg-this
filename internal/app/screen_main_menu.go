package app

import (
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type menuItem struct {
	title string
	desc  string
	kind  string
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

type mainMenuModel struct {
	cfg   Config
	list  list.Model
	style lipgloss.Style
}

func newMainMenu(cfg Config) *mainMenuModel {
	items := []list.Item{
		menuItem{title: "Process a Single Media File", desc: "Inspect, trim, extract audio, modify tracks", kind: "single"},
		menuItem{title: "Join Multiple Videos", desc: "Concatenate multiple videos (handles res/sample rate)", kind: "join"},
		menuItem{title: "Batch Convert Directory", desc: "Convert all media files in a folder", kind: "batch"},
		menuItem{title: "Exit", desc: "", kind: "exit"},
	}
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "ffwiz — ffmpeg wizard"
	l.SetShowStatusBar(false)
	l.DisableQuitKeybindings()
	l.SetFilteringEnabled(false)

	return &mainMenuModel{
		cfg:   cfg,
		list:  l,
		style: lipgloss.NewStyle().Padding(1, 2),
	}
}

func (m *mainMenuModel) Init() tea.Cmd { return nil }

func (m *mainMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width-4, msg.Height-4)
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "enter":
			if it, ok := m.list.SelectedItem().(menuItem); ok {
				switch it.kind {
				case "single":
					cwd, _ := os.Getwd()
					return m, push(newFilePicker(m.cfg, cwd))
				case "join":
					cwd, _ := os.Getwd()
					return m, push(newJoinWizard(m.cfg, cwd))
				case "batch":
					cwd, _ := os.Getwd()
					return m, push(newBatchWizard(m.cfg, cwd))
				case "exit":
					return m, tea.Quit
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *mainMenuModel) View() string {
	help := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("↑/↓ to navigate • Enter to select • q to quit")
	return m.style.Render(m.list.View() + "\n" + help)
}
