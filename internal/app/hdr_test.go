package app

import (
	"testing"

	"github.com/fsx8/ffwiz/internal/ffprobe"
)

func TestHDRNoteFor(t *testing.T) {
	cases := []struct {
		name    string
		stream  ffprobe.Stream
		wantSub string // empty means no note expected
	}{
		{"SDR 8-bit", ffprobe.Stream{PixFmt: "yuv420p"}, ""},
		{"HDR10 10-bit", ffprobe.Stream{PixFmt: "yuv420p10le", ColorTransfer: "smpte2084"}, "HDR10"},
		{"HLG", ffprobe.Stream{PixFmt: "yuv420p10le", ColorTransfer: "arib-std-b67"}, "HLG"},
		{"10-bit SDR", ffprobe.Stream{PixFmt: "yuv420p10le"}, "10-bit"},
		{"12-bit", ffprobe.Stream{PixFmt: "yuv420p12be"}, "12-bit"},
	}
	for _, c := range cases {
		got := hdrNoteFor(c.stream)
		if c.wantSub == "" {
			if got != "" {
				t.Errorf("%s: expected no note, got %q", c.name, got)
			}
			continue
		}
		if got == "" || !contains(got, c.wantSub) {
			t.Errorf("%s: note %q should mention %q", c.name, got, c.wantSub)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestPixDepth(t *testing.T) {
	cases := map[string]int{
		"yuv420p":     8,
		"yuv420p10le": 10,
		"yuv420p10be": 10,
		"yuv420p12le": 12,
		"yuv444p16le": 16,
		"nv12":        8,
		"p010le":      10, // semi-planar 10-bit layout
		"yuv420p10":   10,
		"gray10le":    10, // non-planar: digits are the depth
		"gray16le":    16,
		"rgb48le":     48, // 16 bits/channel, 48 bits/pixel naming
		"bgr48be":     48,
		"y210le":      10, // 10-bit despite the trailing 210
		"x2rgb10le":   10, // 10-bit despite the trailing 2
		"rgb565le":    8,  // layout code, not depth
		"junk":        8,
	}
	for fmt, want := range cases {
		if got := pixDepth(fmt); got != want {
			t.Errorf("pixDepth(%q) = %d, want %d", fmt, got, want)
		}
	}
}
