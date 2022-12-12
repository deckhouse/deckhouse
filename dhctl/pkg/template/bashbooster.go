// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func RenderBashBooster(templatesDir string) (string, error) {
	files, err := os.ReadDir(templatesDir)
	if err != nil {
		return "", fmt.Errorf("bashbooster read dir: %v", err)
	}

	builder := strings.Builder{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(templatesDir, file.Name())

		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("bashbooster read file %q: %v", filePath, err)
		}

		// BashBooster step can have no endline symbol at the end of the file. Tolerate this.
		bashBoosterScriptContent := strings.TrimSuffix(string(fileContent), "\n")
		builder.WriteString(fmt.Sprintf("# %s\n\n%s\n", filePath, bashBoosterScriptContent))
	}

	return builder.String(), nil
}
