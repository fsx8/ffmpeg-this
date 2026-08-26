package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fsx8/ffwiz/internal/media"
)

type fileItem struct {
	title string
	path  string
	kind  string // "file" | "manual" | "back"
}

func (i fileItem) Title() string       { return i.title }
func (i fileItem) Description() string { return "" }
func (i fileItem) FilterValue() string { return i.title }

type filePickerModel struct {
	cfg   Config
	dir   string
	list  list.Model
	input textinput.Model
	mode  string // "list" | "manual"
	err   string
	style lipgloss.Style
}

func newFilePicker(cfg Config, dir string) *filePickerModel {
	files, _ := media.ListMediaFiles(dir)
	items := make([]list.Item, 0, len(files)+3)
	for _, f := range files {
		items = append(items, fileItem{title: f, path: filepath.Join(dir, f), kind: "file"})
	}
	items = append(items, fileItem{title: "Enter a path…", kind: "manual"})
	items = append(items, fileItem{title: "Back", kind: "back"})

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select a media file"
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()

	in := textinput.New()
	in.Placeholder = "/path/to/file.mp4"
	in.Prompt = "> "
	in.CharLimit = 4096
	in.Width = 60

	return &filePickerModel{
		cfg:   cfg,
		dir:   dir,
		list:  l,
		input: in,
		mode:  "list",
		style: lipgloss.NewStyle().Padding(1, 2),
	}
}

func (m *filePickerModel) Init() tea.Cmd { return nil }

func (m *filePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width-4, msg.Height-6)
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.mode == "manual" {
				m.mode = "list"
				m.err = ""
				m.input.Blur()
				return m, nil
			}
			return m, pop()
		case "q":
			return m, tea.Quit
		case "enter":
			if m.mode == "manual" {
				p := strings.TrimSpace(m.input.Value())
				if p == "" {
					m.err = "enter a file path"
					return m, nil
				}
				ap, err := filepath.Abs(p)
				if err != nil {
					m.err = "invalid path"
					return m, nil
				}
				if fi, err := os.Stat(ap); err != nil || fi.IsDir() {
					m.err = "file does not exist"
					return m, nil
				}
				return m, push(newActionMenu(m.cfg, ap))
			}

			if it, ok := m.list.SelectedItem().(fileItem); ok {
				switch it.kind {
				case "file":
					return m, push(newActionMenu(m.cfg, it.path))
				case "manual":
					m.mode = "manual"
					m.err = ""
					m.input.SetValue("")
					m.input.Focus()
					return m, textinput.Blink
				case "back":
					return m, pop()
				}
			}
		}
	}

	if m.mode == "manual" {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *filePickerModel) View() string {
	if m.mode == "manual" {
		errLine := ""
		if m.err != "" {
			errLine = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.err)
		}
		return m.style.Render("Enter a media file path:\n\n" + m.input.View() + errLine + "\n\nesc to go back")
	}
	return m.style.Render(m.list.View() + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Enter to select • Esc to go back • q to quit"))
}
