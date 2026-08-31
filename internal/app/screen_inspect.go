package app

import (
	"context"
	"errors"
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
		if m.cfg.Prober == nil {
			return inspectDoneMsg{err: errors.New("ffprobe unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
		defer cancel()
		res, err := m.cfg.Prober.Probe(ctx, m.filePath)
		return inspectDoneMsg{res: res, err: err}
	})
}

func (m *inspectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.vp = viewport.New(dim(msg.Width, 4), dim(msg.Height, 6))
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
		return m.style.Render(m.spin.View() + " Inspecting…\n\nEsc to go back • q to quit")
	}
	if m.err != "" {
		return m.style.Render(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Error:\n"+m.err) + "\n\nEsc to go back • q to quit")
	}
	return m.style.Render(m.vp.View() + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("↑/↓ to scroll • Esc to go back • q to quit"))
}

// hdrLabel classifies HDR color metadata into a short human-readable label.
func hdrLabel(s ffprobe.Stream) string {
	switch s.ColorTransfer {
	case "smpte2084":
		return "HDR10"
	case "arib-std-b67":
		return "HLG"
	}
	return ""
}

// bitrateLabel renders a per-stream bit rate; empty for unknown values.
func bitrateLabel(bitRate string) string {
	n, err := strconv.ParseFloat(bitRate, 64)
	if err != nil || n <= 0 {
		return ""
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%0.1f Mb/s", n/1_000_000)
	}
	return fmt.Sprintf("%0.0f kb/s", n/1000)
}

func formatProbe(res *ffprobe.ProbeResult) string {
	if res == nil {
		return "No data."
	}

	bold := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

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
	sb.WriteString(fmt.Sprintf("%s %s\n", muted.Render("File:"), res.Format.Filename))
	sb.WriteString(fmt.Sprintf("%s %s\n", muted.Render("Size:"), sizeStr))
	sb.WriteString(fmt.Sprintf("%s %s\n", muted.Render("Duration:"), durationStr))
	sb.WriteString(fmt.Sprintf("%s %s\n", muted.Render("Format:"), res.Format.FormatLongName))
	sb.WriteString(fmt.Sprintf("%s %s\n", muted.Render("Bitrate:"), bitrateStr))

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
			var parts []string
			switch kind {
			case "video":
				parts = []string{
					fmt.Sprintf("#%d", s.Index),
					s.CodecName,
					fmt.Sprintf("%dx%d", s.Width, s.Height),
					s.RFrameRate + " fps",
				}
				if s.PixFmt != "" {
					parts = append(parts, s.PixFmt)
				}
				if s.Profile != "" {
					parts = append(parts, s.Profile)
				}
				if hdr := hdrLabel(s); hdr != "" {
					parts = append(parts, hdr)
				}
			case "audio":
				parts = []string{fmt.Sprintf("#%d", s.Index), s.CodecName}
				if s.SampleRate != "" {
					parts = append(parts, s.SampleRate+" Hz")
				}
				if s.Channels > 0 {
					parts = append(parts, fmt.Sprintf("%dch", s.Channels))
				}
			default:
				parts = []string{fmt.Sprintf("#%d", s.Index), s.CodecName}
			}
			if lang := s.Tags["language"]; lang != "" {
				parts = append(parts, lang)
			}
			if br := bitrateLabel(s.BitRate); br != "" {
				parts = append(parts, br)
			}
			sb.WriteString(strings.Join(parts, "  ") + "\n")
		}
	}

	writeStreams("video")
	writeStreams("audio")
	writeStreams("subtitle")
	return sb.String()
}
