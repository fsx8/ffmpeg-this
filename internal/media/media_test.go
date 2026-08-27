package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsMediaFile(t *testing.T) {
	for _, name := range []string{"x.MKV", "a.m4a", "b.m4v", "c.aac", "d.opus", "e.ts", "f.mpg", "g.mpeg"} {
		if !IsMediaFile(name) {
			t.Errorf("expected %s to be recognized as media", name)
		}
	}
	if IsMediaFile("x.txt") {
		t.Fatal("expected txt false")
	}
}

func TestListMediaFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.mp4", "b.txt", "c.mkv"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := ListMediaFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0] != "a.mp4" || files[1] != "c.mkv" {
		t.Fatalf("unexpected files: %#v", files)
	}
}

func TestListMediaFilesNaturalOrder(t *testing.T) {
	dir := t.TempDir()
	names := []string{"part10.mp4", "part1.mp4", "part2.mp4", "part9.mp4"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := ListMediaFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"part1.mp4", "part2.mp4", "part9.mp4", "part10.mp4"}
	if len(files) != len(want) {
		t.Fatalf("unexpected files: %#v", files)
	}
	for i := range want {
		if files[i] != want[i] {
			t.Fatalf("natural order broken: got %#v, want %#v", files, want)
		}
	}
}

func TestNaturalLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"part2", "part10", true},
		{"part10", "part2", false},
		{"a1b2", "a1b10", true},
		{"abc", "abd", true},
		{"ab", "abc", true},
		{"a01", "a1", false}, // numerically equal, not less
		{"", "a", true},
		{"2", "10", true},
	}
	for _, c := range cases {
		if got := naturalLess(c.a, c.b); got != c.want {
			t.Errorf("naturalLess(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
