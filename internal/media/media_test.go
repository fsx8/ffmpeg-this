package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsMediaFile(t *testing.T) {
	if !IsMediaFile("x.MKV") {
		t.Fatal("expected mkv true")
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
