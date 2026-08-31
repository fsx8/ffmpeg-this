package app

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
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
	order    int // 1-based join position; 0 when unselected
}

func (i *joinItem) Title() string {
	box := "[ ]"
	if i.selected {
		box = "[x]"
	}
	return box + " " + i.name
}
func (i *joinItem) Description() string {
	if i.selected {
		return fmt.Sprintf("joined at position %d", i.order)
	}
	return ""
}
func (i *joinItem) FilterValue() string { return i.name }

type joinProbeMsg struct {
	inputs []ffx.JoinInput
	target ffx.JoinTargets
	fps    []string // distinct video frame rates across the inputs
	err    error
}

type joinWizard struct {
	cfg      Config
	dir      string
	startDir string

	list list.Model
	out  textinput.Model
	spin spinner.Model

	guard       overwriteGuard
	probeCancel context.CancelFunc // cancels an in-flight join probe on Esc

	step string // "select" | "output" | "probing" | "confirm"
	err  string

	inputs []ffx.JoinInput
	target ffx.JoinTargets
	fps    []string

	style lipgloss.Style

	listW, listH int // kept across refreshes so navigation preserves the size
}

func newJoinWizard(cfg Config, dir string) *joinWizard {
	j := &joinWizard{
		cfg:      cfg,
		dir:      dir,
		startDir: dir,
		step:     "select",
		style:    lipgloss.NewStyle().Padding(1, 2),
	}

	j.refreshFiles()

	out := newPathInput()
	out.SetValue("joined_video.mp4")
	j.out = out

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	j.spin = sp
	return j
}

var joinVideoExtensions = map[string]bool{
	".mp4": true, ".mkv": true, ".mov": true, ".avi": true, ".webm": true,
}

// refreshFiles rebuilds the list for m.dir: parent entry, subdirectories,
// then joinable videos. The selection is per directory; navigating resets
// it. Errors are surfaced instead of silently showing an empty list.
func (m *joinWizard) refreshFiles() {
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
			if joinVideoExtensions[strings.ToLower(filepath.Ext(f))] {
				items = append(items, &joinItem{name: f, absPath: filepath.Join(m.dir, f)})
			}
		}
	}
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select videos to join (space toggles)"
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	if m.listW > 0 {
		l.SetSize(m.listW, m.listH)
	}
	m.list = l
}

func (m *joinWizard) Init() tea.Cmd { return nil }

