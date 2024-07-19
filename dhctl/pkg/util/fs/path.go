package fs

import (
	"path/filepath"
	"strings"
)

func RevealWildcardPaths(paths []string) []string {
	for _, path := range paths {
		if strings.Contains(path, "*") {
			revealPaths, _ := filepath.Glob(path)
			paths = append(paths, revealPaths...)
		}
	}
	return paths
}
