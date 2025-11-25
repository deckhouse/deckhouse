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

package fs

import (
	"fmt"
	"os"
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

func DoAbsolutePath(p string, shouldBeDir bool) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		p = "/"
	}

	p, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("Cannot get absolute path for %s: %w", p, err)
	}

	p = filepath.Clean(p)

	stat, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("Cannot get stat for %s: %w", p, err)
	}

	if shouldBeDir && !stat.IsDir() {
		return "", fmt.Errorf("%s is not a directory", p)
	}

	return p, nil
}

func IsExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}
