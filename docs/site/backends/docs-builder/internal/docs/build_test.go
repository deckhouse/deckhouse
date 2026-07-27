// Copyright 2023 Flant JSC
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

package docs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/fsync"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestModuleNameFromErrorPathRegexp(t *testing.T) {
	input := "error building site: assemble: \"/app/hugo/content/modules/moduleName/BROKEN.md:1:1\": EOF looking for end YAML front matter delimiter"

	moduleName, ok := getModuleNameFromErrorPath(input)
	if !ok || moduleName != "moduleName" {
		t.Fatalf("unexpected module name %q", moduleName)
	}
}

func TestModuleNameFromErrorPathWithColorRegexp(t *testing.T) {
	input := "error building site: assemble: \x1b[1;36m\"/app/hugo/content/modules/moduleName/BROKEN.md:1:1\"\x1b[0m: EOF looking for end YAML front matter delimiter"

	moduleName, ok := getModuleNameFromErrorPath(input)
	if !ok || moduleName != "moduleName" {
		t.Fatalf("unexpected module name %q", moduleName)
	}
}

func TestGetModulePath(t *testing.T) {
	var tests = []struct {
		filePath string
		expected string
	}{
		{
			filePath: "/app/hugo/content/modules/moduleName/BROKEN.md",
			expected: "/app/hugo/content/modules/moduleName",
		},
	}

	for _, test := range tests {
		t.Run(test.filePath, func(t *testing.T) {
			got := filepath.Dir(test.filePath)
			if got != test.expected {
				t.Error("unexpected result", got)
			}
		})
	}
}

func TestSyncDirMissingSource(t *testing.T) {
	syncer := fsync.NewSyncer()
	dst := filepath.Join(t.TempDir(), "dst")

	// A missing source is a legitimate case (Hugo emits no output dir for a
	// language with no pages): syncDir must return nil and not create dst.
	if err := syncDir(syncer, filepath.Join(t.TempDir(), "does-not-exist"), dst); err != nil {
		t.Fatalf("expected nil error for missing source, got %v", err)
	}

	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("expected dst not to be created, stat err = %v", err)
	}
}

func TestSyncDirCopiesExistingSource(t *testing.T) {
	syncer := fsync.NewSyncer()
	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")

	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "index.html"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncDir(syncer, src, dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "index.html"))
	if err != nil {
		t.Fatalf("expected file mirrored to dst: %v", err)
	}
	if string(got) != "hi" {
		t.Fatalf("unexpected content %q", got)
	}
}

func TestSyncDirStatError(t *testing.T) {
	syncer := fsync.NewSyncer()
	base := t.TempDir()

	// Reference a path *under* a regular file so os.Stat fails with ENOTDIR,
	// which is not IsNotExist and therefore must be surfaced, not swallowed.
	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := syncDir(syncer, filepath.Join(file, "child"), filepath.Join(base, "dst"))
	if err == nil {
		t.Fatal("expected error for non-NotExist stat failure, got nil")
	}
	if os.IsNotExist(err) {
		t.Fatalf("expected a non-NotExist stat error to be surfaced, got %v", err)
	}
}

func TestParseModulePath(t *testing.T) {
	var tests = []struct {
		modulePath string
		moduleName string
		channel    string
	}{
		{
			modulePath: "/app/hugo/content/modules/moduleName/alpha",
			moduleName: "moduleName",
			channel:    "alpha",
		},
	}

	for _, test := range tests {
		t.Run(test.modulePath, func(t *testing.T) {
			svc := &Service{
				logger: log.NewNop(),
			}
			moduleName, channel := svc.parseModulePath(test.modulePath)
			if moduleName != test.moduleName {
				t.Errorf("unexpected module name %q", moduleName)
			}

			if channel != test.channel {
				t.Errorf("unexpected channel %q", channel)
			}
		})
	}
}
