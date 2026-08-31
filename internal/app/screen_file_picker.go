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

	listW, listH int // kept across refreshes so navigation preserves the size
}

func newFilePicker(cfg Config, dir string) *filePickerModel {
	m := &filePickerModel{
		cfg:   cfg,
		dir:   dir,
		mode:  "list",
		input: newPathInput(),
		style: lipgloss.NewStyle().Padding(1, 2),
	}
	m.input.Placeholder = "/path/to/file.mp4"
	m.refresh()
	return m
}

// refresh rebuilds the list for m.dir: parent entry, subdirectories, then
// media files, plus the fixed manual-entry/back actions. Errors are
// surfaced in the view instead of silently showing an empty list.
func (m *filePickerModel) refresh() {
	files, dirs, err := media.ListDir(m.dir)
	var items []list.Item
	if err != nil {
		m.err = err.Error()
	} else {
		m.err = ""
		if parent := filepath.Dir(m.dir); parent != m.dir {
			items = append(items, dirItem{name: "..", path: parent})
		}
		for _, d := range dirs {
			items = append(items, dirItem{name: d, path: filepath.Join(m.dir, d)})
		}
		for _, f := range files {
			items = append(items, fileItem{title: f, path: filepath.Join(m.dir, f), kind: "file"})
		}
	}
	items = append(items, fileItem{title: "Enter a path…", kind: "manual"})
	items = append(items, fileItem{title: "Back", kind: "back"})

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select a media file"
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	if m.listW > 0 {
		l.SetSize(m.listW, m.listH)
	}
	m.list = l
}

// navigate enters a subdirectory (or the parent for "..") and refreshes.
func (m *filePickerModel) navigate(d dirItem) {
	m.dir = d.path
	m.refresh()
}

func (m *filePickerModel) Init() tea.Cmd { return nil }

func (m *filePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.listW, m.listH = dim(msg.Width, 4), dim(msg.Height, 6)
		m.list.SetSize(m.listW, m.listH)
	case tea.KeyMsg:
		typingPath := m.mode == "manual" && textInputFocused(m.input)
		filtering := m.mode == "list" && filterActive(m.list)

		switch msg.String() {
		case "esc":
			if filtering {
				break // let the list clear/leave its filter
			}
			if filterApplied(m.list) {
				m.list.ResetFilter()
				return m, nil
			}
			if m.mode == "manual" {
				m.mode = "list"
				m.err = ""
				m.input.Blur()
				return m, nil
			}
			return m, pop()
		case "q":
			if typingPath || filtering {
				break
			}
			return m, tea.Quit
		case "enter":
			if filtering {
				break // let the list apply the filter
			}
			if m.mode == "manual" {
				return m, m.submitManualPath()
			}

			switch it := m.list.SelectedItem().(type) {
			case dirItem:
				m.navigate(it)
				return m, nil
			case fileItem:
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

// submitManualPath validates the manually entered path and opens the action
// menu for it.
func (m *filePickerModel) submitManualPath() tea.Cmd {
	p := strings.TrimSpace(m.input.Value())
	if p == "" {
		m.err = "enter a file path"
		return nil
	}
	ap, err := filepath.Abs(p)
	if err != nil {
		m.err = "invalid path"
		return nil
	}
	if fi, err := os.Stat(ap); err != nil || fi.IsDir() {
		m.err = "file does not exist"
		return nil
	}
	return push(newActionMenu(m.cfg, ap))
}

func (m *filePickerModel) View() string {
	if m.mode == "manual" {
		errLine := ""
		if m.err != "" {
			errLine = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.err)
		}
		return m.style.Render("Enter a media file path:\n\n" + m.input.View() + errLine + "\n\nesc to go back")
	}
	errLine := ""
	if m.err != "" {
		errLine = renderErrLine(m.err)
	}
	info := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		"Directory: " + m.dir + "\nEnter to select • / to filter • Esc to go back • q to quit")
	return m.style.Render(m.list.View() + "\n" + info + errLine)
}
