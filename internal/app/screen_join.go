package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fsx8/ffwiz/internal/execx"
	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
	"github.com/fsx8/ffwiz/internal/media"
)

type joinItem struct {
	name     string
	absPath  string
	selected bool
}

func (i *joinItem) Title() string {
	box := "[ ]"
	if i.selected {
		box = "[x]"
	}
	return box + " " + i.name
}
func (i *joinItem) Description() string { return "" }
func (i *joinItem) FilterValue() string { return i.name }

type joinWizard struct {
	cfg Config
	dir string

	list list.Model
	out  textinput.Model

	step string // "select" | "output"
	err  string

	style lipgloss.Style
}

func newJoinWizard(cfg Config, dir string) *joinWizard {
	j := &joinWizard{
		cfg:   cfg,
		dir:   dir,
		step:  "select",
		style: lipgloss.NewStyle().Padding(1, 2),
	}

	j.refreshFiles()

	out := textinput.New()
	out.Prompt = "> "
	out.CharLimit = 4096
	out.Width = 40
	out.SetValue("joined_video.mp4")
	j.out = out
	return j
}

func (m *joinWizard) refreshFiles() {
	files, _ := media.ListMediaFiles(m.dir)
	var items []list.Item
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if ext != ".mp4" && ext != ".mkv" && ext != ".mov" && ext != ".avi" && ext != ".webm" {
			continue
		}
		items = append(items, &joinItem{name: f, absPath: filepath.Join(m.dir, f)})
	}
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select videos to join (space toggles)"
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	m.list = l
}

func (m *joinWizard) Init() tea.Cmd { return nil }

func (m *joinWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width-4, msg.Height-6)
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "esc":
			if m.step == "output" {
				m.step = "select"
				m.out.Blur()
				return m, nil
			}
			return m, pop()
		case " ":
			if m.step == "select" {
				if it, ok := m.list.SelectedItem().(*joinItem); ok {
					it.selected = !it.selected
					return m, nil
				}
			}
		case "enter":
			switch m.step {
			case "select":
				selected := m.selectedPaths()
				if len(selected) < 2 {
					m.err = "select at least two videos"
					return m, nil
				}
				m.err = ""
				m.step = "output"
				m.out.Focus()
				return m, textinput.Blink
			case "output":
				outName := strings.TrimSpace(m.out.Value())
				if outName == "" {
					m.err = "output file name is required"
					return m, nil
				}
				outPath := outName
				if !filepath.IsAbs(outPath) {
					outPath = filepath.Join(m.dir, outName)
				}
				selected := m.selectedPaths()
				first := selected[0]
				target, err := m.joinTargets(first)
				if err != nil {
					m.err = err.Error()
					return m, nil
				}
				ff := ffx.BuildJoinCmd(selected, outPath, target)
				return m, push(newExecScreen(m.cfg, "Joining videos…", execx.Cmd{Name: "ffmpeg", Args: ff.Args}))
			}
		}
	}

	if m.step == "output" {
		var cmd tea.Cmd
		m.out, cmd = m.out.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *joinWizard) View() string {
	if m.step == "output" {
		errLine := ""
		if m.err != "" {
			errLine = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.err)
		}
		return m.style.Render("Output file:\n\n" + m.out.View() + errLine + "\n\nEnter to start • Esc to go back")
	}

	info := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Space toggles selection • Enter to continue • Esc to go back")
	errLine := ""
	if m.err != "" {
		errLine = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.err)
	}
	return m.style.Render(m.list.View() + "\n" + info + errLine)
}

func (m *joinWizard) selectedPaths() []string {
	var selected []string
	for _, it := range m.list.Items() {
		if ji, ok := it.(*joinItem); ok && ji.selected {
			selected = append(selected, ji.absPath)
		}
	}
	return selected
}

func (m *joinWizard) joinTargets(firstPath string) (ffx.JoinTargets, error) {
	if _, err := os.Stat(firstPath); err != nil {
		return ffx.JoinTargets{}, err
	}
	probe, err := m.cfg.Prober.Probe(context.Background(), firstPath)
	if err != nil {
		return ffx.JoinTargets{}, err
	}
	var vWidth, vHeight int
	sar := "1:1"
	var sampleRate string
	for _, s := range probe.Streams {
		if s.CodecType == "video" && vWidth == 0 {
			vWidth, vHeight = s.Width, s.Height
			if s.SampleAspectRatio != "" {
				sar = s.SampleAspectRatio
			}
		}
		if s.CodecType == "audio" && sampleRate == "" {
			sampleRate = s.SampleRate
		}
	}
	if vWidth == 0 || vHeight == 0 {
		return ffx.JoinTargets{}, errors.New("could not determine target resolution from first video")
	}
	if sampleRate == "" {
		return ffx.JoinTargets{}, errors.New("could not determine target audio sample rate from first video")
	}
	return ffx.JoinTargets{Width: vWidth, Height: vHeight, SAR: sar, SampleRate: sampleRate}, nil
}
