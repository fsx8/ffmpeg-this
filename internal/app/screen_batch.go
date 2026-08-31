package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fsx8/ffwiz/internal/execx"
	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
	"github.com/fsx8/ffwiz/internal/ffprobe"
	"github.com/fsx8/ffwiz/internal/media"
)

type batchWizard struct {
	cfg      Config
	dir      string
	startDir string
	step     string // "dir" | "format" | "quality"
	format   string
	quality  ffx.BatchVideoQuality

	list  list.Model
	style lipgloss.Style

	listErr string // directory read failure, shown on the dir step

	listW, listH int // kept across refreshes so navigation preserves the size
}

var batchFormats = []list.Item{
	simpleItem{value: "mp4"}, simpleItem{value: "mkv"}, simpleItem{value: "mov"}, simpleItem{value: "avi"}, simpleItem{value: "webm"},
	simpleItem{value: "mp3"}, simpleItem{value: "flac"}, simpleItem{value: "wav"},
	simpleItem{value: "gif"},
}

func newBatchWizard(cfg Config, dir string) *batchWizard {
	m := &batchWizard{
		cfg:      cfg,
		dir:      dir,
		startDir: dir,
		step:     "dir",
		quality:  ffx.QualityMedium,
		style:    lipgloss.NewStyle().Padding(1, 2),
	}
	m.refreshDirs()
	return m
}

// refreshDirs rebuilds the directory browser shown before choosing the
// output format. Errors are surfaced and cleared again by the next
// successful refresh.
func (m *batchWizard) refreshDirs() {
	_, dirs, err := media.ListDir(m.dir)
	var items []list.Item
	if err != nil {
		m.errList(err)
		return
	}
	m.listErr = ""
	if parent := filepath.Dir(m.dir); parent != m.dir {
		items = append(items, dirItem{name: "..", path: parent})
	}
	for _, d := range dirs {
		items = append(items, dirItem{name: d, path: filepath.Join(m.dir, d)})
	}
	items = append(items, simpleItem{value: "Convert files in this directory"})

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Batch convert: choose directory"
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	if m.listW > 0 {
		l.SetSize(m.listW, m.listH)
	}
	m.list = l
}

// errList shows a bare error list when the directory cannot be read.
func (m *batchWizard) errList(err error) {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Batch convert: cannot read " + m.dir
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()
	m.list = l
	m.step = "dir"
	m.listErr = err.Error()
}

func (m *batchWizard) Init() tea.Cmd { return nil }

