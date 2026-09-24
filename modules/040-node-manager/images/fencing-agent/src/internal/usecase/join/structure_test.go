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

package join

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

const (
	watchdogUsecase   = "fencing-agent/internal/usecase/watchdog"
	structureMovedMsg = "the code this test reads has moved"
)

func TestJoinDoesNotImportTheWatchdogUsecase(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the package directory: %v", err)
	}

	fset := token.NewFileSet()
	checked := 0

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		checked++

		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("%s: unquote import %s: %v", name, spec.Path.Value, err)
			}

			if path == watchdogUsecase || strings.HasPrefix(path, watchdogUsecase+"/") {
				t.Errorf("%s imports %s, want the join usecase to stay out of the watchdog usecase", name, path)
			}
		}
	}

	if checked == 0 {
		t.Fatalf("no non-test Go file found in the package directory: %s", structureMovedMsg)
	}
}
