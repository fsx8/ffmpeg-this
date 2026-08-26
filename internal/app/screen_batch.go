package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fsx8/ffwiz/internal/execx"
	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
	"github.com/fsx8/ffwiz/internal/media"
)

type batchWizard struct {
	cfg     Config
	dir     string
	step    string // "format" | "quality" | "confirm"
	format  string
	quality ffx.BatchVideoQuality

	list  list.Model
	style lipgloss.Style
}

func newBatchWizard(cfg Config, dir string) *batchWizard {
	items := []list.Item{
		formatItem{"mp4"}, formatItem{"mkv"}, formatItem{"mov"}, formatItem{"avi"}, formatItem{"webm"},
		formatItem{"mp3"}, formatItem{"flac"}, formatItem{"wav"},
		formatItem{"gif"},
	}
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Batch convert: select output format"
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()
	return &batchWizard{
		cfg:     cfg,
		dir:     dir,
		step:    "format",
		list:    l,
		quality: ffx.QualityMedium,
		style:   lipgloss.NewStyle().Padding(1, 2),
	}
}

func (m *batchWizard) Init() tea.Cmd { return nil }

func (m *batchWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width-4, msg.Height-6)
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "esc":
			if m.step == "quality" {
				m.step = "format"
				m.setFormatList()
				return m, nil
			}
			return m, pop()
		case "enter":
			fi, ok := m.list.SelectedItem().(formatItem)
			if !ok {
				return m, nil
			}
			switch m.step {
			case "format":
				m.format = fi.v
				if isVideoFormat(m.format) {
					m.step = "quality"
					m.setQualityList()
					return m, nil
				}
				m.step = "confirm"
				return m, push(newBatchRun(m.cfg, m.dir, m.format, m.quality))
			case "quality":
				switch fi.v {
				case "Same as source":
					m.quality = ffx.QualitySame
				case "High (CRF 18)":
					m.quality = ffx.QualityHigh
				case "Medium (CRF 23)":
					m.quality = ffx.QualityMedium
				case "Low (CRF 28)":
					m.quality = ffx.QualityLow
				}
				m.step = "confirm"
				return m, push(newBatchRun(m.cfg, m.dir, m.format, m.quality))
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *batchWizard) View() string {
	return m.style.Render(m.list.View() + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Enter to select • Esc to go back"))
}

func (m *batchWizard) setFormatList() {
	items := []list.Item{
		formatItem{"mp4"}, formatItem{"mkv"}, formatItem{"mov"}, formatItem{"avi"}, formatItem{"webm"},
		formatItem{"mp3"}, formatItem{"flac"}, formatItem{"wav"},
		formatItem{"gif"},
	}
	m.list.Title = "Batch convert: select output format"
	m.list.SetItems(items)
}

func (m *batchWizard) setQualityList() {
	items := []list.Item{
		formatItem{"Same as source"},
		formatItem{"High (CRF 18)"},
		formatItem{"Medium (CRF 23)"},
		formatItem{"Low (CRF 28)"},
	}
	m.list.Title = "Select quality preset"
	m.list.SetItems(items)
}

func isVideoFormat(format string) bool {
	switch format {
	case "mp4", "mkv", "mov", "avi", "webm":
		return true
	default:
		return false
	}
}

type batchStatusMsg struct {
	ok      int
	fail    int
	skipped int
	last    string
	err     error
}

type batchRunModel struct {
	cfg     Config
	dir     string
	format  string
	quality ffx.BatchVideoQuality

	ctx    context.Context
	cancel context.CancelFunc

	spin    spinner.Model
	running bool

	ok      int
	fail    int
	skipped int
	last    string
	err     string

	style lipgloss.Style
}

func newBatchRun(cfg Config, dir, format string, quality ffx.BatchVideoQuality) *batchRunModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	ctx, cancel := context.WithCancel(context.Background())
	return &batchRunModel{
		cfg:     cfg,
		dir:     dir,
		format:  format,
		quality: quality,
		ctx:     ctx,
		cancel:  cancel,
		spin:    sp,
		running: true,
		style:   lipgloss.NewStyle().Padding(1, 2),
	}
}