func (m *batchWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.listW, m.listH = dim(msg.Width, 4), dim(msg.Height, 6)
		m.list.SetSize(m.listW, m.listH)
	case tea.KeyMsg:
		filtering := m.step == "dir" && filterActive(m.list)
		switch msg.String() {
		case "q":
			if filtering {
				break
			}
			return m, tea.Quit
		case "esc":
			if filtering {
				break // let the list clear/leave its filter
			}
			switch m.step {
			case "dir":
				if filterApplied(m.list) {
					m.list.ResetFilter()
					return m, nil
				}
				// While browsing, Esc walks back up before leaving.
				if m.dir != m.startDir {
					m.dir = filepath.Dir(m.dir)
					m.refreshDirs()
					return m, nil
				}
				return m, pop()
			case "quality":
				m.step = "format"
				m.setFormatList()
				return m, nil
			default: // "format"
				m.step = "dir"
				m.refreshDirs()
				return m, nil
			}
		case "enter":
			if filtering {
				break // let the list apply the filter
			}
			switch m.step {
			case "dir":
				switch it := m.list.SelectedItem().(type) {
				case dirItem:
					m.dir = it.path
					m.refreshDirs()
					return m, nil
				case simpleItem:
					if it.value == "Convert files in this directory" {
						m.step = "format"
						m.setFormatList()
						return m, nil
					}
				}
				return m, nil
			case "format":
				fi, ok := m.list.SelectedItem().(simpleItem)
				if !ok {
					return m, nil
				}
				m.format = fi.value
				if isVideoFormat(m.format) {
					m.step = "quality"
					m.setQualityList()
					return m, nil
				}
				// Audio targets don't need a quality choice; run directly.
				// Returning from the run screen lands on format selection.
				m.step = "format"
				return m, push(newBatchRun(m.cfg, m.dir, m.format, m.quality))
			case "quality":
				fi, ok := m.list.SelectedItem().(simpleItem)
				if !ok {
					return m, nil
				}
				switch fi.value {
				case "Same as source":
					m.quality = ffx.QualitySame
				case "High (CRF 18)":
					m.quality = ffx.QualityHigh
				case "Medium (CRF 23)":
					m.quality = ffx.QualityMedium
				case "Low (CRF 28)":
					m.quality = ffx.QualityLow
				}
				// Returning from the run screen lands on format selection.
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
	errLine := ""
	if m.listErr != "" {
		errLine = renderErrLine(m.listErr)
	}
	return m.style.Render(m.list.View() + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		"Directory: "+m.dir+"\nEnter to select • / to filter • Esc to go back • q to quit") + errLine)
}

func (m *batchWizard) setFormatList() {
	m.list.Title = "Batch convert: select output format"
	m.list.SetItems(batchFormats)
	m.listErr = ""
}

func (m *batchWizard) setQualityList() {
	items := []list.Item{
		simpleItem{value: "High (CRF 18)"},
		simpleItem{value: "Medium (CRF 23)"},
		simpleItem{value: "Low (CRF 28)"},
	}
	if m.format != "webm" {
		// webm cannot hold typical H.264/AAC sources, so a stream copy
		// would fail; hide the option (see BuildBatchConvertCmd).
		items = append([]list.Item{simpleItem{value: "Same as source"}}, items...)
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
	tail      []string // tail of ffmpeg stderr across the run, for the summary
	err       error
	cancelled bool
}

type batchProgMsg struct {
	name  string
	idx   int
	total int
	stage string
	pct   float64
	speed float64
}

// isOwnBatchOutput reports whether name looks like a file this tool's batch
// conversion produced for the target format (foo.mp4 -> foo_batch.mp4).
// Skipping them keeps repeated runs from chaining foo_batch_batch.mp4,
// foo_batch_batch_batch.mp4, …
func isOwnBatchOutput(name, format string) bool {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	if ext != format {
		return false
	}
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	return strings.HasSuffix(strings.ToLower(stem), "_batch")
}

type batchRunModel struct {
	cfg     Config
	dir     string
	format  string
	quality ffx.BatchVideoQuality

	ctx    context.Context
	cancel context.CancelFunc

	spin       spinner.Model
	started    bool // pre-flight confirmed; the run loop is live
	running    bool
	cancelling bool

	overwrites int // existing outputs that this run will replace
	fileCount  int // media files found in the directory

	ok        int
	fail      int
	skipped   int
	last      string
	tail      []string
	err       string
	cancelled bool

	progCh     chan batchProgMsg
	width      int
	curName    string
	curStage   string
	curPct     float64
	curSpeed   float64
	curIdx     int
	totalFiles int

	style lipgloss.Style
}

func newBatchRun(cfg Config, dir, format string, quality ffx.BatchVideoQuality) *batchRunModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	ctx, cancel := context.WithCancel(context.Background())
	count, _ := media.ListMediaFiles(dir)
	return &batchRunModel{
		cfg:        cfg,
		dir:        dir,
		format:     format,
		quality:    quality,
		ctx:        ctx,
		cancel:     cancel,
		spin:       sp,
		fileCount:  len(count),
		progCh:     make(chan batchProgMsg, 64),
		overwrites: countExistingOutputs(dir, format),
		style:      lipgloss.NewStyle().Padding(1, 2),
	}
}

// countExistingOutputs reports how many of the batch outputs the run is
// about to create already exist (they will be overwritten with -y).
func countExistingOutputs(dir, format string) int {
	files, err := media.ListMediaFiles(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, f := range files {
		if isOwnBatchOutput(f, format) {
			continue
		}
		out := ffx.BatchOutputName(f, format)
		if outputExists(filepath.Join(dir, out)) {
			n++
		}
	}
	return n
}

func (m *batchRunModel) Init() tea.Cmd {
	// The run only starts after the user confirms the pre-flight summary
	// with Enter (it may overwrite existing outputs).
	return nil
}

// start transitions from the pre-flight summary into the live run.
func (m *batchRunModel) start() tea.Cmd {
	m.started = true
	m.running = true
	return tea.Batch(m.spin.Tick, pollBatchProgress(m.progCh), m.runBatchCmd())
}

func pollBatchProgress(ch <-chan batchProgMsg) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return nil
		}
		return p
	}
}

