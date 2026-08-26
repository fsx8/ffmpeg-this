package media

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var extensions = []string{
	".mkv", ".mp4", ".avi", ".mov", ".webm", ".flv", ".wmv",
	".mp3", ".flac", ".wav", ".ogg",
	".gif",
}

func IsMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return slices.Contains(extensions, ext)
}

func ListMediaFiles(dir string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if IsMediaFile(name) {
			files = append(files, name)
		}
	}
	slices.Sort(files)
	return files, nil
}
