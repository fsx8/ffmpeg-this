package media

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var extensions = []string{
	".mkv", ".mp4", ".avi", ".mov", ".webm", ".flv", ".wmv", ".m4v",
	".ts", ".mpg", ".mpeg",
	".mp3", ".flac", ".wav", ".ogg", ".m4a", ".aac", ".opus",
	".gif",
}

func IsMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return slices.Contains(extensions, ext)
}

// ListMediaFiles returns media file names in natural order, so that
// "part2.mp4" sorts before "part10.mp4".
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
	slices.SortFunc(files, naturalCompare)
	return files, nil
}

func naturalCompare(a, b string) int {
	less := naturalLess(a, b)
	if less {
		return -1
	}
	if naturalLess(b, a) {
		return 1
	}
	return 0
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// naturalLess compares byte-by-byte, but runs of digits are compared by
// numeric value so "file9" < "file10".
func naturalLess(a, b string) bool {
	ia, ib := 0, 0
	for ia < len(a) && ib < len(b) {
		if isDigit(a[ia]) && isDigit(b[ib]) {
			ja, jb := ia, ib
			for ja < len(a) && isDigit(a[ja]) {
				ja++
			}
			for jb < len(b) && isDigit(b[jb]) {
				jb++
			}
			na := strings.TrimLeft(a[ia:ja], "0")
			nb := strings.TrimLeft(b[ib:jb], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			ia, ib = ja, jb
			continue
		}
		if a[ia] != b[ib] {
			return a[ia] < b[ib]
		}
		ia++
		ib++
	}
	// Whatever ran out of characters first is smaller.
	return len(a)-ia < len(b)-ib
}
