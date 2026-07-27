/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testenv

import (
	"os"
	"path/filepath"
)

func findDirUp(name string) string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(dir, name)
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func resolveUpPaths[T ~string](dirUp string, subpath []T) []string {
	if len(subpath) == 0 {
		return nil
	}

	base := findDirUp(dirUp)
	ret := make([]string, 0, len(subpath))

	for _, f := range subpath {
		ret = append(ret, filepath.Join(base, string(f)))
	}
	return ret
}
