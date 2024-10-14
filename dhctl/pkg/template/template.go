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
	"fmt"
	"os"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func RenderAndSaveTemplate(outFileName, templatePath string, data map[string]interface{}) (string, error) {
	fileContent, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("loading %s: %v", templatePath, err)
	}

	e := Engine{
		Name: outFileName,
		Data: data,
	}

	content := string(fileContent)

	if data != nil {
		res, err := e.Render(fileContent)
		if err != nil {
			return "", err
		}
		cnt := res.Bytes()
		content = string(cnt)
		log.DebugF("Render and save template content:\n%s", content)
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
