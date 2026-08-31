package app

import (
	"path/filepath"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type actionItem struct {
	title string
	desc  string
	kind  string
}

func (i actionItem) Title() string       { return i.title }
func (i actionItem) Description() string { return i.desc }
func (i actionItem) FilterValue() string { return i.title }

type actionMenuModel struct {
	cfg      Config
	filePath string
	list     list.Model
	style    lipgloss.Style
}

func newActionMenu(cfg Config, filePath string) *actionMenuModel {
	items := []list.Item{
		actionItem{title: "Inspect File Details", desc: "Show ffprobe info", kind: "inspect"},
		actionItem{title: "Modify Tracks", desc: "Keep/remove/convert video/audio/subs", kind: "tracks"},
		actionItem{title: "Trim Video", desc: "Lossless cut with -c copy", kind: "trim"},
		actionItem{title: "Extract Audio", desc: "Rip audio to mp3/flac/wav", kind: "audio"},
		actionItem{title: "Resize Video", desc: "Scale to 480p-2160p or custom size", kind: "resize"},
		actionItem{title: "Rotate / Flip / Crop", desc: "Rotate 90/180/270, mirror, or crop", kind: "transform"},
		actionItem{title: "Compress Video", desc: "Shrink with a CRF quality preset", kind: "compress"},
		actionItem{title: "Speed / Reverse / Mute", desc: "Change speed, play backwards, or remove audio", kind: "effects"},
		actionItem{title: "Take Screenshot", desc: "Grab a single frame as png/jpg", kind: "screenshot"},
		actionItem{title: "Edit Metadata", desc: "Edit title/artist/comment or strip all tags", kind: "metadata"},
		actionItem{title: "Back", kind: "back"},
	}
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Actions for: " + filepath.Base(filePath)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()
	return &actionMenuModel{
		cfg:      cfg,
		filePath: filePath,
		list:     l,
		style:    lipgloss.NewStyle().Padding(1, 2),
	}
}

func (m *actionMenuModel) Init() tea.Cmd { return nil }

func (m *actionMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(dim(msg.Width, 4), dim(msg.Height, 4))
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, pop()
		case "q":
			return m, tea.Quit
		case "enter":
			if it, ok := m.list.SelectedItem().(actionItem); ok {
				switch it.kind {
				case "inspect":
					return m, push(newInspectScreen(m.cfg, m.filePath))
				case "tracks":
					return m, push(newTracksWizard(m.cfg, m.filePath))
				case "trim":
					return m, push(newTrimWizard(m.cfg, m.filePath))
				case "audio":
					return m, push(newExtractAudioWizard(m.cfg, m.filePath))
				case "resize":
					return m, push(newResizeWizard(m.cfg, m.filePath))
				case "transform":
					return m, push(newTransformWizard(m.cfg, m.filePath))
				case "compress":
					return m, push(newCompressWizard(m.cfg, m.filePath))
				case "effects":
					return m, push(newEffectsWizard(m.cfg, m.filePath))
				case "screenshot":
					return m, push(newScreenshotWizard(m.cfg, m.filePath))
				case "metadata":
					return m, push(newMetadataWizard(m.cfg, m.filePath))
				case "back":
					return m, pop()
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *actionMenuModel) View() string {
	help := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Enter to select • Esc to go back • q to quit")
	return m.style.Render(m.list.View() + "\n" + help)
}
