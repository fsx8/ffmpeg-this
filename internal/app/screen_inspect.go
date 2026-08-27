package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fsx8/ffwiz/internal/ffprobe"
)

type inspectDoneMsg struct {
	res *ffprobe.ProbeResult
	err error
}

type inspectModel struct {
	cfg      Config
	filePath string

	loading bool
	spin    spinner.Model
	vp      viewport.Model
	content string
	err     string
	style   lipgloss.Style
}

func newInspectScreen(cfg Config, filePath string) *inspectModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return &inspectModel{
		cfg:      cfg,
		filePath: filePath,
		loading:  true,
		spin:     sp,
		style:    lipgloss.NewStyle().Padding(1, 2),
	}
}

func (m *inspectModel) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, func() tea.Msg {
		res, err := m.cfg.Prober.Probe(context.Background(), m.filePath)
		return inspectDoneMsg{res: res, err: err}
	})
}

func (m *inspectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.vp = viewport.New(msg.Width-4, msg.Height-6)
		m.vp.SetContent(m.content)
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "esc":
			return m, pop()
		}
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
	case inspectDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.content = formatProbe(msg.res)
		m.vp.SetContent(m.content)
		return m, nil
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m *inspectModel) View() string {
	if m.loading {
		return m.style.Render(m.spin.View() + " Inspecting…\n\nEsc to go back")
	}
	if m.err != "" {
		return m.style.Render(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Error:\n"+m.err) + "\n\nEsc to go back")
	}
	return m.style.Render(m.vp.View() + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("↑/↓ to scroll • Esc to go back"))
}

func formatProbe(res *ffprobe.ProbeResult) string {
	if res == nil {
		return "No data."
	}

	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	sizeStr := "unknown"
	if n, err := strconv.ParseFloat(res.Format.Size, 64); err == nil {
		sizeStr = fmt.Sprintf("%0.2f MB", n/(1024*1024))
	}
	durationStr := "unknown"
	if n, err := strconv.ParseFloat(res.Format.Duration, 64); err == nil {
		durationStr = fmt.Sprintf("%0.2f seconds", n)
	}
	bitrateStr := "unknown"
	if n, err := strconv.ParseFloat(res.Format.BitRate, 64); err == nil {
		bitrateStr = fmt.Sprintf("%0.0f kb/s", n/1000)
	}

	var sb strings.Builder
	sb.WriteString(bold.Render("File Information") + "\n")
	sb.WriteString(fmt.Sprintf("%s %s\n", dim.Render("File:"), res.Format.Filename))
	sb.WriteString(fmt.Sprintf("%s %s\n", dim.Render("Size:"), sizeStr))
	sb.WriteString(fmt.Sprintf("%s %s\n", dim.Render("Duration:"), durationStr))
	sb.WriteString(fmt.Sprintf("%s %s\n", dim.Render("Format:"), res.Format.FormatLongName))
	sb.WriteString(fmt.Sprintf("%s %s\n", dim.Render("Bitrate:"), bitrateStr))

	writeStreams := func(kind string) {
		var ss []ffprobe.Stream
		for _, s := range res.Streams {
			if s.CodecType == kind {
				ss = append(ss, s)
			}
		}
		if len(ss) == 0 {
			return
		}
		sb.WriteString("\n" + bold.Render(strings.ToUpper(kind)+" Streams") + "\n")
		for _, s := range ss {
			switch kind {
			case "video":
				sb.WriteString(fmt.Sprintf("#%d  %s  %dx%d  %s\n", s.Index, s.CodecName, s.Width, s.Height, s.RFrameRate))
			case "audio":
				sb.WriteString(fmt.Sprintf("#%d  %s  %s Hz  %dch\n", s.Index, s.CodecName, s.SampleRate, s.Channels))
			default:
				sb.WriteString(fmt.Sprintf("#%d  %s\n", s.Index, s.CodecName))
			}
		}
	}

	writeStreams("video")
	writeStreams("audio")
	writeStreams("subtitle")
	return sb.String()
}
