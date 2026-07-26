package util

import (
	"os"
	"path/filepath"
	"strings"
)

func Glob(dir string, ext string) ([]string, error) {
	files := []string{}
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if filepath.Ext(path) == ext && !strings.Contains(path, "Body 13") && !strings.Contains(path, "Bull of Heaven") {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

