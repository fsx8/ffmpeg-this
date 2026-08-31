package app

import (
	"context"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	ffprobe "github.com/fsx8/ffwiz/internal/ffprobe"
)

type hdrProbeMsg struct{ note string }

// hdrWarner carries the async color-format probe shared by the wizards
// that re-encode video to 8-bit SDR (compress, resize, transform, speed,
// reverse). Those commands force -pix_fmt yuv420p, so an HDR or 10-bit
// source would be crushed to washed-out 8-bit without tone mapping; the
// wizards surface a warning instead of doing that silently.
type hdrWarner struct {
	note   string
	cancel context.CancelFunc // aborts the in-flight probe (set by begin)
}

// begin starts the off-UI-thread probe; it never fails the wizard — an
// unreadable file simply produces no warning. The context is created
// before the command is returned and kept in cancel, so quitting the
// wizard aborts the probe instead of letting it run to completion.
func (h *hdrWarner) begin(prober ffprobe.Prober, path string) tea.Cmd {
	h.note = ""
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	h.cancel = cancel
	return func() tea.Msg {
		defer cancel()
		if prober == nil {
			return hdrProbeMsg{}
		}
		res, err := prober.Probe(ctx, path)
		if err != nil || res == nil {
			return hdrProbeMsg{}
		}
		if s := res.FirstVideo(); s != nil {
			return hdrProbeMsg{note: hdrNoteFor(*s)}
		}
		return hdrProbeMsg{}
	}
}

// cancelProbe aborts an in-flight probe, if one is running.
func (h *hdrWarner) cancelProbe() {
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
}

// apply folds a probe result in and reports whether a warning was found.
func (h *hdrWarner) apply(msg hdrProbeMsg) bool {
	h.note = msg.note
	return h.note != ""
}

// hdrNoteFor renders the user-facing warning for HDR / >8-bit sources;
// empty for plain 8-bit SDR input.
func hdrNoteFor(s ffprobe.Stream) string {
	hdr := ""
	switch s.ColorTransfer {
	case "smpte2084":
		hdr = "HDR10"
	case "arib-std-b67":
		hdr = "HLG"
	}
	depth := pixDepth(s.PixFmt)

	var what []string
	if hdr != "" {
		what = append(what, hdr)
	}
	if depth > 8 {
		what = append(what, strconv.Itoa(depth)+"-bit "+s.PixFmt)
	}
	if len(what) == 0 {
		return ""
	}

	note := "Note: source is " + strings.Join(what, " ") +
		". The output will be re-encoded to 8-bit SDR (yuv420p) without tone mapping and may look washed out."
	if hdr != "" {
		note += " Use Modify Tracks with a stream copy to keep HDR intact."
	}
	return note
}

// knownPixDepths pins formats whose names a digit heuristic would misread:
// y210, x2rgb10, and xv30 are 10-bit despite their trailing digits, and
// rgb48/bgr48 use bits-per-pixel naming (16 bits per channel).
var knownPixDepths = map[string]int{
	"y210":    10,
	"x2rgb10": 10,
	"xv30":    10,
	"rgb48":   48,
	"bgr48":   48,
}

// pixDepth extracts the bit depth from a pix_fmt name (yuv420p10le -> 10);
// unrecognized names report 8.
func pixDepth(pixFmt string) int {
	for _, suffix := range []string{"", "le", "be"} {
		p := strings.TrimSuffix(pixFmt, suffix)
		if d, ok := knownPixDepths[p]; ok {
			return d
		}
		// Planar formats spell the depth after the final "p"
		// (yuv420p10le -> yuv420p10 -> 10).
		if i := strings.LastIndex(p, "p"); i >= 0 {
			if n, err := strconv.Atoi(p[i+1:]); err == nil && n > 0 {
				return n
			}
			continue
		}
		// Packed gray/alpha formats carry the depth as trailing digits
		// (gray10le -> gray10 -> 10); other packed families (nv12,
		// rgb565le) encode layout, not depth, and stay unrecognized.
		if !strings.HasPrefix(p, "gray") && !strings.HasPrefix(p, "ya") {
			continue
		}
		digits := p
		for len(digits) > 0 && digits[len(digits)-1] >= '0' && digits[len(digits)-1] <= '9' {
			digits = digits[:len(digits)-1]
		}
		if len(digits) < len(p) {
			if n, err := strconv.Atoi(p[len(digits):]); err == nil && n > 0 {
				return n
			}
		}
	}
	return 8
}
