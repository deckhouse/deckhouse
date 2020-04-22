package template

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

func RenderBashBooster(templatesDir string) (string, error) {
	templatesDir = strings.TrimSuffix(templatesDir, "/") + "/"

	files, err := ioutil.ReadDir(templatesDir)
	if err != nil {
		return "", fmt.Errorf("read dir: %v", err)
	}

	filesContent := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(templatesDir, file.Name())

		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("read file %q error: %v", filePath, err)
		}

		filesContent = append(filesContent, fmt.Sprintf("# %s\n\n%s\n", filePath, strings.TrimSuffix(string(data), "\n")))
	}

	return strings.Join(filesContent, "\n"), nil
}