func (m *batchRunModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		if !m.started {
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "esc":
				return m, pop()
			case "enter":
				return m, m.start()
			}
			return m, nil
		}
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
	case batchProgMsg:
		if msg.name != "" {
			m.curName = msg.name
		}
		m.curStage = msg.stage
		m.curPct = msg.pct
		m.curSpeed = msg.speed
		if msg.total > 0 {
			m.totalFiles = msg.total
		}
		if msg.idx > 0 {
			m.curIdx = msg.idx
		}
		return m, pollBatchProgress(m.progCh)
	case batchStatusMsg:
		close(m.progCh)
		m.running = false
		m.ok = msg.ok
		m.fail = msg.fail
		m.skipped = msg.skipped
		m.last = msg.last
		m.tail = msg.tail
		m.cancelled = msg.cancelled
		if msg.err != nil {
			m.err = msg.err.Error()
		}
		return m, nil
	}
	return m, nil
}

func (m *batchRunModel) View() string {
	if !m.started {
		return m.style.Render(m.preflightView())
	}

	header := lipgloss.NewStyle().Bold(true).Render("Batch conversion") + "\n"
	switch {
	case m.running && m.cancelling:
		header += m.spin.View() + " Cancelling…\n"
	case m.running:
		header += m.spin.View() + " Running… (Esc to cancel)\n"
	case m.cancelled:
		header += lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("Cancelled.\n")
	case m.fail > 0:
		header += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(
			fmt.Sprintf("Done with %d failure(s).\n", m.fail))
	default:
		header += lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("Done.\n")
	}
	if m.running && m.overwrites > 0 {
		header += lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(
			fmt.Sprintf("%d output file(s) already exist and will be overwritten.\n", m.overwrites))
	}

	body := ""
	// The file line only appears once a file has actually started, so a
	// run waiting for its first progress message shows no "File 0/N".
	if m.running && m.totalFiles > 0 && m.curIdx >= 1 {
		muted := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		fileLine := fmt.Sprintf("File %d/%d: %s", m.curIdx, m.totalFiles, m.curName)
		body += "\n" + fileLine + "\n"
		if m.curName == "" {
			body += muted.Render("starting…") + "\n"
		} else {
			if m.curStage != "" {
				body += m.spin.View() + " " + m.curStage + "…\n"
			}
			if m.curPct > 0 {
				bar := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(
					renderBar(progressWidth(dim(m.width, 8)), m.curPct))
				line := fmt.Sprintf("%3.0f%%", m.curPct*100)
				if m.curSpeed > 0 {
					line += fmt.Sprintf("  %.1fx", m.curSpeed)
				}
				body += bar + muted.Render(line) + "\n"
			}
		}
	}

	stats := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		fmt.Sprintf("\nOK: %d  Failed: %d  Skipped: %d", m.ok, m.fail, m.skipped),
	)
	last := ""
	if m.last != "" {
		last = "\n\nLast: " + m.last
	}
	errLine := ""
	if m.err != "" {
		errLine = "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Error:\n"+m.err)
	}
	footer := "\n\nEsc/Enter to go back • q to quit"
	return m.style.Render(header + body + stats + last + errLine + m.tailView() + footer)
}