func (m *batchRunModel) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, m.runBatchCmd())
}

func (m *batchRunModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
		case "esc", "enter":
			if !m.running {
				return m, pop()
			}
		}
	case spinner.TickMsg:
		if m.running {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
	case batchStatusMsg:
		m.running = false
		m.ok = msg.ok
		m.fail = msg.fail
		m.skipped = msg.skipped
		m.last = msg.last
		if msg.err != nil {
			m.err = msg.err.Error()
		}
		return m, nil
	}
	return m, nil
}

func (m *batchRunModel) View() string {
	header := lipgloss.NewStyle().Bold(true).Render("Batch conversion") + "\n"
	if m.running {
		header += m.spin.View() + " Running…\n"
	} else {
		header += lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("Done.\n")
	}
	stats := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		fmt.Sprintf("OK: %d  Failed: %d  Skipped: %d", m.ok, m.fail, m.skipped),
	)
	last := ""
	if m.last != "" {
		last = "\n\nLast: " + m.last
	}
	errLine := ""
	if m.err != "" {
		errLine = "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Error:\n"+m.err)
	}
	footer := "\n\nEsc/Enter to go back"
	return m.style.Render(header + stats + last + errLine + footer)
}

func (m *batchRunModel) runBatchCmd() tea.Cmd {
	return func() tea.Msg {
		files, err := media.ListMediaFiles(m.dir)
		if err != nil {
			return batchStatusMsg{err: err}
		}
		okCount, failCount, skippedCount := 0, 0, 0
		last := ""

		for _, f := range files {
			if m.ctx.Err() != nil {
				last = "cancelled"
				break
			}
			inPath := filepath.Join(m.dir, f)
			ext := strings.ToLower(filepath.Ext(inPath))
			isGif := ext == ".gif"

			hasAudio := false
			if !isGif {
				hasAudio, _ = m.cfg.Prober.HasAudio(m.ctx, inPath)
			}

			if (isGif || !hasAudio) && (m.format == "mp3" || m.format == "flac" || m.format == "wav") {
				skippedCount++
				last = "skipped " + f + " (no audio)"
				continue
			}

			outName := ffx.BatchOutputName(inPath, m.format)
			outPath := filepath.Join(m.dir, outName)

			if m.format == "gif" {
				palette := filepath.Join(m.dir, "palette_"+strings.TrimSuffix(f, ext)+".png")
				if _, _, err := m.cfg.Runner.Run(m.ctx, execx.Cmd{Name: "ffmpeg", Args: ffx.BuildGifPaletteCmd(inPath, palette).Args}); err != nil {
					failCount++
					last = "failed palette for " + f
					continue
				}
				if _, _, err := m.cfg.Runner.Run(m.ctx, execx.Cmd{Name: "ffmpeg", Args: ffx.BuildGifFromPaletteCmd(inPath, palette, outPath).Args}); err != nil {
					failCount++
					last = "failed gif for " + f
					_ = os.Remove(palette)
					continue
				}
				_ = os.Remove(palette)
				okCount++
				last = "converted " + f + " -> " + outName
				continue
			}

			cmd := ffx.BuildBatchConvertCmd(inPath, outPath, m.format, m.quality, hasAudio)
			if _, _, err := m.cfg.Runner.Run(m.ctx, execx.Cmd{Name: "ffmpeg", Args: cmd.Args}); err != nil {
				failCount++
				last = "failed " + f
				continue
			}
			okCount++
			last = "converted " + f + " -> " + outName
		}
		return batchStatusMsg{ok: okCount, fail: failCount, skipped: skippedCount, last: last, err: nil}
	}
}
