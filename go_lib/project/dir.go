// Copyright 2024 Flant JSC
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

package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Dir() (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get process working directory: %w", err)
	}

	return dir(pwd)
}

func dir(pwd string) (string, error) {
	const (
		filePathSeparator = string(filepath.Separator)
		d8Dir             = "deckhouse"
	)

	var projectDir string
	baseDir, _, ok := strings.Cut(pwd, filePathSeparator+d8Dir)
	if ok {
		if baseDir == "" {
			baseDir = filePathSeparator
		}
		projectDir = filepath.Join(baseDir, d8Dir)
	} else {
		projectDir = baseDir
	}

	return projectDir, nil
}
