package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fsx8/ffwiz/internal/execx"
	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
)

type audioCheckDoneMsg struct {
	hasAudio bool
	err      error
}

type extractAudioWizard struct {
	cfg      Config
	filePath string

	loading  bool
	spin     spinner.Model
	hasAudio bool

	formatList list.Model
	out        textinput.Model
	guard      overwriteGuard

	mode  string // "format" | "output"
	err   string
	style lipgloss.Style
}

func newExtractAudioWizard(cfg Config, filePath string) *extractAudioWizard {
	items := []list.Item{simpleItem{value: "mp3"}, simpleItem{value: "flac"}, simpleItem{value: "wav"}}
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select audio format"
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return &extractAudioWizard{
		cfg:        cfg,
		filePath:   filePath,
		loading:    true,
		spin:       sp,
		formatList: l,
		out:        newPathInput(),
		mode:       "format",
		style:      lipgloss.NewStyle().Padding(1, 2),
	}
}

func (m *extractAudioWizard) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, func() tea.Msg {
		if m.cfg.Prober == nil {
			return audioCheckDoneMsg{err: errors.New("ffprobe unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
		defer cancel()
		has, err := m.cfg.Prober.HasAudio(ctx, m.filePath)
		return audioCheckDoneMsg{hasAudio: has, err: err}
	})
}

func (m *extractAudioWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.formatList.SetSize(dim(msg.Width, 4), dim(msg.Height, 6))
	case tea.KeyMsg:
		typing := m.mode == "output" && textInputFocused(m.out)
		switch msg.String() {
		case "esc":
			if m.mode == "output" {
				m.mode = "format"
				m.out.Blur()
				m.err = ""
				m.guard.armedFor = ""
				return m, nil
			}
			return m, pop()
		case "q":
			if typing {
				break
			}
			return m, tea.Quit
		case "enter":
			if m.loading || !m.hasAudio {
				return m, nil
			}
			switch m.mode {
			case "format":
				fi, ok := m.formatList.SelectedItem().(simpleItem)
				if !ok {
					return m, nil
				}
				m.out.SetValue(ffx.ExtractAudioOutputName(m.filePath, fi.value))
				m.mode = "output"
				m.out.Focus()
				return m, textinput.Blink
			case "output":
				fi, ok := m.formatList.SelectedItem().(simpleItem)
				if !ok {
					return m, nil
				}
				outName := strings.TrimSpace(m.out.Value())
				if outName == "" {
					m.err = "output file is required"
					return m, nil
				}
				m.err = ""
				outPath := resolveOutputPath(filepath.Dir(m.filePath), outName)
				// -y is passed to ffmpeg; make overwriting explicit.
				if m.guard.shouldWarn(outPath) {
					return m, nil
				}
				cmd := ffx.BuildExtractAudioCmd(m.filePath, fi.value, outPath)
				return m, push(newExecScreen(m.cfg, "Extracting audio…", execx.Cmd{Name: "ffmpeg", Args: cmd.Args}))
			}
		}
	}

	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
	case audioCheckDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.hasAudio = msg.hasAudio
		if !m.hasAudio {
			m.err = "no audio stream found in this file"
		}
		return m, nil
	}

	if m.mode == "output" {
		var cmd tea.Cmd
		m.out, cmd = m.out.Update(msg)
		return m, cmd
	}
	var cmd tea.Cmd
	m.formatList, cmd = m.formatList.Update(msg)
	return m, cmd
}

func (m *extractAudioWizard) View() string {
	if m.loading {
		return m.style.Render(m.spin.View() + " Checking for audio…\n\nEsc to go back • q to quit")
	}
	if !m.hasAudio {
		return m.style.Render(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.err) + "\n\nEsc to go back • q to quit")
	}

	errLine := renderErrLine(m.err)

	if m.mode == "output" {
		warnLine := renderWarnLine(m.guard)
		return m.style.Render("Output file:\n\n" + m.out.View() + errLine + warnLine + "\n\nEnter to run • Esc to go back • q to quit")
	}
	return m.style.Render(m.formatList.View() + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Enter to select format • Esc to go back • q to quit"))
}
