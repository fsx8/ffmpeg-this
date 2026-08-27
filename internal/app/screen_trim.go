package app

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fsx8/ffwiz/internal/execx"
	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
)

type trimWizard struct {
	cfg      Config
	filePath string

	start textinput.Model
	end   textinput.Model
	out   textinput.Model

	focus    int
	err      string
	warnPath string // output path the overwrite warning was armed for

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
		style:    lipgloss.NewStyle().Padding(1, 2),
	}
}

func (m *trimWizard) Init() tea.Cmd { return textinput.Blink }

func (m *trimWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.warnPath = ""
			m.err = ""
			return m, pop()
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
			cmd := ffx.BuildTrimCmd(m.filePath, start, end, outPath)
			return m, push(newExecScreen(m.cfg, "Trimming video…", execx.Cmd{Name: "ffmpeg", Args: cmd.Args}))
		}
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

func (m *trimWizard) View() string {
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
			"Tab to switch fields • Enter to run • Esc to go back\nNote: -c copy cuts on keyframes; the cut may differ slightly from the times you enter.")
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
