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
	step    string // "format" | "quality"
	format  string
	quality ffx.BatchVideoQuality

	list  list.Model
	style lipgloss.Style
}

var batchFormats = []list.Item{
	formatItem{"mp4"}, formatItem{"mkv"}, formatItem{"mov"}, formatItem{"avi"}, formatItem{"webm"},
	formatItem{"mp3"}, formatItem{"flac"}, formatItem{"wav"},
	formatItem{"gif"},
}

func newBatchWizard(cfg Config, dir string) *batchWizard {
	l := list.New(batchFormats, list.NewDefaultDelegate(), 0, 0)
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
				// Audio targets don't need a quality choice; run directly.
				// Returning from the run screen lands back on format selection.
				m.step = "format"
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
				// Returning from the run screen lands back on format selection.
				m.step = "format"
				m.setFormatList()
				return m, push(newBatchRun(m.cfg, m.dir, m.format, m.quality))
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *batchWizard) View() string {
	return m.style.Render(m.list.View() + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Enter to select • Esc to go back • q to quit"))
}

func (m *batchWizard) setFormatList() {
	m.list.Title = "Batch convert: select output format"
	m.list.SetItems(batchFormats)
}

func (m *batchWizard) setQualityList() {
	items := []list.Item{
		formatItem{"High (CRF 18)"},
		formatItem{"Medium (CRF 23)"},
		formatItem{"Low (CRF 28)"},
	}
	if m.format != "webm" {
		// webm cannot hold typical H.264/AAC sources, so a stream copy
		// would fail; hide the option (see BuildBatchConvertCmd).
		items = append([]list.Item{formatItem{"Same as source"}}, items...)
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
	ok        int
	fail      int
	skipped   int
	last      string
	err       error
	cancelled bool
}

type batchRunModel struct {
	cfg     Config
	dir     string
	format  string
	quality ffx.BatchVideoQuality

	ctx    context.Context
	cancel context.CancelFunc

	spin       spinner.Model
	running    bool
	cancelling bool

	ok        int
	fail      int
	skipped   int
	last      string
	err       string
	cancelled bool

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
				m.cancelling = true
			}
			return m, tea.Quit
		case "ctrl+c":
			if m.running && m.cancel != nil {
				m.cancel()
				m.cancelling = true
			}
			return m, nil
		case "esc":
			if m.running {
				if m.cancel != nil {
					m.cancel()
					m.cancelling = true
				}
				return m, nil
			}
			return m, pop()
		case "enter":
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
		m.cancelled = msg.cancelled
		if msg.err != nil {
			m.err = msg.err.Error()
		}
		return m, nil
	}
	return m, nil
}

func (m *batchRunModel) View() string {
	header := lipgloss.NewStyle().Bold(true).Render("Batch conversion") + "\n"
	switch {
	case m.running && m.cancelling:
		header += m.spin.View() + " Cancelling…\n"
	case m.running:
		header += m.spin.View() + " Running… (Esc to cancel)\n"
	case m.cancelled:
		header += lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("Cancelled.\n")
	default:
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

func (m *batchRunModel) logf(format string, args ...any) {
	if m.cfg.Logger != nil {
		m.cfg.Logger.Printf(format, args...)
	}
}

// runBatchCmd converts every media file in the directory. Files that cannot
// have the target format (e.g. audio-only files for video targets) are
// skipped; failures are logged with the ffmpeg stderr for diagnosis.
func (m *batchRunModel) runBatchCmd() tea.Cmd {
	cfg := m.cfg
	dir := m.dir
	format := m.format
	quality := m.quality
	return func() tea.Msg {
		files, err := media.ListMediaFiles(dir)
		if err != nil {
			return batchStatusMsg{err: err}
		}
		okCount, failCount, skippedCount := 0, 0, 0
		last := ""
		cancelled := false

		for _, f := range files {
			if m.ctx.Err() != nil {
				cancelled = true
				last = "cancelled"
				break
			}
			inPath := filepath.Join(dir, f)

			res, err := cfg.Prober.Probe(m.ctx, inPath)
			if err != nil {
				failCount++
				last = "probe failed " + f
				m.logf("batch: probe failed for %s: %v", inPath, err)
				continue
			}
			hasAudio, hasVideo := false, false
			for _, s := range res.Streams {
				switch s.CodecType {
				case "audio":
					hasAudio = true
				case "video":
					hasVideo = true
				}
			}

			if isAudioFormat(format) {
				if !hasAudio {
					skippedCount++
					last = "skipped " + f + " (no audio)"
					continue
				}
			} else if !hasVideo {
				skippedCount++
				last = "skipped " + f + " (no video)"
				continue
			}

			outName := ffx.BatchOutputName(inPath, format)
			outPath := filepath.Join(dir, outName)

			if format == "gif" {
				palette := filepath.Join(dir, "palette_"+strings.TrimSuffix(f, filepath.Ext(f))+".png")
				if _, stderr, err := cfg.Runner.Run(m.ctx, execx.Cmd{Name: "ffmpeg", Args: ffx.BuildGifPaletteCmd(inPath, palette).Args}); err != nil {
					failCount++
					last = "failed palette for " + f
					m.logf("batch: palette generation failed for %s: %v\n%s", inPath, err, stderr)
					continue
				}
				if _, stderr, err := cfg.Runner.Run(m.ctx, execx.Cmd{Name: "ffmpeg", Args: ffx.BuildGifFromPaletteCmd(inPath, palette, outPath).Args}); err != nil {
					failCount++
					last = "failed gif for " + f
					_ = os.Remove(palette)
					m.logf("batch: gif conversion failed for %s: %v\n%s", inPath, err, stderr)
					continue
				}
				_ = os.Remove(palette)
				okCount++
				last = "converted " + f + " -> " + outName
				continue
			}

			cmd := ffx.BuildBatchConvertCmd(inPath, outPath, format, quality, hasAudio)
			if _, stderr, err := cfg.Runner.Run(m.ctx, execx.Cmd{Name: "ffmpeg", Args: cmd.Args}); err != nil {
				failCount++
				last = "failed " + f
				m.logf("batch: conversion failed for %s: %v\n%s", inPath, err, stderr)
				if m.ctx.Err() != nil {
					cancelled = true
					last = "cancelled"
					break
				}
				continue
			}
			okCount++
			last = "converted " + f + " -> " + outName
		}
		return batchStatusMsg{ok: okCount, fail: failCount, skipped: skippedCount, last: last, cancelled: cancelled}
	}
}

func isAudioFormat(format string) bool {
	switch format {
	case "mp3", "flac", "wav":
		return true
	default:
		return false
	}
}