// preflightView summarizes what the run will do before anything starts, so
// overwriting existing outputs is a deliberate act (all commands run -y).
func (m *batchRunModel) preflightView() string {
	quality := ""
	switch m.quality {
	case ffx.QualitySame:
		quality = "Same as source (stream copy)"
	case ffx.QualityHigh:
		quality = "High (CRF 18)"
	case ffx.QualityMedium:
		quality = "Medium (CRF 23)"
	case ffx.QualityLow:
		quality = "Low (CRF 28)"
	}
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("Batch conversion") + "\n\n")
	sb.WriteString("Directory: " + m.dir + "\n")
	sb.WriteString("Format: " + m.format + "\n")
	if quality != "" {
		sb.WriteString("Quality: " + quality + "\n")
	}
	sb.WriteString(fmt.Sprintf("Media files found: %d\n", m.fileCount))
	if m.overwrites > 0 {
		sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(
			fmt.Sprintf("%d output file(s) already exist and will be overwritten.", m.overwrites)) + "\n")
	}
	sb.WriteString("\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		"Enter to start • Esc to go back • q to quit"))
	return sb.String()
}

// tailView renders the last ffmpeg stderr lines of a finished run so
// failures can be diagnosed without opening ffmpeg_log.txt.
func (m *batchRunModel) tailView() string {
	const maxLines = 8
	if m.running || len(m.tail) == 0 {
		return ""
	}
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	shown := m.tail
	if len(shown) > maxLines {
		shown = shown[len(shown)-maxLines:]
	}
	out := "\n\n" + muted.Render("ffmpeg output (tail):")
	for _, l := range shown {
		out += "\n" + muted.Render("  "+l)
	}
	return out + "\n" + muted.Render("Full log: ffmpeg_log.txt")
}

func (m *batchRunModel) logf(format string, args ...any) {
	if m.cfg.Logger != nil {
		m.cfg.Logger.Printf(format, args...)
	}
}

// skipsConversion reports whether a file whose extension already matches
// the target format should be left alone. Re-encoding only makes sense when
// the user explicitly picked a quality preset for a video target.
func skipsConversion(format string, quality ffx.BatchVideoQuality) bool {
	return quality == ffx.QualitySame || isAudioFormat(format) || format == "gif"
}

