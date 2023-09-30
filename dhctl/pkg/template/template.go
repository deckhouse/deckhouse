// Copyright 2023 Flant JSC
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
	"bytes"
	"fmt"
	"os"
	"text/template"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func RenderAndSaveTemplate(outFileName, templatePath string, data map[string]interface{}) (string, error) {
	fileContent, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("loading %s: %v", templatePath, err)
	}

	content := string(fileContent)

	if data != nil {
		t := template.New(fmt.Sprintf("%s-render", outFileName)).Funcs(FuncMap())
		t, err := t.Parse(content)
		if err != nil {
			return "", err
		}

		var tpl bytes.Buffer

		err = t.Execute(&tpl, data)
		if err != nil {
			return "", err
		}

		content = tpl.String()
		log.DebugF("Bundle script content:\n%s", content)
	}

	outFile, err := os.CreateTemp(os.TempDir(), fmt.Sprintf("*-%s", outFileName))
	if err != nil {
		return "", err
	}

	defer func() {
		if err := outFile.Close(); err != nil {
			log.ErrorF("Cannot close rendered %s %s:%v", outFileName, outFile.Name(), err)
		}
	}()

	if _, err = outFile.WriteString(content); err != nil {
		return "", err
	}

	if err = outFile.Sync(); err != nil {
		return "", err
	}

	if err = outFile.Chmod(0775); err != nil {
		return "", err
	}

	return outFile.Name(), nil
}
