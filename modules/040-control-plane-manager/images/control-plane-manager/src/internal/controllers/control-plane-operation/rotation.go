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

package controlplaneoperation

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type timedDirEntry struct {
	name  string
	mtime time.Time
}

func rotateDirectories(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}

	var dirs []timedDirEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return fmt.Errorf("stat dir %s: %w", e.Name(), err)
		}
		dirs = append(dirs, timedDirEntry{name: e.Name(), mtime: info.ModTime()})
	}

	if len(dirs) <= keep {
		return nil
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].mtime.After(dirs[j].mtime)
	})

	for _, d := range dirs[keep:] {
		if err := os.RemoveAll(filepath.Join(dir, d.name)); err != nil {
			return fmt.Errorf("remove old dir %s: %w", d.name, err)
		}
	}

	return nil
}
