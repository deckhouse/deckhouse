package template

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

func RenderBashBooster(templatesDir string) (string, error) {
	templatesDir = formatDir(templatesDir)

	files, err := ioutil.ReadDir(templatesDir)
	if err != nil {
		return "", fmt.Errorf("bashbooster read dir: %v", err)
	}

	builder := strings.Builder{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(templatesDir, file.Name())

		fileContent, err := ioutil.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("bashbooster read file %q: %v", filePath, err)
		}

		// BashBooster step can have no endline symbol at the end of the file. Tolerate this.
		bashBoosterScriptContent := strings.TrimSuffix(string(fileContent), "\n")
		builder.WriteString(fmt.Sprintf("# %s\n\n%s\n", filePath, bashBoosterScriptContent))
	}

	return builder.String(), nil
}
