// Copyright 2026 Flant JSC
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

package image

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func PrepareFiles(path string) error {
	pathStat, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !pathStat.IsDir() {
		return fmt.Errorf("%s isn't directory", path)
	}

	bashiblePath := filepath.Join(path, "candi", "bashible")

	walkFunc := func(fullPath string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}

		if err != nil {
			return err
		}

		if !strings.HasSuffix(fullPath, ".tpl") {
			return nil
		}

		patched, err := replaceTextInFile(fullPath, "deckhouse/candi/bashible", bashiblePath, info.Mode())
		if err != nil {
			return err
		}
		if !patched {
			return nil
		}

		log.DebugF("found and patched file %s\n", fullPath)

		return nil
	}

	err = filepath.Walk(path, walkFunc)
	if err != nil {
		return err
	}

	return nil
}

func replaceTextInFile(file, oldText, newText string, mode fs.FileMode) (bool, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return false, fmt.Errorf("failed to open input file: %w", err)
	}

	out := strings.ReplaceAll(string(content), oldText, newText)
	if string(content) != out {
		return true, os.WriteFile(file, []byte(out), mode)
	}

	return false, nil
}
