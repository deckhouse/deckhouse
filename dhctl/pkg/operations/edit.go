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

package operations

import (
	"os"
	"os/exec"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func Edit(data []byte) ([]byte, error) {
	schemaStore := config.NewSchemaStore()

	editor := app.Editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
	}

	tmpFile, err := os.CreateTemp(app.TmpDirName, "dhctl-editor.*.yaml")
	if err != nil {
		log.ErrorF("can't save cluster configuration: %s\n", err)
		return nil, err
	}

	err = os.WriteFile(tmpFile.Name(), data, 0o600)
	if err != nil {
		log.ErrorF("can't write write cluster configuration to the file %s: %s\n", tmpFile.Name(), err)
		return nil, err
	}

	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return nil, err
	}

	modifiedData, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, err
	}

	_, err = schemaStore.Validate(&modifiedData)
	if err != nil {
		return nil, err
	}

	modifiedData, err = yaml.JSONToYAML(modifiedData)
	if err != nil {
		return nil, err
	}

	return modifiedData, nil
}
