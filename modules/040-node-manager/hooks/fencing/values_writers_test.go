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

package fencing

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var fencingValuesWrite = regexp.MustCompile(`Values\.(?:Set|Remove|Patch)\w*\(\s*"(nodeManager\.internal\.fencing[^"]*)"`)

// Every path of nodeManager.internal.fencing has one writer. The test reads the sources of all
// hooks of the module, so a hook outside this package cannot take over a path either.
func TestEveryFencingValuesPathHasOneWriter(t *testing.T) {
	writers := map[string]map[string]bool{}

	err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range fencingValuesWrite.FindAllStringSubmatch(string(source), -1) {
			if writers[match[1]] == nil {
				writers[match[1]] = map[string]bool{}
			}
			writers[match[1]][path] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(writers) == 0 {
		t.Fatal("no hook writes nodeManager.internal.fencing: the pattern is stale")
	}

	for path, files := range writers {
		if len(files) != 1 {
			t.Errorf("%s has %d writers: %v", path, len(files), files)
		}
		for other := range writers {
			if other != path && strings.HasPrefix(other, path+".") {
				t.Errorf("%s is written together with its child %s", path, other)
			}
		}
	}
}
