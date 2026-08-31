package execx

import (
	"strings"
	"testing"
)

func TestScanLinesSplitsOnLFAndCR(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"lf terminated", "one\ntwo\n", []string{"one", "two"}},
		{"crlf terminated", "one\r\ntwo\r\n", []string{"one", "two"}},
		{"bare cr terminated", "one\rtwo\rthree\r", []string{"one", "two", "three"}},
		{"no trailing terminator", "one\ntwo", []string{"one", "two"}},
		{"empty", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			scanLines(strings.NewReader(c.input), func(line string) { got = append(got, line) })
			if len(got) != len(c.want) {
				t.Fatalf("lines: got %#v want %#v", got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Fatalf("lines: got %#v want %#v", got, c.want)
				}
			}
		})
	}
}

func TestScanLinesNilCallbackDrains(t *testing.T) {
	// Must not panic and must consume the reader.
	scanLines(strings.NewReader("a\nb"), nil)
}