// runBatchCmd converts every media file in the directory. Files that cannot
// have the target format (e.g. audio-only files for video targets), that
// already have it (without an explicit quality preset), or that are the
// tool's own earlier batch outputs are skipped; failures are logged with
// the ffmpeg stderr for diagnosis.
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
		sendProg := func(p batchProgMsg) {
			select {
			case m.progCh <- p:
			default:
			}
		}

		okCount, failCount, skippedCount := 0, 0, 0
		last := ""
		var tail []string
		addTail := func(line string) {
			if line == "" || len(tail) >= 400 {
				return
			}
			tail = append(tail, line)
		}
		cancelled := false
		total := len(files)
		if total > 0 {
			sendProg(batchProgMsg{name: "", idx: 0, total: total})
		}

		for i, f := range files {
			if m.ctx.Err() != nil {
				cancelled = true
				last = "cancelled"
				break
			}
			inPath := filepath.Join(dir, f)

			// Never re-convert the tool's own outputs.
			if isOwnBatchOutput(f, format) {
				skippedCount++
				last = "skipped " + f + " (previous batch output)"
				continue
			}

			if strings.TrimPrefix(strings.ToLower(filepath.Ext(f)), ".") == format && skipsConversion(format, quality) {
				skippedCount++
				last = "skipped " + f + " (already " + format + ")"
				continue
			}

			// A missing prober fails the file like an unreadable one
			// instead of panicking mid-run.
			var res *ffprobe.ProbeResult
			var perr error
			if cfg.Prober == nil {
				perr = errors.New("ffprobe unavailable")
			} else {
				res, perr = cfg.Prober.Probe(m.ctx, inPath)
			}
			if perr != nil {
				failCount++
				last = "probe failed " + f
				addTail("probe failed: " + f)
				m.logf("batch: probe failed for %s: %v", inPath, perr)
				continue
			}
			hasAudio, hasVideo := res.HasAudio(), res.HasVideo()

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

			fileDur := time.Duration(0)
			if d, ok := res.Duration(); ok {
				fileDur = time.Duration(d * float64(time.Second))
			}

			stderrLog := func(line string) {
				addTail(line)
				if line != "" && m.cfg.Logger != nil {
					m.cfg.Logger.Printf("%s", line)
				}
			}
			newTracker := func() (*ffx.ProgressTracker, func(line string)) {
				tr := &ffx.ProgressTracker{}
				return tr, func(line string) {
					if s, ok := tr.Observe(line); ok {
						sendProg(batchProgMsg{name: f, idx: i + 1, pct: s.Percent(fileDur), speed: s.Speed})
					}
				}
			}

			if format == "gif" {
				palette := filepath.Join(dir, "palette_"+strings.TrimSuffix(f, filepath.Ext(f))+".png")
				removePalette := func() { _ = os.Remove(palette) }
				sendProg(batchProgMsg{name: f, idx: i + 1, stage: "palette"})
				_, onProg := newTracker()
				args := ffx.AddProgressArgs(ffx.BuildGifPaletteCmd(inPath, palette).Args)
				if _, err := cfg.Runner.RunStreaming(m.ctx, execx.Cmd{Name: "ffmpeg", Args: args}, onProg, stderrLog); err != nil {
					removePalette()
					failCount++
					last = "failed palette for " + f
					m.logf("batch: palette generation failed for %s: %v", inPath, err)
					if m.ctx.Err() != nil {
						cancelled = true
						last = "cancelled"
						break
					}
					continue
				}
				_, onProg = newTracker()
				args = ffx.AddProgressArgs(ffx.BuildGifFromPaletteCmd(inPath, palette, outPath).Args)
				if _, err := cfg.Runner.RunStreaming(m.ctx, execx.Cmd{Name: "ffmpeg", Args: args}, onProg, stderrLog); err != nil {
					removePalette()
					failCount++
					last = "failed gif for " + f
					m.logf("batch: gif conversion failed for %s: %v", inPath, err)
					if m.ctx.Err() != nil {
						cancelled = true
						last = "cancelled"
						break
					}
					continue
				}
				removePalette()
				okCount++
				last = "converted " + f + " -> " + outName
				continue
			}

			// The copy preset adapts per target container, which needs the
			// source stream layout the probe just delivered.
			var info ffx.BatchStreamInfo
			for _, s := range res.StreamsOfType("audio") {
				info.AudioCodecs = append(info.AudioCodecs, s.CodecName)
			}
			for _, s := range res.StreamsOfType("subtitle") {
				info.SubtitleCodecs = append(info.SubtitleCodecs, s.CodecName)
			}
			cmd := ffx.BuildBatchConvertCmd(inPath, outPath, format, quality, hasAudio, info)
			_, onProg := newTracker()
			args := ffx.AddProgressArgs(cmd.Args)
			if _, err := cfg.Runner.RunStreaming(m.ctx, execx.Cmd{Name: "ffmpeg", Args: args}, onProg, stderrLog); err != nil {
				failCount++
				last = "failed " + f
				m.logf("batch: conversion failed for %s: %v", inPath, err)
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
		return batchStatusMsg{ok: okCount, fail: failCount, skipped: skippedCount, last: last, tail: tail, cancelled: cancelled}
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
