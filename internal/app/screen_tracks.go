package app

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fsx8/ffwiz/internal/execx"
	ffx "github.com/fsx8/ffwiz/internal/ffmpeg"
	"github.com/fsx8/ffwiz/internal/ffprobe"
)

type tracksDoneMsg struct {
	tracks []trackView
	err    error
}

type trackView struct {
	Track      ffx.Track
	Width      int
	Height     int
	FPS        string
	SampleRate string
	Channels   int
	Language   string
	Title      string
}

type codecChoice struct {
	label string
}

func (c codecChoice) Title() string       { return c.label }
func (c codecChoice) Description() string { return "" }
func (c codecChoice) FilterValue() string { return c.label }

type tracksWizard struct {
	cfg      Config
	filePath string

	loading bool
	spin    spinner.Model
	err     string

	tracks  []trackView
	actions map[int]ffx.TrackActionInfo
	cursor  int

	step string // "tracks" | "codec" | "output" | "confirm"

	codecList list.Model
	out       textinput.Model

	style lipgloss.Style
	w, h  int
}

func newTracksWizard(cfg Config, filePath string) *tracksWizard {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	out := textinput.New()
	out.Prompt = "> "
	out.CharLimit = 4096
	out.Width = 50

	return &tracksWizard{
		cfg:      cfg,
		filePath: filePath,
		loading:  true,
		spin:     sp,
		actions:  map[int]ffx.TrackActionInfo{},
		step:     "tracks",
		out:      out,
		style:    lipgloss.NewStyle().Padding(1, 2),
	}
}

func (m *tracksWizard) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, func() tea.Msg {
		res, err := m.cfg.Prober.Probe(context.Background(), m.filePath)
		if err != nil {
			return tracksDoneMsg{err: err}
		}
		tvs := tracksFromProbe(res)
		if len(tvs) == 0 {
			return tracksDoneMsg{err: errNoTracks{}}
		}
		return tracksDoneMsg{tracks: tvs}
	})
}

type errNoTracks struct{}

func (errNoTracks) Error() string { return "no tracks found in media file" }

func (m *tracksWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		if m.step == "codec" {
			m.codecList.SetSize(msg.Width-4, msg.Height-6)
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "esc":
			switch m.step {
			case "codec", "output", "confirm":
				m.step = "tracks"
				m.codecList = list.Model{}
				m.out.Blur()
				return m, nil
			default:
				return m, pop()
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
	case tracksDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.tracks = msg.tracks
		for i := range m.tracks {
			m.actions[i] = ffx.TrackActionInfo{Action: ffx.ActionKeep}
		}
		return m, nil
	}

	if m.loading {
		return m, nil
	}

	switch m.step {
	case "tracks":
		return m.updateTracksStep(msg)
	case "codec":
		var cmd tea.Cmd
		m.codecList, cmd = m.codecList.Update(msg)
		if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
			if ch, ok := m.codecList.SelectedItem().(codecChoice); ok {
				m.actions[m.cursor] = ffx.TrackActionInfo{Action: ffx.ActionConvert, Codec: ch.label}
				m.step = "tracks"
				return m, nil
			}
		}
		return m, cmd
	case "output":
		if km, ok := msg.(tea.KeyMsg); ok && km.String() == "enter" {
			outName := strings.TrimSpace(m.out.Value())
			if outName == "" {
				m.err = "output file name is required"
				return m, nil
			}
			m.err = ""
			m.step = "confirm"
			return m, nil
		}
		var cmd tea.Cmd
		m.out, cmd = m.out.Update(msg)
		return m, cmd
	case "confirm":
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "y", "enter":
				outPath := m.outputPath()
				tracks := make([]ffx.Track, 0, len(m.tracks))
				for _, tv := range m.tracks {
					tracks = append(tracks, tv.Track)
				}
				cmd := ffx.BuildInteractiveConvertCmd(m.filePath, outPath, tracks, m.actions)
				if cmd == nil {
					m.err = "all tracks are removed; nothing to output"
					m.step = "tracks"
					return m, nil
				}
				return m, push(newExecScreen(m.cfg, "Converting…", execx.Cmd{Name: "ffmpeg", Args: cmd.Args}))
			case "n":
				m.step = "tracks"
				return m, nil
			case "esc":
				m.step = "tracks"
				return m, nil
			}
		}
		return m, nil
	}

	return m, nil
}

