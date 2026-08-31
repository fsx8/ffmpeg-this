package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsMediaFile(t *testing.T) {
	for _, name := range []string{"x.MKV", "a.m4a", "b.m4v", "c.aac", "d.opus", "e.ts", "f.mpg", "g.mpeg", "h.mka", "i.m2ts", "j.ogv"} {
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

func TestListDirOmitsHiddenEntries(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"show.mp4", ".hidden.mp4"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"show", ".git"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files, dirs, err := ListDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "show.mp4" {
		t.Fatalf("files: %#v", files)
	}
	if len(dirs) != 1 || dirs[0] != "show" {
		t.Fatalf("dirs: %#v", dirs)
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
		{"B.mp4", "a.mp4", false}, // case-insensitive: b > a
		{"a.mp4", "B.mp4", true},
		{"ABC", "abd", true},
		{"Episode_2", "episode_10", true},
	}
	for _, c := range cases {
		if got := naturalLess(c.a, c.b); got != c.want {
			t.Errorf("naturalLess(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// Symlinked directories must be navigable: DirEntry reports them as
// non-directories (lstat), so ListDir resolves them explicitly.
func TestListDirFollowsSymlinkedDirectories(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "e.mp4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	_, dirs, err := ListDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range dirs {
		if d == "link" {
			found = true
		}
	}
	if !found {
		t.Fatalf("symlinked directory must be listed, dirs = %v", dirs)
	}
}