func (m *joinWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.listW, m.listH = dim(msg.Width, 4), dim(msg.Height, 6)
		m.list.SetSize(m.listW, m.listH)
	case spinner.TickMsg:
		if m.step == "probing" {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
	case joinProbeMsg:
		if m.step != "probing" {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err.Error()
			m.step = "select"
			return m, nil
		}
		m.err = ""
		m.inputs = msg.inputs
		m.target = msg.target
		m.fps = msg.fps
		m.step = "confirm"
		return m, nil
	case tea.KeyMsg:
		filtering := m.step == "select" && filterActive(m.list)
		typing := m.step == "output" && textInputFocused(m.out)

		switch msg.String() {
		case "q":
			if filtering || typing {
				break
			}
			if m.step == "probing" && m.probeCancel != nil {
				m.probeCancel()
				m.probeCancel = nil
			}
			return m, tea.Quit
		case "ctrl+c":
			if m.probeCancel != nil {
				m.probeCancel()
				m.probeCancel = nil
			}
			return m, tea.Quit
		case "esc":
			if filtering {
				break // let the list clear/leave its filter
			}
			switch m.step {
			case "confirm":
				m.guard.armedFor = ""
				m.step = "output"
				m.out.Focus()
				return m, textinput.Blink
			case "probing":
				// Abort the in-flight probes instead of letting them run to
				// completion (a stuck network mount would spin forever).
				if m.probeCancel != nil {
					m.probeCancel()
					m.probeCancel = nil
				}
				m.step = "select"
				return m, nil
			case "output":
				m.step = "select"
				m.out.Blur()
				return m, nil
			default:
				if filterApplied(m.list) {
					m.list.ResetFilter()
					return m, nil
				}
				// While browsing, Esc walks back up before leaving.
				if m.dir != m.startDir {
					m.dir = filepath.Dir(m.dir)
					m.refreshFiles()
					return m, nil
				}
				return m, pop()
			}
		case " ":
			if m.step == "select" && !filtering {
				if it, ok := m.list.SelectedItem().(*joinItem); ok {
					it.selected = !it.selected
					m.recomputeJoinOrder()
					m.err = ""
					return m, nil
				}
			}
		case "enter":
			if filtering {
				break // let the list apply the filter
			}
			switch m.step {
			case "select":
				if it, ok := m.list.SelectedItem().(dirItem); ok {
					m.dir = it.path
					m.refreshFiles()
					return m, nil
				}
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
				m.err = ""
				m.out.Blur()
				m.step = "probing"
				return m, tea.Batch(m.spin.Tick, m.probeCmd(m.selectedPaths()))
			case "confirm":
				outPath := m.outputPath()
				// -y is passed to ffmpeg; make overwriting explicit.
				if m.guard.shouldWarn(outPath) {
					return m, nil
				}
				cmd := ffx.BuildJoinCmd(m.inputs, outPath, m.target)
				return m, push(newExecScreen(m.cfg, "Joining videos…", execx.Cmd{Name: "ffmpeg", Args: cmd.Args}))
			}
		}
	}

	if m.step == "output" {
		var cmd tea.Cmd
		m.out, cmd = m.out.Update(msg)
		return m, cmd
	}
	if m.step == "probing" || m.step == "confirm" {
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *joinWizard) View() string {
	switch m.step {
	case "probing":
		return m.style.Render(m.spin.View() + " Analyzing selected videos…\n\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Esc to go back • q to quit"))
	case "confirm":
		return m.style.Render(m.confirmView())
	case "output":
		errLine := renderErrLine(m.err)
		return m.style.Render("Output file:\n\n" + m.out.View() + errLine + "\n\nEnter to continue • Esc to go back • q to quit")
	default:
		info := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
			"Directory: " + m.dir + "\n" +
				"Space toggles • / to filter • Enter to continue • Esc to go back • q to quit\n" +
				"Files are joined in list order (natural sort)")
		errLine := renderErrLine(m.err)
		return m.style.Render(m.list.View() + "\n" + info + errLine)
	}
}

func (m *joinWizard) confirmView() string {
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Join %d videos", len(m.inputs))) + "\n\n")
	for i, in := range m.inputs {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, filepath.Base(in.Path)))
	}
	sb.WriteString(fmt.Sprintf("\nTarget: %dx%d @ %s Hz\n", m.target.Width, m.target.Height, m.target.SampleRate))
	if note := m.fpsNote(); note != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(note) + "\n")
	}

	cmd := ffx.BuildJoinCmd(m.inputs, m.outputPath(), m.target)
	sb.WriteString("\n" + lipgloss.NewStyle().Bold(true).Render("Command:") + "\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(strings.Join(cmd.FullArgs(), " ")) + "\n")

	if p := m.guard.armedFor; p != "" {
		sb.WriteString(renderWarnLine(m.guard))
	}
	sb.WriteString("\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Enter to run • Esc to go back • q to quit"))
	return sb.String()
}

