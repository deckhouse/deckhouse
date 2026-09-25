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

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every hook folder documents each of its hooks in README.md, so a hook added or
// moved without a README line fails here instead of going unnoticed.
func TestEveryHookIsDescribedInFolderReadme(t *testing.T) {
	hooksDir := ".."
	folders, err := os.ReadDir(hooksDir)
	if err != nil {
		t.Fatalf("read hooks dir: %v", err)
	}
	for _, folder := range folders {
		if !folder.IsDir() || folder.Name() == "internal" || folder.Name() == "pkg" {
			continue
		}
		dir := filepath.Join(hooksDir, folder.Name())
		readme, err := os.ReadFile(filepath.Join(dir, "README.md"))
		if err != nil {
			t.Errorf("%s has no README.md: %v", folder.Name(), err)
			continue
		}
		sources, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatalf("list %s: %v", dir, err)
		}
		for _, source := range sources {
			name := filepath.Base(source)
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			if !strings.Contains(string(readme), "`"+strings.TrimSuffix(name, ".go")+"`") && !strings.Contains(string(readme), "`"+name+"`") {
				t.Errorf("%s/README.md does not mention %s", folder.Name(), name)
			}
		}
	}
}