func (m *tracksWizard) updateTracksStep(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down":
			if m.cursor < len(m.tracks)-1 {
				m.cursor++
			}
			return m, nil
		case "r":
			m.actions[m.cursor] = ffx.TrackActionInfo{Action: ffx.ActionRemove}
			return m, nil
		case "k":
			m.actions[m.cursor] = ffx.TrackActionInfo{Action: ffx.ActionKeep}
			return m, nil
		case "c":
			tt := m.tracks[m.cursor].Track.Type
			opts := ffx.CodecOptions(tt)
			items := make([]list.Item, 0, len(opts))
			for _, o := range opts {
				items = append(items, codecChoice{label: o})
			}
			l := list.New(items, list.NewDefaultDelegate(), 0, 0)
			l.Title = "Select codec (" + string(tt) + ")"
			l.SetFilteringEnabled(false)
			l.DisableQuitKeybindings()
			if m.w > 0 && m.h > 0 {
				l.SetSize(m.w-4, m.h-6)
			}
			m.codecList = l
			m.step = "codec"
			return m, nil
		case "enter":
			m.out.SetValue(defaultModifiedName(m.filePath))
			m.out.CursorEnd()
			m.out.Focus()
			m.err = ""
			m.step = "output"
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m *tracksWizard) View() string {
	if m.loading {
		return m.style.Render(m.spin.View() + " Analyzing tracks…\n\nEsc to go back")
	}
	if m.err != "" && m.step == "tracks" {
		// Show errors in tracks view, but allow continuing.
	}
	if m.err != "" && m.step != "tracks" {
		// Keep showing within step views.
	}

	switch m.step {
	case "codec":
		return m.style.Render(m.codecList.View() + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Enter to select • Esc to go back"))
	case "output":
		errLine := ""
		if m.err != "" {
			errLine = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.err)
		}
		return m.style.Render("Output file:\n\n" + m.out.View() + errLine + "\n\nEnter to continue • Esc to go back")
	case "confirm":
		cmdStr := m.previewCommand()
		errLine := ""
		if m.err != "" {
			errLine = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.err)
		}
		return m.style.Render("Generated command:\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(cmdStr) + errLine + "\n\nEnter/Y to run • N to cancel • Esc to go back")
	default:
		return m.style.Render(m.tracksView())
	}
}

func (m *tracksWizard) tracksView() string {
	title := lipgloss.NewStyle().Bold(true).Render("Interactive Modify Tracks") + "\n\n"
	help := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("↑/↓ navigate • r remove • k keep • c convert • Enter continue • Esc back")
	errLine := ""
	if m.err != "" {
		errLine = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.err)
	}

	var sb strings.Builder
	sb.WriteString(title)
	for i, tv := range m.tracks {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		action := m.actions[i].Action
		actionStr := strings.ToUpper(string(action))
		if action == ffx.ActionConvert {
			actionStr += ":" + ffx.CleanCodecChoice(m.actions[i].Codec)
		}
		line := prefix + trackLine(i, tv) + "  [" + actionStr + "]"
		if i == m.cursor {
			line = lipgloss.NewStyle().Bold(true).Render(line)
		}
		sb.WriteString(line + "\n")
	}
	sb.WriteString("\n" + help + errLine)
	return sb.String()
}

func tracksFromProbe(res *ffprobe.ProbeResult) []trackView {
	var out []trackView
	if res == nil {
		return nil
	}
	for _, s := range res.Streams {
		switch s.CodecType {
		case "video", "audio", "subtitle":
		default:
			continue
		}
		tv := trackView{
			Track: ffx.Track{
				Index: s.Index,
				Type:  ffx.TrackType(s.CodecType),
				Codec: s.CodecName,
			},
			Width:      s.Width,
			Height:     s.Height,
			FPS:        s.RFrameRate,
			SampleRate: s.SampleRate,
			Channels:   s.Channels,
			Language:   s.Tags["language"],
			Title:      s.Tags["title"],
		}
		out = append(out, tv)
	}
	return out
}

func trackLine(i int, tv trackView) string {
	switch tv.Track.Type {
	case ffx.TrackVideo:
		return "[" + strconv.Itoa(i) + "] VIDEO  idx=" + strconv.Itoa(tv.Track.Index) + "  " + tv.Track.Codec + "  " + strconv.Itoa(tv.Width) + "x" + strconv.Itoa(tv.Height) + "  " + tv.FPS
	case ffx.TrackAudio:
		lang := tv.Language
		if lang == "" {
			lang = "und"
		}
		return "[" + strconv.Itoa(i) + "] AUDIO  idx=" + strconv.Itoa(tv.Track.Index) + "  " + tv.Track.Codec + "  " + tv.SampleRate + "Hz  " + strconv.Itoa(tv.Channels) + "ch  " + lang
	case ffx.TrackSubtitle:
		lang := tv.Language
		if lang == "" {
			lang = "und"
		}
		title := tv.Title
		if title != "" {
			title = "  " + title
		}
		return "[" + strconv.Itoa(i) + "] SUBS   idx=" + strconv.Itoa(tv.Track.Index) + "  " + tv.Track.Codec + "  " + lang + title
	default:
		return "[" + strconv.Itoa(i) + "] " + string(tv.Track.Type) + " idx=" + strconv.Itoa(tv.Track.Index)
	}
}

func defaultModifiedName(filePath string) string {
	ext := filepath.Ext(filePath)
	base := strings.TrimSuffix(filepath.Base(filePath), ext)
	return base + "_modified" + ext
}

func (m *tracksWizard) outputPath() string {
	outName := strings.TrimSpace(m.out.Value())
	if outName == "" {
		outName = defaultModifiedName(m.filePath)
	}
	if filepath.IsAbs(outName) {
		return outName
	}
	return filepath.Join(filepath.Dir(m.filePath), outName)
}

func (m *tracksWizard) previewCommand() string {
	tracks := make([]ffx.Track, 0, len(m.tracks))
	for _, tv := range m.tracks {
		tracks = append(tracks, tv.Track)
	}
	cmd := ffx.BuildInteractiveConvertCmd(m.filePath, m.outputPath(), tracks, m.actions)
	if cmd == nil {
		return "ffmpeg (no output; all tracks removed)"
	}
	return "ffmpeg " + strings.Join(cmd.Args, " ")
}