// fpsNote warns about differing frame rates; the concat filter emits a
// variable frame rate track in that case.
func (m *joinWizard) fpsNote() string {
	if len(m.fps) <= 1 {
		return ""
	}
	shown := m.fps
	if len(shown) > 3 {
		shown = append(append([]string{}, shown[:3]...), "…")
	}
	return fmt.Sprintf("Note: inputs use different frame rates (%s); the joined video will have a variable frame rate.",
		strings.Join(shown, ", "))
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

func (m *joinWizard) outputPath() string {
	outName := strings.TrimSpace(m.out.Value())
	if outName == "" {
		outName = "joined_video.mp4"
	}
	return resolveOutputPath(m.dir, outName)
}

// recomputeJoinOrder numbers the selected items by their position in the
// list, which is the order they will be joined in.
func (m *joinWizard) recomputeJoinOrder() {
	n := 0
	for _, it := range m.list.Items() {
		ji, ok := it.(*joinItem)
		if !ok {
			continue
		}
		if ji.selected {
			n++
			ji.order = n
		} else {
			ji.order = 0
		}
	}
}

// probeCmd probes every selected input off the UI thread so slow storage
// cannot freeze the TUI, and derives the join targets from the first input.
// The probe honors probeTimeout and can be cancelled via Esc
// (m.probeCancel).
func (m *joinWizard) probeCmd(selected []string) tea.Cmd {
	cfg := m.cfg
	if cfg.Prober == nil {
		return func() tea.Msg { return joinProbeMsg{err: errors.New("ffprobe unavailable")} }
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	m.probeCancel = cancel
	return func() tea.Msg {
		defer cancel()
		inputs := make([]ffx.JoinInput, 0, len(selected))
		var target ffx.JoinTargets
		var fps []string
		haveResolution := false
		for _, p := range selected {
			res, err := cfg.Prober.Probe(ctx, p)
			if err != nil {
				return joinProbeMsg{err: fmt.Errorf("%s: %w", filepath.Base(p), err)}
			}
			in := ffx.JoinInput{Path: p}
			for _, s := range res.StreamsOfType("audio") {
				in.HasAudio = true
				if target.SampleRate == "" {
					target.SampleRate = s.SampleRate
				}
			}
			for _, s := range res.VideoStreams() {
				// Equal rates spelled differently ("24/1" vs
				// "24000/1000") must dedupe, so compare numerically.
				if fpsDistinct(fps, s.RFrameRate) {
					fps = append(fps, s.RFrameRate)
				}
				if !haveResolution {
					target.Width, target.Height = s.Width, s.Height
					haveResolution = true
					if s.SampleAspectRatio != "" {
						target.SAR = s.SampleAspectRatio
					}
				}
			}
			in.DurationSec, _ = res.Duration()
			inputs = append(inputs, in)
		}
		// Even dimensions up front so the confirm screen shows the same
		// target the command will use (libx264 refuses odd sizes).
		target.Width = ffx.EvenDimension(target.Width)
		target.Height = ffx.EvenDimension(target.Height)
		if !haveResolution || target.Width == 0 || target.Height == 0 {
			return joinProbeMsg{err: errors.New("could not determine target resolution (no video stream found)")}
		}
		if anyHasAudio(inputs) && target.SampleRate == "" {
			return joinProbeMsg{err: errors.New("could not determine target audio sample rate")}
		}
		return joinProbeMsg{inputs: inputs, target: target, fps: fps}
	}
}

func anyHasAudio(inputs []ffx.JoinInput) bool {
	for _, in := range inputs {
		if in.HasAudio {
			return true
		}
	}
	return false
}

// parseFps parses a frame rate given as a rational ("30000/1001") or a
// plain number ("29.97"); ok is false for unparseable input.
func parseFps(s string) (float64, bool) {
	if num, den, ok := strings.Cut(s, "/"); ok {
		n, errN := strconv.ParseFloat(strings.TrimSpace(num), 64)
		d, errD := strconv.ParseFloat(strings.TrimSpace(den), 64)
		if errN != nil || errD != nil || d == 0 {
			return 0, false
		}
		return n / d, true
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// fpsDistinct reports whether rate differs from every collected rate by
// more than a small relative epsilon, so mathematically equal rates
// spelled differently dedupe. Unparseable rates fall back to exact
// string comparison.
func fpsDistinct(known []string, rate string) bool {
	f, ok := parseFps(rate)
	if !ok {
		return !slices.Contains(known, rate)
	}
	for _, k := range known {
		g, ok := parseFps(k)
		if !ok {
			if k == rate {
				return false
			}
			continue
		}
		if math.Abs(f-g) <= 1e-6*math.Max(math.Abs(f), math.Abs(g)) {
			return false
		}
	}
	return true
}
