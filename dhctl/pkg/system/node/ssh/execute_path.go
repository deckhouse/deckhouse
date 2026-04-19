// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssh

import (
	"path/filepath"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
)

type ScriptPath interface {
	IsSudo() bool
	UploadDir() string
}

// ExecuteRemoteScriptPath
// deprecated - ugly solution
func ExecuteRemoteScriptPath(u ScriptPath, scriptName string, full bool) string {
	root := ""
	if u.IsSudo() {
		root = app.DeckhouseNodeTmpPath
	}

	if uploadDir := u.UploadDir(); uploadDir != "" {
		root = uploadDir
	}

	if root == "" {
		res := "."
		if full {
			res = res + "/" + scriptName
		}

		return res
	}

	return filepath.Join(root, scriptName)
}
